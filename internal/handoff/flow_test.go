package handoff

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopher-launch/concoct/internal/codexapp"
	"github.com/gopher-launch/concoct/internal/stop"
)

type flowRunner struct {
	repo, root             string
	turns, starts, resumes int
	prompts                []string
	roles                  []string
	replacementRole        string
	replacementUsed        bool
	failReplacementStart   bool
	failedStartOnce        bool
	blocker                bool
	reviewBlocker          bool
	integrationFailure     bool
	staleReviewEvidence    bool
	linkedReviewEvidence   bool
	thirdReviewAlias       string
}

func (f *flowRunner) Models(context.Context) ([]codexapp.Model, error) { return model(), nil }
func (f *flowRunner) StartThread(_ context.Context, cwd string, s codexapp.RoleSettings, _ []codexapp.ToolSpec) (codexapp.Thread, error) {
	if cwd != f.repo {
		return codexapp.Thread{}, fmt.Errorf("wrong checkout")
	}
	if f.failReplacementStart && f.replacementUsed && !f.failedStartOnce {
		f.failedStartOnce = true
		return codexapp.Thread{}, errors.New("replacement launch unavailable")
	}
	f.starts++
	return codexapp.Thread{ID: fmt.Sprintf("%s-%d", s.Role, f.starts)}, nil
}
func (f *flowRunner) ResumeThread(_ context.Context, id string, hadTurn bool, s codexapp.RoleSettings) (codexapp.Thread, error) {
	f.resumes++
	if !hadTurn {
		return codexapp.Thread{}, fmt.Errorf("completed role turn was not retained")
	}
	if f.replacementRole == s.Role && !f.replacementUsed {
		f.replacementUsed = true
		return codexapp.Thread{}, errors.New("session unavailable")
	}
	return codexapp.Thread{ID: id}, nil
}
func (f *flowRunner) Stop(context.Context, string, string) stop.Result {
	return stop.Result{Status: "stopped_safely"}
}
func (f *flowRunner) Close() error { return nil }
func flowResult(disposition, summary, task, candidate string, findings []string, question *codexapp.RoleQuestion) string {
	body, _ := json.Marshal(map[string]any{"disposition": disposition, "summary": summary, "task_id": task, "candidate": candidate, "findings": findings, "evidence": nil, "question": question})
	return string(body)
}
func (f *flowRunner) RunRoleTurn(_ context.Context, id, prompt string, s codexapp.RoleSettings, onEvent func(codexapp.Event)) (codexapp.TurnOutcome, error) {
	f.prompts = append(f.prompts, prompt)
	f.roles = append(f.roles, s.Role)
	step := f.turns
	f.turns++
	onEvent(codexapp.Event{Kind: "turn/started", ThreadID: id, TurnID: fmt.Sprintf("turn-%d", step+1)})
	taskEvidence := filepath.Join(f.root, "evidence", "APP-002", "task.md")
	reviewEvidence := func(n int) string { return reviewPath(f.root, "APP-002", n) }
	var result string
	switch step {
	case 0:
		if s.Role != "reagent" {
			panic("expected reagent selection")
		}
		if f.blocker {
			result = flowResult("decision_required", "Need a mode", "", "", nil, &codexapp.RoleQuestion{Text: "Which mode?"})
			break
		}
		gitTest(f.repo, "switch", "-qc", "task/APP-002")
		if err := os.WriteFile(filepath.Join(f.repo, "feature.txt"), []byte("initial\n"), 0600); err != nil {
			panic(err)
		}
		gitTest(f.repo, "add", "feature.txt")
		gitTest(f.repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "initial")
		writeEvidence(taskEvidence)
		result = flowResult("ready_for_review", "Initial implementation", "APP-002", gitTest(f.repo, "rev-parse", "HEAD"), nil, nil)
	case 1:
		if f.blocker {
			if s.Role != "reagent" || !strings.Contains(prompt, "Use safe mode") {
				panic("blocker answer was not routed to reagent")
			}
			gitTest(f.repo, "switch", "-qc", "task/APP-002")
			if err := os.WriteFile(filepath.Join(f.repo, "feature.txt"), []byte("answered\n"), 0600); err != nil {
				panic(err)
			}
			gitTest(f.repo, "add", "feature.txt")
			gitTest(f.repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "answered")
			writeEvidence(taskEvidence)
			result = flowResult("ready_for_review", "Answered implementation", "APP-002", gitTest(f.repo, "rev-parse", "HEAD"), nil, nil)
			break
		}
		if f.reviewBlocker {
			result = flowResult("decision_required", "Need review input", "APP-002", gitTest(f.repo, "rev-parse", "HEAD"), nil, &codexapp.RoleQuestion{Text: "Which review mode?"})
			break
		}
		if s.Role != "iragent" {
			panic("expected review")
		}
		writeEvidence(reviewEvidence(1))
		result = flowResult("changes_requested", "One correction needed", "APP-002", gitTest(f.repo, "rev-parse", "HEAD"), []string{"Update feature text"}, nil)
	case 2:
		if f.blocker {
			if s.Role != "iragent" {
				panic("expected review after answer")
			}
			writeEvidence(reviewEvidence(1))
			result = flowResult("approved", "Approved after answer", "APP-002", gitTest(f.repo, "rev-parse", "HEAD"), nil, nil)
			break
		}
		if f.reviewBlocker {
			if s.Role != "iragent" || !strings.Contains(prompt, "Use strict review") {
				panic("reviewer answer was not routed")
			}
			writeEvidence(reviewEvidence(1))
			result = flowResult("approved", "Approved after review answer", "APP-002", gitTest(f.repo, "rev-parse", "HEAD"), nil, nil)
			break
		}
		if s.Role != "reagent" || !strings.Contains(prompt, "Update feature text") || !strings.Contains(prompt, reviewEvidence(1)) {
			panic("correction context missing")
		}
		if err := os.WriteFile(filepath.Join(f.repo, "feature.txt"), []byte("corrected\n"), 0600); err != nil {
			panic(err)
		}
		gitTest(f.repo, "add", "feature.txt")
		gitTest(f.repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "corrected")
		writeEvidence(taskEvidence)
		result = flowResult("ready_for_review", "Corrected candidate", "APP-002", gitTest(f.repo, "rev-parse", "HEAD"), nil, nil)
	case 3:
		if s.Role != "iragent" {
			panic("expected reassessment")
		}
		if f.thirdReviewAlias != "" {
			writeEvidence(reviewEvidence(2))
			result = flowResult("changes_requested", "Another correction needed", "APP-002", gitTest(f.repo, "rev-parse", "HEAD"), []string{"Revise feature again"}, nil)
			break
		}
		if f.staleReviewEvidence {
			body, _ := json.Marshal(map[string]any{"disposition": "approved", "summary": "Claimed reassessment", "task_id": "APP-002", "candidate": gitTest(f.repo, "rev-parse", "HEAD"), "findings": nil, "evidence": reviewEvidence(1), "question": nil})
			result = string(body)
			break
		}
		if f.linkedReviewEvidence {
			if err := os.Symlink(reviewEvidence(1), reviewEvidence(2)); err != nil {
				panic(err)
			}
			result = flowResult("approved", "Claimed linked reassessment", "APP-002", gitTest(f.repo, "rev-parse", "HEAD"), nil, nil)
			break
		}
		writeEvidence(reviewEvidence(2))
		result = flowResult("approved", "Corrected candidate approved", "APP-002", gitTest(f.repo, "rev-parse", "HEAD"), nil, nil)
	case 4:
		if f.thirdReviewAlias != "" {
			if s.Role != "reagent" {
				panic("expected second correction")
			}
			if err := os.WriteFile(filepath.Join(f.repo, "feature.txt"), []byte("corrected again\n"), 0600); err != nil {
				panic(err)
			}
			gitTest(f.repo, "add", "feature.txt")
			gitTest(f.repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "corrected again")
			writeEvidence(taskEvidence)
			result = flowResult("ready_for_review", "Second corrected candidate", "APP-002", gitTest(f.repo, "rev-parse", "HEAD"), nil, nil)
			break
		}
		if s.Role != "reagent" {
			panic("expected integration reagent")
		}
		candidate := gitTest(f.repo, "rev-parse", "HEAD")
		if !strings.Contains(prompt, candidate) {
			panic("integration prompt omitted approved commit")
		}
		if f.integrationFailure {
			result = flowResult("integration_failed", "Parent branch moved; no integration", "APP-002", candidate, nil, nil)
			break
		}
		base := gitTest(f.repo, "rev-parse", "main")
		gitTest(f.repo, "update-ref", "refs/heads/main", candidate, base)
		gitTest(f.repo, "switch", "main")
		result = flowResult("integrated", "Integrated locally", "APP-002", candidate, nil, nil)
	case 5:
		if f.thirdReviewAlias == "" || s.Role != "iragent" {
			panic("unexpected third review")
		}
		var err error
		if f.thirdReviewAlias == "symlink" {
			err = os.Symlink(reviewEvidence(1), reviewEvidence(3))
		} else {
			err = os.Link(reviewEvidence(1), reviewEvidence(3))
		}
		if err != nil {
			panic(err)
		}
		result = flowResult("approved", "Claimed third assessment", "APP-002", gitTest(f.repo, "rev-parse", "HEAD"), nil, nil)
	default:
		panic("unexpected extra turn")
	}
	return codexapp.TurnOutcome{ThreadID: id, TurnID: fmt.Sprintf("turn-%d", step+1), Status: "completed", FinalText: result}, nil
}
func testFlow(t *testing.T, options Options, f *flowRunner) (State, error) {
	t.Helper()
	return Run(context.Background(), options, func(context.Context, string, io.Writer) (Runner, error) { return f, nil })
}
func TestRequestedChangesCorrectionReassessmentApprovalIntegration(t *testing.T) {
	repo := testRepo(t)
	root := t.TempDir()
	dir, err := storage(repo, root)
	if err != nil {
		t.Fatal(err)
	}
	f := &flowRunner{repo: repo, root: dir}
	o := Options{Repository: repo, Root: root}
	s, err := testFlow(t, o, f)
	if err != nil {
		t.Fatal(err)
	}
	if s.Phase != "awaiting_approval" || s.Assessments != 2 || s.Attempts != 4 || s.ApprovedCommit != s.Candidate || s.ReagentSummary != "Corrected candidate" || s.ReviewSummary != "Corrected candidate approved" || s.ReviewEvidence != reviewPath(dir, "APP-002", 2) {
		t.Fatalf("approval boundary: %+v", s)
	}
	if f.starts != 2 || f.resumes != 2 {
		t.Fatalf("roles were not reused: starts=%d resumes=%d", f.starts, f.resumes)
	}
	if _, err := testFlow(t, Options{Repository: repo, Root: root, Approve: "wrong"}, f); err == nil || f.turns != 4 {
		t.Fatalf("stale approval launched agent: %v", err)
	}
	candidate := s.Candidate
	s, err = testFlow(t, Options{Repository: repo, Root: root, Approve: candidate}, f)
	if err != nil {
		t.Fatal(err)
	}
	if s.Phase != "integrated" || s.AuthorizedCommit != candidate || s.Attempts != 5 || f.turns != 5 || gitTest(repo, "rev-parse", "main") != candidate || gitTest(repo, "branch", "--show-current") != "main" {
		t.Fatalf("integration result: %+v", s)
	}
}
func TestBlockerAnswerContinuesSameTask(t *testing.T) {
	repo := testRepo(t)
	root := t.TempDir()
	dir, err := storage(repo, root)
	if err != nil {
		t.Fatal(err)
	}
	f := &flowRunner{repo: repo, root: dir, blocker: true}
	s, err := testFlow(t, Options{Repository: repo, Root: root}, f)
	if err != nil || s.Phase != "blocked" || s.Question == nil || s.Question.Text != "Which mode?" || s.Attempts != 1 {
		t.Fatalf("blocker: %+v %v", s, err)
	}
	s, err = testFlow(t, Options{Repository: repo, Root: root, Answer: "Use safe mode"}, f)
	if err != nil || s.Phase != "awaiting_approval" || s.Attempts != 3 || s.Assessments != 1 || s.TaskID != "APP-002" || f.starts != 2 || f.resumes != 1 {
		t.Fatalf("answer routing: %+v %v", s, err)
	}
}
func TestSessionReplacementKeepsCountsAndContext(t *testing.T) {
	for _, role := range []string{"reagent", "iragent"} {
		t.Run(role, func(t *testing.T) {
			repo := testRepo(t)
			root := t.TempDir()
			dir, err := storage(repo, root)
			if err != nil {
				t.Fatal(err)
			}
			f := &flowRunner{repo: repo, root: dir, replacementRole: role}
			s, err := testFlow(t, Options{Repository: repo, Root: root}, f)
			if err != nil || s.Phase != "awaiting_approval" || s.Attempts != 4 || s.Assessments != 2 || f.starts != 3 || !f.replacementUsed {
				t.Fatalf("replacement: %+v %v starts=%d", s, err, f.starts)
			}
			step := 2
			if role == "iragent" {
				step = 3
			}
			if !strings.Contains(f.prompts[step], s.Evidence) || !strings.Contains(f.prompts[step], s.TaskID) {
				t.Fatal("replacement prompt lacked task and evidence context")
			}
		})
	}
}
func TestCompletedReviewBoundaryContinuesAfterLaunchFailure(t *testing.T) {
	repo := testRepo(t)
	root := t.TempDir()
	dir, err := storage(repo, root)
	if err != nil {
		t.Fatal(err)
	}
	f := &flowRunner{repo: repo, root: dir, replacementRole: "reagent", failReplacementStart: true}
	s, err := testFlow(t, Options{Repository: repo, Root: root}, f)
	if err == nil || s.Phase != "correcting" || s.Assessments != 1 || s.Attempts != 2 || f.turns != 2 {
		t.Fatalf("completed review boundary: %+v %v", s, err)
	}
	s, err = testFlow(t, Options{Repository: repo, Root: root}, f)
	if err != nil || s.Phase != "awaiting_approval" || s.Assessments != 2 || s.Attempts != 4 || f.turns != 4 {
		t.Fatalf("continued review cycle: %+v %v", s, err)
	}
}
func TestReassessmentRequiresDistinctCurrentReviewFile(t *testing.T) {
	for _, tc := range []struct {
		name   string
		linked bool
	}{
		{"old reported path", false},
		{"current path linked to old file", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := testRepo(t)
			root := t.TempDir()
			dir, err := storage(repo, root)
			if err != nil {
				t.Fatal(err)
			}
			f := &flowRunner{repo: repo, root: dir, staleReviewEvidence: !tc.linked, linkedReviewEvidence: tc.linked}
			s, err := testFlow(t, Options{Repository: repo, Root: root}, f)
			if err == nil || s.Phase != "invalid_result" || s.Assessments != 1 || s.ApprovedCommit != "" || f.turns != 4 {
				t.Fatalf("stale reassessment accepted: %+v %v", s, err)
			}
		})
	}
}
func TestThirdAssessmentCannotReuseFirstFile(t *testing.T) {
	for _, alias := range []string{"symlink", "hardlink"} {
		t.Run(alias, func(t *testing.T) {
			repo := testRepo(t)
			root := t.TempDir()
			dir, err := storage(repo, root)
			if err != nil {
				t.Fatal(err)
			}
			f := &flowRunner{repo: repo, root: dir, thirdReviewAlias: alias}
			s, err := testFlow(t, Options{Repository: repo, Root: root}, f)
			if err == nil || s.Phase != "invalid_result" || s.Assessments != 2 || s.ApprovedCommit != "" || f.turns != 6 {
				t.Fatalf("nonadjacent review reuse accepted: %+v %v", s, err)
			}
		})
	}
}
func TestApprovedReviewWithNoIntegrationInvocationStopsBeforeApproval(t *testing.T) {
	repo := testRepo(t)
	root := t.TempDir()
	dir, err := storage(repo, root)
	if err != nil {
		t.Fatal(err)
	}
	nominal := &fakeRunner{repo: repo, evidenceDir: dir, models: model(), output: []string{result("ready_for_review", "Ready", "APP-002", "CANDIDATE"), result("approved", "Approved", "APP-002", "CANDIDATE")}}
	open := func(context.Context, string, io.Writer) (Runner, error) { return nominal, nil }
	s, err := Run(context.Background(), Options{Repository: repo, Root: root, MaxAttempts: 2}, open)
	if err != nil || s.Phase != "limit_exhausted" || s.Verdict != "approved" || s.ApprovedCommit != s.Candidate || s.Attempts != 2 || !strings.Contains(s.LastError, "no integration turn") {
		t.Fatalf("exhausted approval boundary: %+v %v", s, err)
	}
	s, err = Run(context.Background(), Options{Repository: repo, Root: root, Approve: s.ApprovedCommit}, open)
	if err == nil || s.Phase != "limit_exhausted" || nominal.turns != 2 {
		t.Fatalf("approval consumed exhausted budget: %+v %v", s, err)
	}
}
func TestReviewerBlockerAnswerReturnsToReviewer(t *testing.T) {
	repo := testRepo(t)
	root := t.TempDir()
	dir, err := storage(repo, root)
	if err != nil {
		t.Fatal(err)
	}
	f := &flowRunner{repo: repo, root: dir, reviewBlocker: true}
	s, err := testFlow(t, Options{Repository: repo, Root: root}, f)
	if err != nil || s.Phase != "blocked" || s.QuestionRole != "iragent" || s.Attempts != 2 {
		t.Fatalf("review blocker: %+v %v", s, err)
	}
	s, err = testFlow(t, Options{Repository: repo, Root: root, Answer: "Use strict review"}, f)
	if err != nil || s.Phase != "awaiting_approval" || s.Assessments != 1 || s.Attempts != 3 || f.starts != 2 || f.resumes != 1 {
		t.Fatalf("review answer: %+v %v", s, err)
	}
}
func TestIntegrationFailureIsAgentReported(t *testing.T) {
	repo := testRepo(t)
	root := t.TempDir()
	dir, err := storage(repo, root)
	if err != nil {
		t.Fatal(err)
	}
	f := &flowRunner{repo: repo, root: dir, integrationFailure: true}
	s, err := testFlow(t, Options{Repository: repo, Root: root}, f)
	if err != nil {
		t.Fatal(err)
	}
	candidate := s.Candidate
	s, err = testFlow(t, Options{Repository: repo, Root: root, Approve: candidate}, f)
	if err != nil || s.Phase != "integration_failed" || s.Summary != "Parent branch moved; no integration" || s.Attempts != 5 || gitTest(repo, "rev-parse", "main") == candidate {
		t.Fatalf("integration failure: %+v %v", s, err)
	}
}
func TestAssessmentAndInvocationLimitsDoNotReset(t *testing.T) {
	for _, tc := range []struct {
		name                                   string
		maxAttempts, maxAssessments, wantTurns int
	}{{"assessments", 12, 1, 2}, {"invocations", 3, 3, 3}} {
		t.Run(tc.name, func(t *testing.T) {
			repo := testRepo(t)
			root := t.TempDir()
			dir, err := storage(repo, root)
			if err != nil {
				t.Fatal(err)
			}
			f := &flowRunner{repo: repo, root: dir}
			s, err := testFlow(t, Options{Repository: repo, Root: root, MaxAttempts: tc.maxAttempts, MaxAssessments: tc.maxAssessments}, f)
			if err != nil || s.Phase != "limit_exhausted" || f.turns != tc.wantTurns || s.Attempts != tc.wantTurns {
				t.Fatalf("limits: %+v %v turns=%d", s, err, f.turns)
			}
			prior := s.Attempts
			s, err = testFlow(t, Options{Repository: repo, Root: root}, f)
			if err != nil || s.Attempts != prior || f.turns != tc.wantTurns {
				t.Fatalf("limit reset: %+v %v", s, err)
			}
		})
	}
}
