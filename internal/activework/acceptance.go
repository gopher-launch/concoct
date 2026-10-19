package activework

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// IssueAcceptance creates one durable operator token for the exact approved
// assessment, source tree and expected parent. Repeated presentation returns
// the same token while every binding remains unchanged.
func (o *Owner) IssueAcceptance() (Acceptance, error) {
	st, err := o.currentState()
	if err != nil {
		return Acceptance{}, err
	}
	if st.TaskID == "" || st.Phase != "ready-for-acceptance" || st.Hold != "" || st.Stop != nil || st.FreezeIntent != nil || st.Question != nil || st.Candidate == nil || st.Review == nil || st.Review.Verdict != "approved" || st.Review.Candidate != st.Candidate.Commit || st.Dispatch == nil || st.Dispatch.Pending != nil {
		return Acceptance{}, fmt.Errorf("run has no settled approved candidate")
	}
	completed := false
	for _, attempt := range st.Dispatch.Attempts {
		if attempt.Status == "reserved" || attempt.Status == "uncertain" {
			return Acceptance{}, fmt.Errorf("execution is not settled")
		}
		if attempt.ID == st.Review.AttemptID && attempt.Status == "completed" && attempt.FinalStatus == "completed" && attempt.Applied && attempt.TurnID == st.Review.TurnID && string(attempt.Binding.Role) == "iragent" && attempt.Binding.Candidate == st.Candidate.Commit {
			completed = true
		}
	}
	if !completed {
		return Acceptance{}, fmt.Errorf("approving assessment lacks completed dispatch")
	}
	task, err := o.readBoundFile(st, "evidence/task.md")
	if err != nil {
		return Acceptance{}, err
	}
	scope, err := scopeDigest(string(task))
	if err != nil || scope != st.Review.ScopeSHA256 {
		return Acceptance{}, fmt.Errorf("agreed scope differs from approved review")
	}
	reviewPath := fmt.Sprintf("evidence/review-%02d.md", st.Review.Number)
	review, err := o.readBoundFile(st, reviewPath)
	if err != nil {
		return Acceptance{}, err
	}
	if !strings.Contains(string(review), fmt.Sprintf("Candidate: %s\nAgreed scope SHA-256: %s\n", st.Candidate.Commit, scope)) || !strings.HasSuffix(string(review), fmt.Sprintf("\nVerdict: approved\nReviewer turn: %s\n", st.Review.TurnID)) {
		return Acceptance{}, fmt.Errorf("review evidence differs from approval")
	}
	bound := Acceptance{TaskID: st.TaskID, ScopeSHA256: scope, Candidate: st.Candidate.Commit, CandidateTree: st.Candidate.Tree, ReviewNumber: st.Review.Number, ReviewAttemptID: st.Review.AttemptID, ReviewTurnID: st.Review.TurnID, ReviewSHA256: digest(review), TargetBranch: st.ParentBranch, ExpectedParent: st.ParentCommit}
	if st.Acceptance != nil {
		bound.ID = st.Acceptance.ID
		if bound != *st.Acceptance {
			return Acceptance{}, fmt.Errorf("existing acceptance has a different binding")
		}
		return bound, nil
	}
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return Acceptance{}, err
	}
	bound.ID = "accept-" + hex.EncodeToString(token[:])
	st.Acceptance = &bound
	st.Revision++
	if err := writeJSON(o.store.statePath(st.RunID), st); err != nil {
		return Acceptance{}, err
	}
	return bound, nil
}

// BeginIntegration durably records the accepted identity before Git mutation.
func (o *Owner) BeginIntegration(id string) (Acceptance, error) {
	a, err := o.IssueAcceptance()
	if err != nil {
		return Acceptance{}, err
	}
	if id == "" || id != a.ID {
		return Acceptance{}, fmt.Errorf("acceptance ID does not match the reviewed candidate and target")
	}
	st, err := o.currentState()
	if err != nil {
		return Acceptance{}, err
	}
	if st.Integration != nil {
		if st.Integration.AcceptanceID != id {
			return Acceptance{}, fmt.Errorf("another integration intent is active")
		}
		return a, nil
	}
	st.Integration = &Integration{AcceptanceID: id, Status: "intent"}
	st.Revision++
	return a, writeJSON(o.store.statePath(st.RunID), st)
}

func (o *Owner) MarkIntegrated(id string) error {
	st, err := o.currentState()
	if err != nil {
		return err
	}
	if st.Integration == nil || st.Acceptance == nil || st.Integration.AcceptanceID != id || st.Acceptance.ID != id {
		return fmt.Errorf("integration has no matching durable intent")
	}
	if st.Integration.Status == "integrated" {
		return nil
	}
	st.Integration.Status = "integrated"
	st.Revision++
	return writeJSON(o.store.statePath(st.RunID), st)
}
