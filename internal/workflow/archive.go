package workflow

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gopher-launch/concoct/internal/gitrepo"
)

// ArchiveOverride is the explicit, durable authority used only when the latest
// review is not approved. Both fields must also be present in summary.md.
type ArchiveOverride struct{ Authority, Reason string }

type archiveOverrideMeta struct{ Authority, Reason string }

type archiveSummary struct {
	TaskID, RoadmapID, Status, Archived, Review, Delivery string
	Impact                                                impact
	Override                                              archiveOverrideMeta
}

func CompleteArchive(root string, override ArchiveOverride) (TransitionResult, error) {
	taskData, populated, err := readPopulated(filepath.Join(root, ".concoct/current/task-plan.md"))
	if err != nil || !populated {
		return TransitionResult{}, fmt.Errorf("archive completion requires a populated active task")
	}
	var task taskMeta
	if err := parseFront(taskData, &task); err != nil {
		return TransitionResult{}, err
	}
	cur := filepath.Join(root, ".concoct/current")
	reviews, diags, err := readReviews(cur)
	if err != nil {
		return TransitionResult{}, err
	}
	if len(diags) > 0 {
		return TransitionResult{}, fmt.Errorf("invalid review evidence: %s", strings.Join(diags, "; "))
	}
	approved := len(reviews) > 0 && reviews[len(reviews)-1].meta.Status == "approved"
	if !approved && (override.Authority == "" || override.Reason == "") {
		return TransitionResult{}, fmt.Errorf("latest review is not approved; archival requires explicit --override-authority and --override-reason evidence")
	}
	if approved && (override.Authority != "" || override.Reason != "") {
		return TransitionResult{}, fmt.Errorf("override flags are only valid when ordinary approved archival is unavailable")
	}
	if task.Status != "implementation-complete" {
		return TransitionResult{}, fmt.Errorf("archive completion requires implementation-complete task evidence")
	}

	archiveRel, summary, err := validateArchiveCandidate(root, task, reviews, override)
	if err != nil {
		return TransitionResult{}, err
	}
	repo, gitBacked, err := gitrepo.Open(root)
	if err != nil {
		return TransitionResult{}, err
	}
	if task.Git.Enabled != gitBacked {
		return TransitionResult{}, fmt.Errorf("task Git metadata disagrees with repository type")
	}
	if gitBacked {
		return completeGitArchive(root, repo, task, archiveRel, summary)
	}
	if err := validateNonGitDelivery(root, task, archiveRel, summary); err != nil {
		return TransitionResult{}, err
	}
	if err := clearCurrent(cur); err != nil {
		return TransitionResult{}, fmt.Errorf("durable archive validated but current cleanup failed; preserve evidence and retry: %w", err)
	}
	if report := Detect(root); report.State != Ready {
		return TransitionResult{}, fmt.Errorf("non-Git archive cleanup did not reach ready: %s", strings.Join(report.Diagnostics, "; "))
	}
	return TransitionResult{Message: "Archive transition completed at " + archiveRel}, nil
}

