package activework

import (
	"fmt"
	"path/filepath"
	"strings"
)

// AcceptanceInspection is a read-only view of the evidence bound to an
// acceptance token. It is safe to request while the execution lock is held.
type AcceptanceInspection struct {
	TaskPath, ReviewPath   string
	Checkpoint, Assessment string
}

func (s *Store) InspectAcceptance(snapshot Snapshot) (AcceptanceInspection, error) {
	if !snapshot.Active || snapshot.Acceptance == nil || snapshot.Candidate == nil || snapshot.Review == nil {
		return AcceptanceInspection{}, fmt.Errorf("no reviewed acceptance to inspect")
	}
	var st state
	if err := readJSON(s.statePath(snapshot.RunID), &st); err != nil {
		return AcceptanceInspection{}, err
	}
	if err := s.validateState(st, snapshot.RunID); err != nil {
		return AcceptanceInspection{}, err
	}
	if st.Acceptance == nil || *st.Acceptance != *snapshot.Acceptance || st.Candidate == nil || st.Candidate.Commit != snapshot.Candidate.Commit || st.Review == nil || st.Review.Number != snapshot.Review.Number {
		return AcceptanceInspection{}, fmt.Errorf("acceptance changed during inspection")
	}
	read := func(relative string) (string, string, error) {
		path := filepath.Join(s.runDir(st.RunID), relative)
		body, err := readRegular(path)
		if err != nil {
			return "", "", err
		}
		if digest(body) != st.Digests[relative] || st.Digests[relative] == "" {
			return "", "", fmt.Errorf("evidence digest mismatch for %s", relative)
		}
		return path, string(body), nil
	}
	taskPath, task, err := read("evidence/task.md")
	if err != nil {
		return AcceptanceInspection{}, err
	}
	reviewPath, review, err := read(fmt.Sprintf("evidence/review-%02d.md", st.Review.Number))
	if err != nil {
		return AcceptanceInspection{}, err
	}
	if digest([]byte(review)) != st.Acceptance.ReviewSHA256 || !strings.HasSuffix(review, fmt.Sprintf("\nVerdict: approved\nReviewer turn: %s\n", st.Acceptance.ReviewTurnID)) {
		return AcceptanceInspection{}, fmt.Errorf("final review differs from acceptance")
	}
	start := strings.Index(task, checkpointStart)
	end := strings.Index(task, checkpointEnd)
	if start < 0 || end <= start {
		return AcceptanceInspection{}, fmt.Errorf("engineering checkpoint is missing")
	}
	checkpoint := strings.TrimSpace(task[start+len(checkpointStart) : end])
	if digest([]byte(checkpoint)) != st.Candidate.CheckpointSHA256 {
		return AcceptanceInspection{}, fmt.Errorf("candidate checkpoint digest mismatch")
	}
	assessment, ok := strings.CutPrefix(review, fmt.Sprintf("# Independent review %02d\n\nCandidate: %s\nAgreed scope SHA-256: %s\n\n## Assessment\n\n", st.Review.Number, st.Candidate.Commit, st.Review.ScopeSHA256))
	if !ok {
		return AcceptanceInspection{}, fmt.Errorf("final review has wrong candidate or scope")
	}
	assessment = strings.TrimSuffix(assessment, fmt.Sprintf("\nVerdict: approved\nReviewer turn: %s\n", st.Acceptance.ReviewTurnID))
	return AcceptanceInspection{TaskPath: taskPath, ReviewPath: reviewPath, Checkpoint: checkpoint, Assessment: strings.TrimSpace(assessment)}, nil
}
