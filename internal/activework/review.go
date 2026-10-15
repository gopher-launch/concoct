package activework

import (
	"fmt"
	"strings"
)

// TaskEvidence returns the integrity-checked active engineering record. The
// agreed scope is the immutable part preceding the engineering checkpoint.
func (o *Owner) TaskEvidence() (string, error) {
	st, err := o.currentState()
	if err != nil {
		return "", err
	}
	if st.TaskID == "" {
		return "", fmt.Errorf("run has no claimed task")
	}
	body, err := o.readBoundFile(st, "evidence/task.md")
	return string(body), err
}

func scopeDigest(task string) (string, error) {
	before, _, ok := strings.Cut(task, "## Engineering checkpoint\n")
	if !ok || !strings.Contains(before, "## Agreed objective\n") || !strings.Contains(before, "## Acceptance\n") {
		return "", fmt.Errorf("agreed task scope is missing")
	}
	return digest([]byte(before)), nil
}

// BeginReview is idempotent for a pending assessment and numbers each new
// frozen candidate after the last completed assessment.
func (o *Owner) BeginReview(attemptID string) (PendingReview, error) {
	st, err := o.currentState()
	if err != nil {
		return PendingReview{}, err
	}
	if st.Candidate == nil {
		return PendingReview{}, fmt.Errorf("review requires a frozen candidate")
	}
	if st.Review != nil && st.Review.Verdict == "" {
		if st.Review.Candidate != st.Candidate.Commit {
			return PendingReview{}, fmt.Errorf("pending review belongs to another candidate")
		}
		task, err := o.readBoundFile(st, "evidence/task.md")
		if err != nil {
			return PendingReview{}, err
		}
		scope, err := scopeDigest(string(task))
		if err != nil || scope != st.Review.ScopeSHA256 {
			return PendingReview{}, fmt.Errorf("pending review agreed scope changed")
		}
		return *st.Review, nil
	}
	if st.Phase != "ready-for-review" || st.Hold != "" || !namePattern.MatchString(attemptID) {
		return PendingReview{}, fmt.Errorf("run is not ready to begin review")
	}
	task, err := o.readBoundFile(st, "evidence/task.md")
	if err != nil {
		return PendingReview{}, err
	}
	scope, err := scopeDigest(string(task))
	if err != nil {
		return PendingReview{}, err
	}
	number := 1
	if st.Review != nil {
		if st.Review.Verdict != "changes_requested" || st.Review.Candidate == st.Candidate.Commit {
			return PendingReview{}, fmt.Errorf("new review requires a corrected candidate")
		}
		number = st.Review.Number + 1
	}
	if st.Dispatch == nil || number > st.Dispatch.Limits.Assessments || number > 99 {
		return PendingReview{}, fmt.Errorf("independent assessment allowance exhausted")
	}
	review := PendingReview{Number: number, Candidate: st.Candidate.Commit, ScopeSHA256: scope, AttemptID: attemptID}
	st.Review = &review
	st.Phase = "reviewing"
	st.Revision++
	if err := writeJSON(o.store.statePath(st.RunID), st); err != nil {
		return PendingReview{}, err
	}
	return review, nil
}

// RenewReviewAttempt keeps the interrupted assessment number and evidence,
// while binding one new recovery dispatch after the old attempt is resolved.
func (o *Owner) RenewReviewAttempt(attemptID string) error {
	st, err := o.currentState()
	if err != nil {
		return err
	}
	if st.Review == nil || st.Review.Verdict != "" || st.Phase != "reviewing" || st.Dispatch == nil || !namePattern.MatchString(attemptID) {
		return fmt.Errorf("no pending assessment for recovery")
	}
	old := st.Review.AttemptID
	resolved := false
	for _, attempt := range st.Dispatch.Attempts {
		if attempt.ID == old && (attempt.Status == "reconciled" || attempt.Status == "failed") {
			resolved = true
		}
	}
	if !resolved {
		return fmt.Errorf("prior review attempt is not reconciled")
	}
	st.Review.AttemptID = attemptID
	st.Hold, st.PendingRequest = "", ""
	st.Revision++
	return writeJSON(o.store.statePath(st.RunID), st)
}

func (o *Owner) ReplaceLostReviewThread(number int, priorID, nextID string) error {
	st, err := o.currentState()
	if err != nil {
		return err
	}
	if st.Review == nil || st.Review.Number != number || st.Review.Verdict != "" || (st.Review.ThreadID != priorID && !(number > 1 && st.Review.ThreadID == "")) || st.RoleIDs["iragent"] != priorID || nextID == "" || nextID == priorID || nextID == st.RoleIDs["reagent"] {
		return fmt.Errorf("review thread replacement is not bound")
	}
	st.Review.ThreadID, st.RoleIDs["iragent"] = nextID, nextID
	st.Revision++
	return writeJSON(o.store.statePath(st.RunID), st)
}

