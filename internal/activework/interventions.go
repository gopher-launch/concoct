package activework

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gopher-launch/concoct/internal/codexapp"
)

type Disposition struct {
	TaskID, RunID, Repository, Status, Reason string
	At                                        time.Time
}

func (s *Store) DeferredTaskIDs() (map[string]bool, error) {
	return s.dispositionTaskIDs("deferred")
}

// UnavailableTaskIDs excludes deferred and terminally abandoned tasks from
// automatic selection. Deferred work is reactivated only by explicit request.
func (s *Store) UnavailableTaskIDs() (map[string]bool, error) {
	result, err := s.dispositionTaskIDs("deferred")
	if err != nil {
		return nil, err
	}
	abandoned, err := s.dispositionTaskIDs("abandoned")
	if err != nil {
		return nil, err
	}
	for id := range abandoned {
		result[id] = true
	}
	return result, nil
}

func (s *Store) dispositionTaskIDs(kind string) (map[string]bool, error) {
	result := map[string]bool{}
	entries, err := os.ReadDir(filepath.Join(s.dir, kind))
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			return nil, fmt.Errorf("invalid deferred record entry")
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		var disposition Disposition
		if !taskPattern.MatchString(id) || readJSON(filepath.Join(s.dir, kind, entry.Name()), &disposition) != nil || disposition.TaskID != id || disposition.Repository != s.Repository || disposition.Status != kind || !validRunID(disposition.RunID) {
			return nil, fmt.Errorf("invalid %s task binding %q", kind, id)
		}
		result[id] = true
	}
	return result, nil
}

func validateQuestionState(q *Question, st state) error {
	if q == nil {
		return nil
	}
	if !namePattern.MatchString(q.ID) || (q.Requester != "reagent" && q.Requester != "iragent") || st.RoleIDs[q.Requester] == "" || q.Requester == "iragent" && st.Review == nil || len(q.Options) > 8 || q.SourceChange && q.Requester != "iragent" || q.ScopeChange && !q.SourceChange {
		return fmt.Errorf("invalid bound operator question")
	}
	bound := false
	if st.Dispatch != nil {
		for _, attempt := range st.Dispatch.Attempts {
			if attempt.ID == q.OriginAttemptID && string(attempt.Binding.Role) == q.Requester && attempt.Status == "completed" && attempt.FinalStatus == "completed" && attempt.Applied {
				bound = true
			}
		}
	}
	if !bound {
		return fmt.Errorf("operator question lacks a completed requester attempt")
	}
	for _, part := range []string{q.Objective, q.Evidence, q.Missing, q.SafeWork, q.NextStep} {
		if strings.TrimSpace(part) == "" || len(part) > 4096 || strings.Contains(part, "<!-- operator-decisions:") || strings.Contains(part, "<!-- engineering-checkpoint:") {
			return fmt.Errorf("operator question lacks bounded context")
		}
	}
	seen := map[string]bool{}
	for _, option := range q.Options {
		if !namePattern.MatchString(option.ID) || seen[option.ID] || option.Description == "" || option.Tradeoff == "" || len(option.Description) > 512 || len(option.Tradeoff) > 512 || strings.Contains(option.Description+option.Tradeoff, "<!--") {
			return fmt.Errorf("invalid question option")
		}
		seen[option.ID] = true
	}
	if q.AnswerMode == "" {
		if st.Hold != "operator-decision" || q.AnswerText != "" || q.AnswerChoice != "" {
			return fmt.Errorf("unanswered question is not held")
		}
	} else if q.AnswerMode != "text" && q.AnswerMode != "choice" && q.AnswerMode != "file" || strings.TrimSpace(q.AnswerText) == "" || len(q.AnswerText) > maxEvidence/2 || st.Hold == "operator-decision" {
		return fmt.Errorf("invalid durable answer")
	}
	return nil
}

