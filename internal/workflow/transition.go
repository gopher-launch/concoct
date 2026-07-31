package workflow

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gopher-launch/concoct/internal/gitrepo"
)

const reservationStatus = "reserved"

type TransitionResult struct {
	Message, Commit string
	Committed       bool
}

func CompleteDeveloper(root string) (TransitionResult, error) {
	gc, repo, entries, retry, err := transitionRepository(root, "code")
	if err != nil || retry != nil {
		if retry != nil {
			return *retry, nil
		}
		return TransitionResult{}, err
	}
	if repo != nil && len(entries) == 0 {
		return TransitionResult{}, fmt.Errorf("developer completion has no authored changes; update the implementation, task plan, and notes before retrying")
	}
	taskChanged, notesChanged := false, false
	for _, entry := range entries {
		for _, path := range entry.Paths {
			path = filepath.ToSlash(path)
			switch {
			case path == ".concoct/current/task-plan.md":
				taskChanged = true
			case path == ".concoct/current/notes.md":
				notesChanged = true
			case strings.HasPrefix(path, ".concoct/current/review-"):
				return TransitionResult{}, fmt.Errorf("Developer may not modify completed or reserved review artifact %s", path)
			case path == ".concoct/roadmap.md", path == ".concoct/capabilities.md", strings.HasPrefix(path, ".concoct/archive/"):
				return TransitionResult{}, fmt.Errorf("Developer transition contains forbidden workflow path %s", path)
			}
		}
	}
	if repo != nil && (!taskChanged || !notesChanged) {
		return TransitionResult{}, fmt.Errorf("developer completion requires fresh changes to both .concoct/current/task-plan.md and .concoct/current/notes.md")
	}
	notes, err := os.ReadFile(filepath.Join(root, ".concoct/current/notes.md"))
	if err != nil {
		return TransitionResult{}, err
	}
	handoff, err := reviewerHandoff(notes)
	if err != nil {
		return TransitionResult{}, err
	}
	for _, heading := range []string{"### Implemented", "### Verification", "### Known risks", "### Capability impact", "### Suggested review focus"} {
		if !bytes.Contains(handoff, []byte(heading)) {
			return TransitionResult{}, fmt.Errorf("notes.md lacks required fresh reviewer handoff heading %q", heading)
		}
	}
	if repo != nil {
		committedNotes, err := repo.FileAt("HEAD", ".concoct/current/notes.md")
		if err != nil {
			return TransitionResult{}, fmt.Errorf("read committed notes for reviewer handoff freshness: %w", err)
		}
		committedHandoff, committedErr := reviewerHandoff(committedNotes)
		if committedErr == nil && bytes.Equal(handoff, committedHandoff) {
			return TransitionResult{}, fmt.Errorf("developer completion requires a fresh reviewer handoff; the current handoff is unchanged from HEAD")
		}
	}
	report := Detect(root)
	if report.State != Complete {
		return TransitionResult{}, fmt.Errorf("developer output is not a valid implementation-complete transition: %s", strings.Join(report.Diagnostics, "; "))
	}
	return commitTransition(repo, gc, "concoct: complete "+gc.ID+" implementation", "Developer transition completed")
}

func reviewerHandoff(notes []byte) ([]byte, error) {
	const heading = "## Handoff to reviewer"
	start := bytes.LastIndex(notes, []byte(heading))
	if start < 0 {
		return nil, fmt.Errorf("notes.md lacks required fresh reviewer handoff heading %q", heading)
	}
	return bytes.TrimSpace(notes[start:]), nil
}

func ReserveReview(root string) (TransitionResult, error) {
	context, err := InspectPromptContext(root)
	if err != nil {
		return TransitionResult{}, err
	}
	if context.Report.State != Complete {
		return TransitionResult{}, fmt.Errorf("review reservation requires implementation-complete state; found %s", context.Report.State)
	}
	path := filepath.Join(root, filepath.FromSlash(context.NextReview))
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return TransitionResult{}, fmt.Errorf("reserve %s without overwriting: %w", context.NextReview, err)
	}
	num := len(context.ReviewFiles) + 1
	body := fmt.Sprintf("---\ntask-id: %s\nreview: %d\nstatus: %s\ncreated: %s\npersona: reviewer\n---\n\n# Review %02d\n\n<!-- Replace status: reserved with exactly one supported outcome and complete the review. -->\n", context.Report.RoadmapItem, num, reservationStatus, time.Now().Format("2006-01-02"), num)
	if _, err = f.WriteString(body); err != nil {
		_ = f.Close()
		return TransitionResult{}, err
	}
	if err = f.Close(); err != nil {
		return TransitionResult{}, err
	}
	return TransitionResult{Message: "Reserved " + context.NextReview + "; complete that file, then run concoct review --complete"}, nil
}