// BeginCorrection retains the reviewed candidate and findings while opening
// the checkpoint for a new revision. Repeated calls do not reset evidence.
func (o *Owner) BeginCorrection() error {
	st, err := o.currentState()
	if err != nil {
		return err
	}
	if st.Phase == "correcting" && st.Review != nil && st.Review.Verdict == "changes_requested" {
		return nil
	}
	if st.Phase != "changes-requested" || st.Hold != "" || st.Candidate == nil || st.Review == nil || st.Review.Verdict != "changes_requested" || st.Review.Candidate != st.Candidate.Commit {
		return fmt.Errorf("run has no completed changes-requested assessment")
	}
	if _, err := o.ReviewEvidence(st.Review.Number); err != nil {
		return err
	}
	st.Phase = "correcting"
	st.Revision++
	return writeJSON(o.store.statePath(st.RunID), st)
}

func (o *Owner) BindReviewThread(number int, id string) error {
	st, err := o.currentState()
	if err != nil {
		return err
	}
	if st.Review == nil || st.Review.Number != number || st.Review.Verdict != "" || id == "" || len(id) > 256 {
		return fmt.Errorf("invalid pending review thread binding")
	}
	if st.Review.ThreadID != "" && st.Review.ThreadID != id {
		return fmt.Errorf("review already belongs to another thread")
	}
	st.Review.ThreadID = id
	st.RoleIDs["iragent"] = id
	st.Revision++
	return writeJSON(o.store.statePath(st.RunID), st)
}

func (o *Owner) ReviewEvidence(number int) (string, error) {
	st, err := o.currentState()
	if err != nil {
		return "", err
	}
	if st.Review == nil || (st.Review.Number != number && !st.FinalReviews[number]) {
		return "", fmt.Errorf("review %02d is not pending", number)
	}
	body, err := o.readBoundFile(st, fmt.Sprintf("evidence/review-%02d.md", number))
	return string(body), err
}

// FinishAssessment consumes exactly one completed review dispatch and freezes
// the pending evidence. A blocker leaves the same review number pending.
func (o *Owner) FinishAssessment(number int, verdict, turnID string) error {
	st, policy, err := o.dispatchState()
	if err != nil {
		return err
	}
	if st.Review == nil || st.Review.Number != number || st.Review.Verdict != "" || st.Review.ThreadID == "" || turnID == "" || (verdict != "approved" && verdict != "changes_requested") {
		return fmt.Errorf("invalid completed review verdict")
	}
	path := fmt.Sprintf("evidence/review-%02d.md", number)
	if st.Digests[path] == "" {
		return fmt.Errorf("review %02d has no assessment", number)
	}
	assessment, err := o.readBoundFile(st, path)
	if err != nil {
		return err
	}
	bound := fmt.Sprintf("Candidate: %s\nAgreed scope SHA-256: %s\n", st.Review.Candidate, st.Review.ScopeSHA256)
	if !strings.Contains(string(assessment), bound) || !strings.HasSuffix(string(assessment), "\nVerdict: "+verdict+"\nReviewer turn: "+turnID+"\n") {
		return fmt.Errorf("review evidence lacks exact candidate, scope or verdict binding")
	}
	task, err := o.readBoundFile(st, "evidence/task.md")
	if err != nil {
		return err
	}
	scope, err := scopeDigest(string(task))
	if err != nil || scope != st.Review.ScopeSHA256 || st.Candidate == nil || st.Candidate.Commit != st.Review.Candidate {
		return fmt.Errorf("review candidate or agreed scope changed")
	}
	if err := policy.RecordAssessment(fmt.Sprintf("review-%02d", number), st.Review.AttemptID); err != nil {
		return err
	}
	st.Dispatch = &policy
	st.FinalReviews[number] = true
	st.Review.Verdict, st.Review.TurnID = verdict, turnID
	if verdict == "approved" {
		st.Phase = "ready-for-acceptance"
	} else {
		st.Phase = "changes-requested"
	}
	st.Hold, st.PendingRequest = "", ""
	st.Revision++
	return writeJSON(o.store.statePath(st.RunID), st)
}

func (o *Owner) HoldReview(reason string) error {
	if err := validateContent(reason); err != nil {
		return err
	}
	st, err := o.currentState()
	if err != nil {
		return err
	}
	if st.Review == nil || st.Review.Verdict != "" {
		return fmt.Errorf("no pending review to hold")
	}
	st.Phase, st.Hold, st.PendingRequest = "reviewing", "operator-decision", reason
	st.Revision++
	return writeJSON(o.store.statePath(st.RunID), st)
}