func (o *Owner) AskRoleQuestion(role, attemptID string, source *codexapp.RoleQuestion) (Question, error) {
	if source == nil {
		return Question{}, fmt.Errorf("structured blocker details are required")
	}
	q := Question{Requester: role, OriginAttemptID: attemptID, Objective: source.Objective, Evidence: source.Evidence, Missing: source.Missing, SafeWork: source.SafeWork, NextStep: source.NextStep, SourceChange: source.SourceChange, ScopeChange: source.ScopeChange}
	for _, option := range source.Options {
		q.Options = append(q.Options, QuestionOption{ID: option.ID, Description: option.Description, Tradeoff: option.Tradeoff})
	}
	return o.askQuestion(q, attemptID)
}

func (o *Owner) askQuestion(q Question, appliedAttempt string) (Question, error) {
	st, err := o.currentState()
	if err != nil {
		return Question{}, err
	}
	if st.Stop != nil || st.Dispatch == nil || st.Hold != "" && st.Hold != "operator-decision" || st.Question != nil && st.Question.AnswerMode == "" {
		return Question{}, fmt.Errorf("another intervention or unresolved stop is active")
	}
	if st.Hold == "operator-decision" && st.Question == nil {
		return Question{}, fmt.Errorf("existing unstructured decision requires reconciliation")
	}
	for _, attempt := range st.Dispatch.Attempts {
		if attempt.Status == "reserved" || attempt.Status == "uncertain" {
			return Question{}, fmt.Errorf("cannot ask while a dispatch is unresolved")
		}
	}
	if q.ID == "" {
		var random [12]byte
		if _, err := rand.Read(random[:]); err != nil {
			return Question{}, err
		}
		q.ID = "question-" + hex.EncodeToString(random[:])
	}
	q.AnswerMode, q.AnswerChoice, q.AnswerText = "", "", ""
	if appliedAttempt != "" {
		if err := st.Dispatch.MarkApplied(appliedAttempt); err != nil {
			return Question{}, err
		}
	}
	st.Question, st.Hold = &q, "operator-decision"
	st.PendingRequest = "answer " + q.ID + " for " + q.Requester
	if err := validateQuestionState(st.Question, st); err != nil {
		return Question{}, err
	}
	st.Revision++
	if err := writeJSON(o.store.statePath(st.RunID), st); err != nil {
		return Question{}, err
	}
	return q, o.ensureQuestionEvidence()
}

func (o *Owner) AnswerQuestion(id, mode, value string) error {
	st, err := o.currentState()
	if err != nil {
		return err
	}
	if st.Question == nil || st.Question.ID != id || st.Question.AnswerMode != "" || st.Hold != "operator-decision" {
		return fmt.Errorf("stale or wrong question ID")
	}
	if st.Question.ScopeChange {
		return fmt.Errorf("question %s requests an agreed-scope revision; explicit scope authority is required", id)
	}
	if mode != "choice" && mode != "text" && mode != "file" || validateContent(value) != nil {
		return fmt.Errorf("invalid answer")
	}
	if mode == "choice" {
		valid := false
		for _, option := range st.Question.Options {
			if option.ID == value {
				valid = true
			}
		}
		if !valid {
			return fmt.Errorf("unknown option for question %s", id)
		}
		st.Question.AnswerChoice = value
	}
	st.Question.AnswerMode, st.Question.AnswerText = mode, value
	st.Hold, st.PendingRequest = "", ""
	if st.Dispatch != nil && st.Dispatch.Pending != nil && !st.Dispatch.Pending.Authorized {
		st.Hold, st.PendingRequest = "dispatch-authorization", "authorize existing bound dispatch "+st.Dispatch.Pending.ID
	}
	st.Revision++
	if err := writeJSON(o.store.statePath(st.RunID), st); err != nil {
		return err
	}
	return o.ensureQuestionEvidence()
}

