package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopher-launch/concoct/internal/codexapp"
	"github.com/gopher-launch/concoct/internal/handoff"
	"github.com/gopher-launch/concoct/internal/stop"
)

type rejectedReviewRunner struct {
	repo, evidence string
	turns          int
	phaseLess      bool
}

func (r *rejectedReviewRunner) Models(context.Context) ([]codexapp.Model, error) {
	model := codexapp.Model{Model: "test-model", IsDefault: true}
	for _, effort := range []string{"medium", "high"} {
		model.SupportedReasoningEfforts = append(model.SupportedReasoningEfforts, struct {
			ReasoningEffort string `json:"reasoningEffort"`
		}{effort})
	}
	return []codexapp.Model{model}, nil
}
func (r *rejectedReviewRunner) StartThread(_ context.Context, cwd string, s codexapp.RoleSettings, _ []codexapp.ToolSpec) (codexapp.Thread, error) {
	if cwd != r.repo {
		return codexapp.Thread{}, fmt.Errorf("wrong checkout")
	}
	return codexapp.Thread{ID: s.Role + "-thread"}, nil
}
func (r *rejectedReviewRunner) ResumeThread(_ context.Context, id string, _ bool, _ codexapp.RoleSettings) (codexapp.Thread, error) {
	return codexapp.Thread{ID: id}, nil
}
func (r *rejectedReviewRunner) RunRoleTurn(_ context.Context, id, _ string, settings codexapp.RoleSettings, emit func(codexapp.Event)) (codexapp.TurnOutcome, error) {
	r.turns++
	turnID := fmt.Sprintf("turn-%d", r.turns)
	emit(codexapp.Event{Kind: "turn/started", ThreadID: id, TurnID: turnID})
	candidate := "wrong-candidate"
	disposition, summary := "approved", "Approved wrong candidate"
	phase := "final_answer"
	if r.phaseLess {
		candidate, disposition, summary, phase = "", "no_suitable_work", "Nothing ready", ""
	} else if settings.Role == "reagent" {
		gitFlow(r.repo, "switch", "-qc", "task/APP-002")
		if err := os.WriteFile(filepath.Join(r.repo, "feature.txt"), []byte("ready\n"), 0600); err != nil {
			return codexapp.TurnOutcome{}, err
		}
		gitFlow(r.repo, "add", "feature.txt")
		gitFlow(r.repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "candidate")
		candidate = gitFlow(r.repo, "rev-parse", "HEAD")
		if err := os.WriteFile(r.evidence, []byte("# Task evidence\n"), 0600); err != nil {
			return codexapp.TurnOutcome{}, err
		}
		disposition, summary = "ready_for_review", "Candidate ready"
	}
	result, _ := json.Marshal(map[string]any{"disposition": disposition, "summary": summary, "task_id": "APP-002", "candidate": candidate, "findings": nil, "evidence": r.evidence, "question": nil})
	item, _ := json.Marshal(map[string]any{"item": map[string]any{"id": "final", "type": "agentMessage", "phase": phase, "text": string(result)}})
	emit(codexapp.Event{Kind: "item/completed", ThreadID: id, TurnID: turnID, Raw: item})
	emit(codexapp.Event{Kind: "turn/completed", ThreadID: id, TurnID: turnID, Raw: []byte(`{"turn":{"status":"completed"}}`)})
	return codexapp.TurnOutcome{ThreadID: id, TurnID: turnID, Status: "completed", FinalText: string(result)}, nil
}
func (r *rejectedReviewRunner) Stop(context.Context, string, string) stop.Result {
	return stop.Result{Status: "stopped_safely"}
}
func (r *rejectedReviewRunner) Close() error { return nil }

func gitFlow(repo string, args ...string) string {
	out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
	if err != nil {
		panic(fmt.Sprintf("git %v: %s: %v", args, out, err))
	}
	return strings.TrimSpace(string(out))
}

func flowRepo(t *testing.T) (string, string) {
	t.Helper()
	repo, root := t.TempDir(), t.TempDir()
	gitFlow(repo, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, "roadmap.md"), []byte("# Roadmap\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitFlow(repo, "add", "roadmap.md")
	gitFlow(repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "base")
	return repo, root
}

func TestRejectedReviewIsNotAnnouncedAsApproval(t *testing.T) {
	repo, root := flowRepo(t)
	runner := &rejectedReviewRunner{repo: repo, evidence: filepath.Join(root, "task.md")}
	var stdout, stderr bytes.Buffer
	exit := run(context.Background(), []string{"run", "--repo", repo, "--root", root}, &stdout, &stderr, func(context.Context, string, io.Writer) (handoff.Runner, error) { return runner, nil })
	if exit != 2 || runner.turns != 2 || !strings.Contains(stdout.String(), "[reagent result] ready-for-review: Candidate ready") || !strings.Contains(stdout.String(), "Concoct: invalid-result") || strings.Contains(stdout.String(), "[iragent result] approved") || strings.Contains(stdout.String(), "Approved wrong candidate") || !strings.Contains(stderr.String(), "review result identity") {
		t.Fatalf("rejected review presentation: exit=%d turns=%d stdout=%q stderr=%q", exit, runner.turns, stdout.String(), stderr.String())
	}
}

func TestPhaseLessFinalResultIsLabeledAfterSuccessfulRun(t *testing.T) {
	repo, root := flowRepo(t)
	runner := &rejectedReviewRunner{repo: repo, evidence: filepath.Join(root, "task.md"), phaseLess: true}
	var stdout, stderr bytes.Buffer
	exit := run(context.Background(), []string{"run", "--repo", repo, "--root", root}, &stdout, &stderr, func(context.Context, string, io.Writer) (handoff.Runner, error) { return runner, nil })
	if exit != 0 || runner.turns != 1 || !strings.Contains(stdout.String(), "[reagent result] no-suitable-work: Nothing ready") || strings.Contains(stdout.String(), "[reagent] Nothing ready") || stderr.Len() != 0 {
		t.Fatalf("phase-less final presentation: exit=%d turns=%d stdout=%q stderr=%q", exit, runner.turns, stdout.String(), stderr.String())
	}
}

func TestRejectedReviewJSONHasNoAgentApprovalRecord(t *testing.T) {
	repo, root := flowRepo(t)
	runner := &rejectedReviewRunner{repo: repo, evidence: filepath.Join(root, "task.md")}
	var stdout, stderr bytes.Buffer
	exit := run(context.Background(), []string{"run", "--repo", repo, "--root", root, "--json"}, &stdout, &stderr, func(context.Context, string, io.Writer) (handoff.Runner, error) { return runner, nil })
	if exit != 2 || strings.Count(stdout.String(), `"type":"agent-result"`) != 1 || !strings.Contains(stdout.String(), `"disposition":"ready-for-review"`) {
		t.Fatalf("rejected JSON review presentation: exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
}