func validateArchiveCandidate(root string, task taskMeta, reviews []review, override ArchiveOverride) (string, archiveSummary, error) {
	archiveRel := deterministicArchivePath(task, time.Now())
	archivePath := filepath.Join(root, filepath.FromSlash(archiveRel))
	info, err := os.Stat(archivePath)
	if err != nil || !info.IsDir() {
		return "", archiveSummary{}, fmt.Errorf("expected authored archive candidate at exact deterministic path %s", archiveRel)
	}
	for _, name := range []string{"task-plan.md", "notes.md"} {
		current, err := os.ReadFile(filepath.Join(root, ".concoct/current", name))
		if err != nil {
			return "", archiveSummary{}, err
		}
		archived, err := os.ReadFile(filepath.Join(archivePath, name))
		if err != nil {
			return "", archiveSummary{}, fmt.Errorf("archive is missing %s: %w", name, err)
		}
		if !bytes.Equal(current, archived) {
			return "", archiveSummary{}, fmt.Errorf("archive %s is not byte-identical to accepted current evidence", name)
		}
	}
	for _, review := range reviews {
		current, _ := os.ReadFile(filepath.Join(root, ".concoct/current", review.name))
		archived, err := os.ReadFile(filepath.Join(archivePath, review.name))
		if err != nil || !bytes.Equal(current, archived) {
			return "", archiveSummary{}, fmt.Errorf("archive %s is missing or differs from append-only current evidence", review.name)
		}
	}
	archivedReviews, err := filepath.Glob(filepath.Join(archivePath, "review-*.md"))
	if err != nil || len(archivedReviews) != len(reviews) {
		return "", archiveSummary{}, fmt.Errorf("archive review history must contain exactly the %d accepted sequential reviews", len(reviews))
	}
	summaryData, err := os.ReadFile(filepath.Join(archivePath, "summary.md"))
	if err != nil {
		return "", archiveSummary{}, fmt.Errorf("archive is missing summary.md: %w", err)
	}
	var raw struct {
		TaskID    string `yaml:"task-id"`
		RoadmapID string `yaml:"roadmap-id"`
		Status    string `yaml:"status"`
		Archived  string `yaml:"archived"`
		Review    string `yaml:"review"`
		Delivery  string `yaml:"delivery"`
		Impact    impact `yaml:"capability-impact"`
		Override  struct {
			Authority string `yaml:"authority"`
			Reason    string `yaml:"reason"`
		} `yaml:"override"`
	}
	if err := parseFront(summaryData, &raw); err != nil {
		return "", archiveSummary{}, fmt.Errorf("summary.md: %w", err)
	}
	s := archiveSummary{TaskID: raw.TaskID, RoadmapID: raw.RoadmapID, Status: raw.Status, Archived: raw.Archived, Review: raw.Review, Delivery: raw.Delivery, Impact: raw.Impact, Override: archiveOverrideMeta{raw.Override.Authority, raw.Override.Reason}}
	if s.TaskID != task.ID || s.RoadmapID != task.RoadmapID || s.Archived != time.Now().Format("2006-01-02") {
		return "", s, fmt.Errorf("summary task, roadmap, or archival date does not match active task and deterministic destination")
	}
	if s.Impact.Type != task.Impact.Type || !sameStrings(s.Impact.IDs, task.Impact.IDs) {
		return "", s, fmt.Errorf("summary capability-impact does not match task-plan.md")
	}
	if override.Authority == "" {
		if len(reviews) == 0 || s.Review != reviews[len(reviews)-1].name {
			return "", s, fmt.Errorf("summary review must name the latest approving review")
		}
		if s.Override.Authority != "" || s.Override.Reason != "" {
			return "", s, fmt.Errorf("ordinary archival summary must not claim override evidence")
		}
	} else if s.Override.Authority != override.Authority || s.Override.Reason != override.Reason || strings.TrimSpace(s.Override.Reason) == "" {
		return "", s, fmt.Errorf("summary override authority and reason must exactly match the explicit command evidence")
	}
	for _, heading := range []string{"## Delivered outcome", "## Key decisions", "## Files and areas changed", "## Verification", "## Review outcome", "## Capability changes", "## Skipped work", "## Follow-up work"} {
		if !nonEmptySection(string(summaryData), heading) {
			return "", s, fmt.Errorf("summary.md requires non-empty section %s", heading)
		}
	}
	if err := validateCapabilityResult(root, task, archiveRel); err != nil {
		return "", s, err
	}
	return archiveRel, s, nil
}

func deterministicArchivePath(task taskMeta, now time.Time) string {
	slug := strings.ToLower(task.Title)
	slug = strings.Trim(regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(slug, "-"), "-")
	return ".concoct/archive/" + now.Format("2006-01-02") + "-" + task.RoadmapID + "-" + slug
}