func (o *Owner) RenewAnsweredReviewAttempt(questionID, nextAttemptID string) error {
	st, err := o.currentState()
	if err != nil {
		return err
	}
	if st.Question == nil || st.Question.ID != questionID || st.Question.AnswerMode == "" || st.Question.Requester != "iragent" || st.Question.SourceChange || st.Review == nil || st.Review.Verdict != "" || st.Review.AttemptID != st.Question.OriginAttemptID || st.Hold != "" || !namePattern.MatchString(nextAttemptID) {
		return fmt.Errorf("answered reviewer question is not ready for requester resume")
	}
	completed := false
	for _, attempt := range st.Dispatch.Attempts {
		if attempt.ID == st.Question.OriginAttemptID && attempt.Status == "completed" && attempt.Applied && attempt.Binding.Role == "iragent" {
			completed = true
		}
	}
	if !completed {
		return fmt.Errorf("requesting reviewer turn is not completed")
	}
	st.Review.AttemptID = nextAttemptID
	st.Revision++
	return writeJSON(o.store.statePath(st.RunID), st)
}

func (o *Owner) ClearAnsweredQuestion(id string) error {
	st, err := o.currentState()
	if err != nil {
		return err
	}
	if st.Question == nil || st.Question.ID != id {
		return nil
	}
	if st.Question.AnswerMode == "" || st.Hold == "operator-decision" {
		return fmt.Errorf("question remains unanswered")
	}
	st.Question = nil
	st.Revision++
	return writeJSON(o.store.statePath(st.RunID), st)
}

func (o *Owner) ensureQuestionEvidence() error {
	st, err := o.currentState()
	if err != nil || st.Question == nil || st.TaskID == "" {
		return err
	}
	body, err := o.TaskEvidence()
	if err != nil {
		return err
	}
	q := st.Question
	marker := "Question ID: " + q.ID
	if !strings.Contains(body, marker) {
		entry := fmt.Sprintf("%s\nRequester: %s\nBlocked objective: %s\nAttempts/evidence: %s\nMissing input/action: %s\nSafe remaining work: %s\nNext step: %s\n", marker, q.Requester, q.Objective, q.Evidence, q.Missing, q.SafeWork, q.NextStep)
		for _, option := range q.Options {
			entry += fmt.Sprintf("Option %s: %s (tradeoff: %s)\n", option.ID, option.Description, option.Tradeoff)
		}
		if err := o.RecordDecision(entry); err != nil {
			return err
		}
		body, err = o.TaskEvidence()
		if err != nil {
			return err
		}
	}
	answerMarker := "Answer to " + q.ID + ":"
	if q.AnswerMode != "" && !strings.Contains(body, answerMarker) {
		return o.RecordDecision(fmt.Sprintf("%s %s: %s\n", answerMarker, q.AnswerMode, q.AnswerText))
	}
	return nil
}

