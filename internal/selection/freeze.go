package selection

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/gopher-launch/concoct/internal/activework"
	"github.com/gopher-launch/concoct/internal/codexapp"
	"github.com/gopher-launch/concoct/internal/roadmap"
)

func freezeReady(ctx context.Context, cfg Config, owner *activework.Owner, store *activework.Store, session *hostSession, project projectSnapshot, parent parentBinding, outcome codexapp.TurnOutcome) (activework.Candidate, error) {
	var candidate activework.Candidate
	if outcome.Status != "completed" || outcome.TurnID == "" || session.taskID == "" || !session.checkpointSaved {
		return candidate, fmt.Errorf("ready result lacks completed turn, claim or checkpoint")
	}
	if err := cfg.Agent.Idle(ctx, session.threadID); err != nil {
		return candidate, fmt.Errorf("reagent tool work is not confirmed idle: %w", err)
	}
	if err := verifyParentUnchanged(store, parent); err != nil {
		return candidate, err
	}
	branch, err := gitOutput(session.workspace, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || branch != session.taskBranch {
		return candidate, fmt.Errorf("task checkout left claimed branch %q", session.taskBranch)
	}
	head, err := gitOutput(session.workspace, "rev-parse", "HEAD")
	if err != nil {
		return candidate, err
	}
	if _, err := gitOutput(session.workspace, "merge-base", "--is-ancestor", parent.Commit, head); err != nil {
		return candidate, fmt.Errorf("task branch no longer descends from baseline: %w", err)
	}
	checkpointText, err := owner.Checkpoint()
	if err != nil {
		return candidate, err
	}
	checkpoint, err := parseReadyCheckpoint(checkpointText)
	if err != nil {
		return candidate, err
	}
	exclusions, err := generatedExclusions(store.Checkout, cfg.GeneratedExclusions)
	if err != nil {
		return candidate, err
	}
	changed, untracked, err := workingPaths(session.workspace, parent.Commit, exclusions, checkpoint.GeneratedOutputs, checkpoint.Deliverables)
	if err != nil {
		return candidate, err
	}
	if len(changed) == 0 {
		return candidate, fmt.Errorf("ready task has no source change")
	}
	entry, err := eligibleEntry(project.Document, session.taskID)
	if err != nil {
		return candidate, err
	}
	if err := validateReadyCheckpoint(checkpoint, entry, project, session.workspace, changed, cfg.AcceptedUnavailable, cfg.RequiredChecks); err != nil {
		return candidate, err
	}
	if err := validateReconciliation(project, session.workspace, entry); err != nil {
		return candidate, err
	}
	generated := slices.Clone(checkpoint.GeneratedOutputs)
	sort.Strings(generated)
	prior := ""
	snapshot, err := store.Status()
	if err != nil {
		return candidate, err
	}
	if snapshot.Candidate != nil {
		prior = snapshot.Candidate.Commit
	}
	if err := owner.BeginCandidateFreeze(activework.CandidateFreezeIntent{TurnID: outcome.TurnID, CheckpointSHA256: sha256Hex([]byte(checkpointText)), PriorCommit: prior, ChangedPaths: changed, GeneratedOutputs: generated}); err != nil {
		return candidate, err
	}
	stagedTree, err := stageSource(session.workspace, untracked, changed)
	if err != nil {
		return candidate, err
	}
	if err := requireCleanIndexWorktree(session.workspace, checkpoint.GeneratedOutputs); err != nil {
		return candidate, err
	}
	staged, err := gitBytes(session.workspace, "diff", "--cached", "--name-only", "-z", "HEAD")
	if err != nil {
		return candidate, err
	}
	if len(staged) != 0 {
		if _, err := gitOutput(session.workspace, "-c", "user.name=Concoct", "-c", "user.email=concoct@localhost", "commit", "-m", "Freeze "+session.taskID+" candidate"); err != nil {
			return candidate, err
		}
	}
	commit, err := gitOutput(session.workspace, "rev-parse", "HEAD")
	if err != nil {
		return candidate, err
	}
	tree, err := gitOutput(session.workspace, "rev-parse", "HEAD^{tree}")
	if err != nil || tree != stagedTree {
		return candidate, fmt.Errorf("candidate commit tree differs from validated source")
	}
	if err := requireCleanIndexWorktree(session.workspace, checkpoint.GeneratedOutputs); err != nil {
		return candidate, err
	}
	if _, err := gitOutput(session.workspace, "diff", "--cached", "--quiet", "HEAD"); err != nil {
		return candidate, fmt.Errorf("candidate index changed after commit: %w", err)
	}
	if err := verifyParentUnchanged(store, parent); err != nil {
		return candidate, err
	}
	actualBranch, err := gitOutput(session.workspace, "rev-parse", "refs/heads/"+session.taskBranch)
	if err != nil || actualBranch != commit {
		return candidate, fmt.Errorf("task branch moved during candidate freeze")
	}
	finalPaths, err := diffPaths(session.workspace, parent.Commit, commit)
	if err != nil || !slices.Equal(changed, finalPaths) {
		return candidate, fmt.Errorf("candidate changed paths differ from validated source")
	}
	for _, path := range untracked {
		if _, err := gitOutput(session.workspace, "cat-file", "-e", commit+":"+path); err != nil {
			return candidate, fmt.Errorf("untracked deliverable %q missing from candidate: %w", path, err)
		}
	}
	baselineTree, err := gitOutput(session.workspace, "rev-parse", parent.Commit+"^{tree}")
	if err != nil {
		return candidate, err
	}
	diff, err := gitBytes(session.workspace, "diff", "--binary", "--no-ext-diff", "--no-renames", parent.Commit, commit)
	if err != nil {
		return candidate, err
	}
	candidate = activework.Candidate{BaselineCommit: parent.Commit, BaselineTree: baselineTree, Commit: commit, Tree: tree, DiffSHA256: sha256Hex(diff), CheckpointSHA256: sha256Hex([]byte(checkpointText)), ChangedPaths: finalPaths, GeneratedOutputs: generated, TurnID: outcome.TurnID}
	if err := owner.FreezeCandidate(candidate); err != nil {
		return activework.Candidate{}, err
	}
	return candidate, nil
}

func validateReconciliation(base projectSnapshot, workspace string, selectedEntry roadmap.Entry) error {
	updated, err := readProject(workspace)
	if err != nil {
		return fmt.Errorf("candidate documentation: %w", err)
	}
	baseline := base.Document.Entries()
	proposed := updated.Document.Entries()
	if len(baseline) != len(proposed) {
		return fmt.Errorf("candidate roadmap changed task ID inventory")
	}
	byID := make(map[string]roadmap.Entry, len(proposed))
	for _, entry := range proposed {
		byID[entry.ID] = entry
	}
	for _, entry := range baseline {
		if _, ok := byID[entry.ID]; !ok {
			return fmt.Errorf("candidate roadmap removed ID %s", entry.ID)
		}
	}
	selected := byID[selectedEntry.ID]
	if selected.Kind != "task" || selected.Status != "delivered" {
		return fmt.Errorf("candidate roadmap does not mark %s delivered", selectedEntry.ID)
	}
	baseObjective, baseAcceptance, err := taskScope(selectedEntry)
	if err != nil {
		return err
	}
	newObjective, newAcceptance, err := taskScope(selected)
	if err != nil || baseObjective != newObjective || baseAcceptance != newAcceptance {
		return fmt.Errorf("candidate changed agreed scope for %s", selectedEntry.ID)
	}
	for _, id := range append(capabilityIDs(base.Capabilities), selectedEntry.ID) {
		if !slices.Contains(capabilityIDs(updated.Capabilities), id) {
			return fmt.Errorf("candidate capabilities omitted delivery ID %s", id)
		}
	}
	return nil
}

func capabilityIDs(markdown string) []string {
	var ids []string
	for _, line := range strings.Split(markdown, "\n") {
		if !strings.HasPrefix(line, "## ") || !strings.Contains(line, " — ") {
			continue
		}
		id := strings.TrimPrefix(strings.SplitN(line, " — ", 2)[0], "## ")
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func gitBytes(checkout string, args ...string) ([]byte, error) {
	command := exec.Command("git", append([]string{"-C", checkout}, args...)...)
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return output, nil
}

func nulPaths(data []byte) []string {
	var paths []string
	for _, part := range bytes.Split(data, []byte{0}) {
		if len(part) > 0 {
			paths = append(paths, string(part))
		}
	}
	return paths
}

func diffPaths(checkout, from, to string) ([]string, error) {
	args := []string{"diff", "--name-only", "--no-renames", "-z", from}
	if to != "" {
		args = append(args, to)
	}
	data, err := gitBytes(checkout, args...)
	if err != nil {
		return nil, err
	}
	paths := nulPaths(data)
	sort.Strings(paths)
	return paths, nil
}

func workingPaths(checkout, baseline string, exclusions, generated, deliverables []string) ([]string, []string, error) {
	tracked, err := diffPaths(checkout, baseline, "")
	if err != nil {
		return nil, nil, err
	}
	for _, path := range tracked {
		if excluded(path, exclusions) && !slices.Contains(deliverables, path) {
			return nil, nil, fmt.Errorf("tracked change %q matches generated exclusion", path)
		}
	}
	data, err := gitBytes(checkout, "ls-files", "--others", "-z")
	if err != nil {
		return nil, nil, err
	}
	var untracked []string
	seenGenerated := make(map[string]bool, len(generated))
	for _, path := range nulPaths(data) {
		if !validRelativePath(path) {
			return nil, nil, fmt.Errorf("invalid untracked path %q", path)
		}
		if slices.Contains(generated, path) {
			if seenGenerated[path] || !excluded(path, exclusions) {
				return nil, nil, fmt.Errorf("generated output %q is duplicated or lacks an explicit exclusion", path)
			}
			seenGenerated[path] = true
			continue
		}
		untracked = append(untracked, path)
	}
	for _, path := range generated {
		if !seenGenerated[path] {
			return nil, nil, fmt.Errorf("declared generated output %q is not untracked in the checkout", path)
		}
	}
	paths := append(append([]string(nil), tracked...), untracked...)
	sort.Strings(paths)
	sort.Strings(untracked)
	paths = slices.Compact(paths)
	return paths, untracked, nil
}

func stageSource(checkout string, untracked, changed []string) (string, error) {
	if _, err := gitOutput(checkout, "add", "-u"); err != nil {
		return "", err
	}
	for _, path := range untracked {
		if _, err := gitOutput(checkout, "add", "-f", "--", path); err != nil {
			return "", err
		}
	}
	tree, err := gitOutput(checkout, "write-tree")
	if err != nil {
		return "", err
	}
	if err := verifyStagedContents(checkout, changed); err != nil {
		return "", err
	}
	if _, err := gitOutput(checkout, "diff", "--quiet"); err != nil {
		return "", fmt.Errorf("source changed while staging candidate: %w", err)
	}
	return tree, nil
}

func verifyStagedContents(checkout string, changed []string) error {
	for _, path := range changed {
		full := filepath.Join(checkout, path)
		info, err := os.Lstat(full)
		if os.IsNotExist(err) {
			continue // deletion is represented by absence from the index
		}
		if err != nil {
			return err
		}
		var contents []byte
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(full)
			if err != nil {
				return err
			}
			contents = []byte(target)
		} else if info.Mode().IsRegular() {
			contents, err = os.ReadFile(full)
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("changed path %q is not a regular file or symlink", path)
		}
		blob, err := gitBytes(checkout, "show", ":"+path)
		if err != nil || !bytes.Equal(contents, blob) {
			return fmt.Errorf("staged blob for %q differs from verified worktree content", path)
		}
	}
	return nil
}

func requireCleanIndexWorktree(checkout string, generated []string) error {
	if _, err := gitOutput(checkout, "diff", "--quiet"); err != nil {
		return err
	}
	data, err := gitBytes(checkout, "ls-files", "--others", "-z")
	if err != nil {
		return err
	}
	for _, path := range nulPaths(data) {
		if !slices.Contains(generated, path) {
			return fmt.Errorf("untracked deliverable %q remains outside candidate", path)
		}
	}
	return nil
}

func generatedExclusions(checkout string, configured []string) ([]string, error) {
	rules := append([]string(nil), configured...)
	content, err := os.ReadFile(filepath.Join(checkout, ".gitignore"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "!") {
			return nil, fmt.Errorf("negative generated exclusion %q needs explicit policy", line)
		}
		rules = append(rules, line)
	}
	for _, rule := range rules {
		bare := strings.TrimPrefix(strings.TrimSuffix(rule, "/"), "/")
		if !validRelativePath(bare) || strings.HasPrefix(bare, ".git/") || bare == ".git" {
			return nil, fmt.Errorf("invalid generated exclusion %q", rule)
		}
	}
	return rules, nil
}

func excluded(path string, rules []string) bool {
	for _, rule := range rules {
		anchored := strings.HasPrefix(rule, "/")
		rule = strings.TrimPrefix(rule, "/")
		if strings.HasSuffix(rule, "/") {
			root := strings.TrimSuffix(rule, "/")
			if path == root || strings.HasPrefix(path, root+"/") {
				return true
			}
			continue
		}
		if path == rule {
			return true
		}
		if !anchored && !strings.Contains(rule, "/") {
			for _, part := range strings.Split(path, "/") {
				if matched, _ := filepath.Match(rule, part); matched {
					return true
				}
			}
		}
	}
	return false
}