func CompleteReview(root string) (TransitionResult, error) {
	gc, repo, entries, retry, err := transitionRepository(root, "review")
	if err != nil || retry != nil {
		if retry != nil {
			return *retry, nil
		}
		return TransitionResult{}, err
	}
	if repo != nil && (len(entries) != 1 || len(entries[0].Paths) != 1 || !regexp.MustCompile(`^\.concoct/current/review-[0-9]{2}\.md$`).MatchString(filepath.ToSlash(entries[0].Paths[0]))) {
		return TransitionResult{}, fmt.Errorf("Reviewer may finalize only the one reserved next review artifact")
	}
	report := Detect(root)
	if report.State != Approved && report.State != ChangesRequested && report.State != Blocked {
		return TransitionResult{}, fmt.Errorf("review output is not a valid finalized outcome: %s", strings.Join(report.Diagnostics, "; "))
	}
	reviewPath := ".concoct/current/" + report.LatestReview
	if repo != nil {
		reviewPath = filepath.ToSlash(entries[0].Paths[0])
	}
	reviewData, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(reviewPath)))
	if err != nil {
		return TransitionResult{}, err
	}
	if !strings.Contains(string(reviewData), "Replace status: reserved") {
		return TransitionResult{}, fmt.Errorf("review was not created from the exclusive Concoct reservation; run concoct review --reserve")
	}
	return commitTransition(repo, gc, "concoct: record "+report.LatestReview+" for "+gc.ID, "Reviewer transition completed with outcome "+report.ReviewOutcome)
}

func transitionRepository(root, role string) (GitContext, *gitrepo.Repository, []gitrepo.StatusEntry, *TransitionResult, error) {
	gc, err := InspectGitContext(root)
	if err != nil {
		return gc, nil, nil, nil, err
	}
	if !gc.Enabled {
		return gc, nil, nil, nil, nil
	}
	repo, ok, err := gitrepo.Open(root)
	if err != nil || !ok {
		return gc, nil, nil, nil, fmt.Errorf("Git-backed transition requires the recorded repository")
	}
	if repo.OperationInProgress() {
		return gc, nil, nil, nil, fmt.Errorf("an unrelated Git operation is in progress")
	}
	branch, err := repo.Branch()
	if err != nil || branch != gc.TaskBranch {
		return gc, nil, nil, nil, fmt.Errorf("checkout drift: expected task branch %s", gc.TaskBranch)
	}
	head, err := repo.Head()
	if err != nil {
		return gc, nil, nil, nil, err
	}
	ancestor, err := repo.IsAncestor(gc.Base, head)
	if err != nil || !ancestor {
		return gc, nil, nil, nil, fmt.Errorf("recorded Git base is not an ancestor of the task branch")
	}
	entries, err := repo.StatusEntries()
	if err != nil {
		return gc, nil, nil, nil, err
	}
	if len(entries) == 0 {
		message, err := repo.LastCommitSubject()
		prefix := "concoct: complete " + gc.ID + " implementation"
		if role == "review" {
			prefix = "concoct: record review-"
		}
		report := Detect(root)
		validState := role == "code" && report.State == Complete || role == "review" && (report.State == Approved || report.State == ChangesRequested || report.State == Blocked)
		if err == nil && validState && strings.HasPrefix(message, prefix) && strings.Contains(message, gc.ID) {
			label := "Developer"
			if role == "review" {
				label = "Reviewer"
			}
			result := TransitionResult{Message: label + " transition already committed; reused clean valid transition", Committed: true, Commit: head}
			return gc, repo, entries, &result, nil
		}
	}
	return gc, repo, entries, nil, nil
}

func commitTransition(repo *gitrepo.Repository, gc GitContext, message, result string) (TransitionResult, error) {
	if repo == nil {
		return TransitionResult{Message: result}, nil
	}
	if err := repo.AddAll(); err != nil {
		return TransitionResult{}, err
	}
	if err := repo.Commit(message); err != nil {
		return TransitionResult{}, err
	}
	head, err := repo.Head()
	if err != nil {
		return TransitionResult{}, err
	}
	if err := repo.Clean(); err != nil {
		return TransitionResult{}, err
	}
	return TransitionResult{Message: result, Committed: true, Commit: head}, nil
}
