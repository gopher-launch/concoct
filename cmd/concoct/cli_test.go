package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gopher-launch/concoct/internal/codexapp"
	"github.com/gopher-launch/concoct/internal/handoff"
)

func TestApprovalAndAnswerOptions(t *testing.T) {
	for _, tc := range []struct {
		args                     []string
		wantApproval, wantAnswer string
		wantReviewLimit          int
	}{{[]string{"run", "--approve", "abc"}, "abc", "", 0}, {[]string{"run", "--answer", "Use safe mode"}, "", "Use safe mode", 0}, {[]string{"run", "--review-limit", "2"}, "", "", 2}} {
		o, command, _, _, err := parse(tc.args)
		if err != nil || command != "run" || o.Approve != tc.wantApproval || o.Answer != tc.wantAnswer || o.MaxAssessments != tc.wantReviewLimit {
			t.Fatalf("parse %v: %+v %v", tc.args, o, err)
		}
	}
	for _, args := range [][]string{{"run", "--review-limit", "2x"}, {"status", "--approve", "abc"}, {"status", "--answer", "yes"}} {
		if _, _, _, _, err := parse(args); err == nil {
			t.Fatalf("accepted invalid args %v", args)
		}
	}
}
func TestApprovalAndBlockerPresentation(t *testing.T) {
	var out bytes.Buffer
	s := handoff.State{Phase: "awaiting_approval", TaskID: "APP-002", Candidate: "abc123", ApprovedCommit: "abc123", ReagentSummary: "implemented", ReviewSummary: "approved", ReviewEvidence: "/tmp/review-01.md", Attempts: 2, MaxAttempts: 12, Assessments: 1, MaxAssessments: 3}
	display(&out, s, false)
	for _, want := range []string{"Task: APP-002", "Candidate: abc123", "Reagent summary: implemented", "Iragent summary: approved", "Review evidence: /tmp/review-01.md", "concoct run --approve abc123"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in %s", want, out.String())
		}
	}
	out.Reset()
	s.Phase = "blocked"
	s.Question = &codexapp.RoleQuestion{Text: "Choose mode?"}
	display(&out, s, false)
	if !strings.Contains(out.String(), "Question: Choose mode?") || !strings.Contains(out.String(), "concoct run --answer TEXT") {
		t.Fatalf("blocker output: %s", out.String())
	}
	out.Reset()
	s.Phase = "limit_exhausted"
	s.Question = nil
	s.LastError = "approved candidate retained, but invocation limit leaves no integration turn"
	display(&out, s, false)
	if strings.Contains(out.String(), "concoct run --approve") || !strings.Contains(out.String(), "no integration turn") {
		t.Fatalf("exhausted approval output: %s", out.String())
	}
}