func completeGitArchive(root string, repo *gitrepo.Repository, task taskMeta, archiveRel string, summary archiveSummary) (TransitionResult, error) {
	if repo.OperationInProgress() {
		return TransitionResult{}, fmt.Errorf("an unrelated Git operation is in progress")
	}
	branch, err := repo.Branch()
	if err != nil || branch != task.Git.TaskBranch {
		return TransitionResult{}, fmt.Errorf("checkout drift: expected task branch %s", task.Git.TaskBranch)
	}
	head, err := repo.Head()
	if err != nil {
		return TransitionResult{}, err
	}
	ancestor, err := repo.IsAncestor(task.Git.Base, head)
	if err != nil || !ancestor {
		return TransitionResult{}, fmt.Errorf("recorded Git base is not an ancestor of the task branch")
	}
	if summary.Status != "archived" || summary.Delivery != "pending-integration" {
		return TransitionResult{}, fmt.Errorf("Git archive summary requires status archived and delivery pending-integration")
	}
	if task.Git.Status != "archived" || task.Git.ArchiveCommit != "self" {
		return TransitionResult{}, fmt.Errorf("Git archive task metadata requires git.status archived and non-recursive git.archive-commit self")
	}
	entries, err := repo.StatusEntries()
	if err != nil {
		return TransitionResult{}, err
	}
	if len(entries) == 0 {
		return validateCommittedArchiveRetry(root, repo, task, archiveRel, head)
	}
	for _, entry := range entries {
		for _, path := range entry.Paths {
			p := filepath.ToSlash(path)
			if !(strings.HasPrefix(p, archiveRel+"/") || p == ".concoct/capabilities.md" || p == ".concoct/roadmap.md" || p == ".concoct/current/task-plan.md" || p == ".concoct/current/notes.md") {
				return TransitionResult{}, fmt.Errorf("Archivist transition contains forbidden path %s", p)
			}
		}
	}
	if err := validateGitCapabilityDiff(repo, "HEAD", task, archiveRel); err != nil {
		return TransitionResult{}, err
	}
	if err := validateGitRoadmapDiff(repo, "HEAD", task); err != nil {
		return TransitionResult{}, err
	}
	if err := validateRoadmapEvidence(root, task, archiveRel, "active"); err != nil {
		return TransitionResult{}, err
	}
	if err := repo.AddAll(); err != nil {
		return TransitionResult{}, err
	}
	if err := repo.Commit("concoct: archive " + task.ID); err != nil {
		return TransitionResult{}, err
	}
	commit, err := repo.Head()
	if err != nil {
		return TransitionResult{}, err
	}
	if err := repo.Clean(); err != nil {
		return TransitionResult{}, err
	}
	if report := Detect(root); report.State != Archived || report.GitArchiveCommit != commit {
		return TransitionResult{}, fmt.Errorf("committed archive did not validate as exact archived HEAD")
	}
	return TransitionResult{Message: "Archive transition completed at " + archiveRel, Committed: true, Commit: commit}, nil
}

func validateCommittedArchiveRetry(root string, repo *gitrepo.Repository, task taskMeta, archiveRel, head string) (TransitionResult, error) {
	wantSubject := "concoct: archive " + task.ID
	subject, err := repo.LastCommitSubject()
	if err != nil || subject != wantSubject {
		return TransitionResult{}, fmt.Errorf("archive completion has no authored changes and HEAD is not the exact archive transition commit")
	}
	parent, err := repo.Ref(head + "^")
	if err != nil {
		return TransitionResult{}, fmt.Errorf("cannot validate archive retry against its immutable parent: %w", err)
	}
	paths, err := repo.DiffPaths(parent, head)
	if err != nil {
		return TransitionResult{}, err
	}
	for path := range paths {
		p := filepath.ToSlash(path)
		if !(strings.HasPrefix(p, archiveRel+"/") || p == ".concoct/capabilities.md" || p == ".concoct/roadmap.md" || p == ".concoct/current/task-plan.md" || p == ".concoct/current/notes.md") {
			return TransitionResult{}, fmt.Errorf("committed Archivist transition contains forbidden path %s", p)
		}
	}
	if err := validateGitCapabilityDiff(repo, parent, task, archiveRel); err != nil {
		return TransitionResult{}, fmt.Errorf("committed archive capability transition is invalid: %w", err)
	}
	if err := validateGitRoadmapDiff(repo, parent, task); err != nil {
		return TransitionResult{}, fmt.Errorf("committed archive roadmap transition is invalid: %w", err)
	}
	if err := validateRoadmapEvidence(root, task, archiveRel, "active"); err != nil {
		return TransitionResult{}, err
	}
	report := Detect(root)
	if report.State != Archived || report.GitArchiveCommit != head {
		return TransitionResult{}, fmt.Errorf("clean archive retry does not validate as exact archived HEAD")
	}
	return TransitionResult{Message: "Archive transition already committed; reused clean valid transition at " + archiveRel, Committed: true, Commit: head}, nil
}