func (o *Owner) recoverInterventions() error {
	st, err := o.currentState()
	if err != nil {
		return err
	}
	if err := o.ensureQuestionEvidence(); err != nil {
		return err
	}
	for _, kind := range []string{"deferred", "abandoned"} {
		var disposition Disposition
		path := filepath.Join(o.store.dir, kind, st.TaskID+".json")
		if err := readJSON(path, &disposition); err == nil && disposition.RunID == st.RunID {
			return removeSync(filepath.Join(o.store.dir, "active.json"))
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (o *Owner) Retire(taskID, kind, reason string) (Disposition, error) {
	var result Disposition
	if kind != "deferred" && kind != "abandoned" || !taskPattern.MatchString(taskID) || strings.TrimSpace(reason) == "" || len(reason) > 2048 {
		return result, fmt.Errorf("invalid task disposition")
	}
	st, err := o.currentState()
	if err != nil {
		return result, err
	}
	if st.TaskID != taskID || st.Stop != nil || st.Dispatch == nil {
		return result, fmt.Errorf("task is not bound or execution stop is unresolved")
	}
	for _, attempt := range st.Dispatch.Attempts {
		if attempt.Status == "reserved" || attempt.Status == "uncertain" {
			return result, fmt.Errorf("execution has an unresolved dispatch")
		}
	}
	result = Disposition{TaskID: taskID, RunID: st.RunID, Repository: st.Repository, Status: kind, Reason: reason, At: time.Now().UTC()}
	path := filepath.Join(o.store.dir, kind, taskID+".json")
	other := "deferred"
	if kind == "deferred" {
		other = "abandoned"
	}
	if _, err := os.Stat(filepath.Join(o.store.dir, other, taskID+".json")); err == nil {
		return Disposition{}, fmt.Errorf("task already has a %s disposition", other)
	} else if !os.IsNotExist(err) {
		return Disposition{}, err
	}
	if _, err := os.Stat(path); err == nil {
		return Disposition{}, fmt.Errorf("task already has a %s disposition", kind)
	} else if !os.IsNotExist(err) {
		return Disposition{}, err
	}
	if err := writeJSON(path, result); err != nil {
		return Disposition{}, err
	}
	if err := removeSync(filepath.Join(o.store.dir, "active.json")); err != nil {
		return Disposition{}, err
	}
	return result, nil
}

func (o *Owner) Reactivate(taskID string) (Disposition, error) {
	var disposition Disposition
	if !taskPattern.MatchString(taskID) {
		return disposition, fmt.Errorf("invalid deferred task ID")
	}
	if _, err := os.Stat(filepath.Join(o.store.dir, "active.json")); err == nil {
		return disposition, fmt.Errorf("another run owns the repository")
	} else if !os.IsNotExist(err) {
		return disposition, err
	}
	path := filepath.Join(o.store.dir, "deferred", taskID+".json")
	if err := readJSON(path, &disposition); err != nil {
		return Disposition{}, err
	}
	if disposition.Status != "deferred" || disposition.TaskID != taskID || disposition.Repository != o.store.Repository || !validRunID(disposition.RunID) {
		return Disposition{}, fmt.Errorf("deferred binding differs from repository or task")
	}
	var st state
	if err := readJSON(o.store.statePath(disposition.RunID), &st); err != nil {
		return Disposition{}, err
	}
	if err := o.store.validateState(st, disposition.RunID); err != nil || st.TaskID != taskID {
		return Disposition{}, fmt.Errorf("deferred task state is invalid: %v", err)
	}
	if err := writeJSON(filepath.Join(o.store.dir, "reactivate.json"), disposition); err != nil {
		return Disposition{}, err
	}
	if err := o.recoverReactivation(); err != nil {
		return Disposition{}, err
	}
	return disposition, nil
}

func (o *Owner) recoverReactivation() error {
	var disposition Disposition
	intentPath := filepath.Join(o.store.dir, "reactivate.json")
	if err := readJSON(intentPath, &disposition); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if disposition.Status != "deferred" || !taskPattern.MatchString(disposition.TaskID) || !validRunID(disposition.RunID) || disposition.Repository != o.store.Repository {
		return fmt.Errorf("invalid deferred reactivation intent")
	}
	var st state
	if err := readJSON(o.store.statePath(disposition.RunID), &st); err != nil {
		return err
	}
	if err := o.store.validateState(st, disposition.RunID); err != nil || st.TaskID != disposition.TaskID {
		return fmt.Errorf("reactivation task binding changed: %v", err)
	}
	activePath := filepath.Join(o.store.dir, "active.json")
	var active pointer
	if err := readJSON(activePath, &active); err == nil {
		if active.RunID != disposition.RunID {
			return fmt.Errorf("another active run conflicts with reactivation")
		}
	} else if os.IsNotExist(err) {
		if err := writeJSON(activePath, pointer{RunID: disposition.RunID}); err != nil {
			return err
		}
	} else {
		return err
	}
	deferredPath := filepath.Join(o.store.dir, "deferred", disposition.TaskID+".json")
	if err := removeSync(deferredPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return removeSync(intentPath)
}