func validateNonGitDelivery(root string, task taskMeta, archiveRel string, summary archiveSummary) error {
	if summary.Status != "delivered" || summary.Delivery != "complete" {
		return fmt.Errorf("non-Git archive summary requires status delivered and delivery complete")
	}
	if err := validateRoadmapEvidence(root, task, archiveRel, "delivered"); err != nil {
		return err
	}
	return nil
}

func validateRoadmapEvidence(root string, task taskMeta, archiveRel, status string) error {
	b, err := os.ReadFile(filepath.Join(root, ".concoct/roadmap.md"))
	if err != nil {
		return err
	}
	items, diags := parseRoadmap(string(b))
	if len(diags) > 0 {
		return fmt.Errorf("invalid roadmap: %s", strings.Join(diags, "; "))
	}
	item, ok := items[task.RoadmapID]
	if !ok || item.Status != status {
		return fmt.Errorf("roadmap item %s must remain %s at this archival boundary", task.RoadmapID, status)
	}
	want := archiveRel + "/summary.md"
	if item.Archive != want {
		return fmt.Errorf("roadmap item %s Archive must reference %s", task.RoadmapID, want)
	}
	return nil
}

func validateCapabilityResult(root string, task taskMeta, archiveRel string) error {
	b, err := os.ReadFile(filepath.Join(root, ".concoct/capabilities.md"))
	if err != nil {
		return err
	}
	records, diags := parseCapabilities(string(b))
	if len(diags) > 0 {
		return fmt.Errorf("invalid capabilities: %s", strings.Join(diags, "; "))
	}
	for _, id := range task.Impact.IDs {
		rec, exists := records[id]
		if task.Impact.Type == "remove" {
			if exists {
				return fmt.Errorf("capability impact remove requires %s to be absent from active capability truth", id)
			}
			continue
		}
		if !exists || rec.Status != "active" {
			return fmt.Errorf("capability impact %s requires active record %s", task.Impact.Type, id)
		}
		want := archiveRel + "/summary.md"
		found := false
		for _, a := range rec.Archives {
			if a == want {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("capability %s must cite archive provenance %s", id, want)
		}
	}
	return nil
}

func validateGitCapabilityDiff(repo *gitrepo.Repository, baseline string, task taskMeta, archiveRel string) error {
	old, err := repo.FileAt(baseline, ".concoct/capabilities.md")
	if err != nil {
		return err
	}
	cur, err := os.ReadFile(filepath.Join(repo.Root, ".concoct/capabilities.md"))
	if err != nil {
		return err
	}
	before, _ := capabilitySections(string(old))
	after, _ := capabilitySections(string(cur))
	if err := validateCapabilitySectionChanges(before, after, task); err != nil {
		return err
	}
	declared := map[string]bool{}
	for _, id := range task.Impact.IDs {
		declared[id] = true
	}
	if !bytes.Equal(capabilityUndeclaredBytes(string(old), declared), capabilityUndeclaredBytes(string(cur), declared)) {
		return fmt.Errorf("capability ledger changed content outside declared capability records")
	}
	if task.Impact.Type == "none" && !bytes.Equal(old, cur) {
		return fmt.Errorf("capability-impact none forbids capability ledger changes")
	}
	return validateCapabilityResult(repo.Root, task, archiveRel)
}

func validateCapabilitySectionChanges(before, after map[string]string, task taskMeta) error {
	declared := map[string]bool{}
	for _, id := range task.Impact.IDs {
		declared[id] = true
	}
	changed := map[string]bool{}
	for id, text := range before {
		if after[id] != text {
			changed[id] = true
		}
	}
	for id, text := range after {
		if before[id] != text {
			changed[id] = true
		}
	}
	for id := range changed {
		if !declared[id] {
			return fmt.Errorf("capability ledger changed undeclared record %s", id)
		}
	}
	for _, id := range task.Impact.IDs {
		switch task.Impact.Type {
		case "add":
			if before[id] != "" || after[id] == "" {
				return fmt.Errorf("capability add requires new record %s", id)
			}
		case "update":
			if before[id] == "" || after[id] == "" || before[id] == after[id] {
				return fmt.Errorf("capability update requires changed existing record %s", id)
			}
		case "remove":
			if before[id] == "" || after[id] != "" {
				return fmt.Errorf("capability remove requires deleting existing record %s", id)
			}
		}
	}
	return nil
}

func capabilityUndeclaredBytes(s string, declared map[string]bool) []byte {
	matches := capabilityHeading.FindAllStringSubmatchIndex(s, -1)
	var b strings.Builder
	if len(matches) == 0 {
		return []byte(s)
	}
	b.WriteString(s[:matches[0][0]])
	for i, m := range matches {
		end := len(s)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		id := s[m[2]:m[3]]
		if !declared[id] {
			b.WriteString(s[m[0]:end])
		}
	}
	return []byte(b.String())
}

func validateGitRoadmapDiff(repo *gitrepo.Repository, baseline string, task taskMeta) error {
	old, err := repo.FileAt(baseline, ".concoct/roadmap.md")
	if err != nil {
		return err
	}
	cur, err := os.ReadFile(filepath.Join(repo.Root, ".concoct/roadmap.md"))
	if err != nil {
		return err
	}
	before, ok := normalizedRoadmapForArchive(string(old), task.RoadmapID)
	if !ok {
		return fmt.Errorf("roadmap baseline is missing selected item %s", task.RoadmapID)
	}
	after, ok := normalizedRoadmapForArchive(string(cur), task.RoadmapID)
	if !ok || before != after {
		return fmt.Errorf("roadmap changed content outside selected item status and Archive fields")
	}
	return nil
}

func normalizedRoadmapForArchive(s, selected string) (string, bool) {
	matches := itemHeading.FindAllStringSubmatchIndex(s, -1)
	for i, m := range matches {
		if s[m[2]:m[3]] != selected {
			continue
		}
		end := len(s)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		section := s[m[0]:end]
		statusRE := regexp.MustCompile(`(?m)^- Status:.*$`)
		archiveRE := regexp.MustCompile(`(?m)^- Archive:.*(?:\n|$)`)
		section = statusRE.ReplaceAllString(section, "- Status: <archive-transition>")
		section = archiveRE.ReplaceAllString(section, "")
		return s[:m[0]] + section + s[end:], true
	}
	return "", false
}

func capabilitySections(s string) (map[string]string, []string) {
	matches := capabilityHeading.FindAllStringSubmatchIndex(s, -1)
	out := map[string]string{}
	for i, m := range matches {
		end := len(s)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		out[s[m[2]:m[3]]] = s[m[0]:end]
	}
	return out, nil
}

func nonEmptySection(s, heading string) bool {
	marker := heading + "\n"
	start := strings.Index(s, marker)
	if start < 0 || (start > 0 && s[start-1] != '\n') {
		return false
	}
	body := s[start+len(marker):]
	if end := strings.Index(body, "\n## "); end >= 0 {
		body = body[:end]
	}
	return strings.TrimSpace(body) != ""
}
func sameStrings(a, b []string) bool {
	aa := append([]string{}, a...)
	bb := append([]string{}, b...)
	sort.Strings(aa)
	sort.Strings(bb)
	return strings.Join(aa, "\x00") == strings.Join(bb, "\x00")
}
func clearCurrent(cur string) error {
	entries, err := os.ReadDir(cur)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Name() != ".gitkeep" && e.Name() != "bootstrap-prompt.md" {
			if err := os.RemoveAll(filepath.Join(cur, e.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}
