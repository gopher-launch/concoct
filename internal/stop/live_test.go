package stop

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/gopher-launch/concoct/internal/codexapp"
)

// This opt-in fixture spends one provider turn. It uses the installed runner
// and the production client/stop sequence; ordinary package tests stay local.
func TestInstalledRunnerChildStop(t *testing.T) {
	if os.Getenv("CONCOCT_LIVE_STOP") != "1" {
		t.Skip("set CONCOCT_LIVE_STOP=1 for the bounded installed-runner fixture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	process, err := codexapp.Launch(ctx, "codex", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer process.Close()
	probe, err := NewProbe(process.PID())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		rows, _ := processTable()
		probe.mu.Lock()
		defer probe.mu.Unlock()
		for identity := range probe.owned {
			if row, ok := rows[identity.PID]; ok && row.Start == identity.Start {
				_ = syscall.Kill(identity.PID, syscall.SIGKILL)
			}
		}
	})
	models, err := process.Models(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var chosen codexapp.Model
	for _, model := range models {
		if model.IsDefault {
			chosen = model
			break
		}
	}
	if chosen.Model == "" {
		t.Fatal("provider has no recommended default model")
	}
	effort := chosen.DefaultReasoningEffort
	if effort == "" {
		effort = "medium"
	}
	settings := codexapp.RoleSettings{Role: "reagent", Model: chosen.Model, Effort: effort, Sandbox: "danger-full-access", ApprovalPolicy: "never"}
	dir := t.TempDir()
	heartbeat := filepath.Join(dir, "heartbeat")
	script := filepath.Join(dir, "child.sh")
	content := "#!/usr/bin/env bash\nwhile :; do date +%s%N > " + heartbeat + "; sleep 0.05; done\n"
	if err := os.WriteFile(script, []byte(content), 0700); err != nil {
		t.Fatal(err)
	}
	thread, err := process.StartThread(ctx, dir, settings, nil)
	if err != nil {
		t.Fatal(err)
	}
	turnCtx, stopTurn := context.WithCancel(ctx)
	defer stopTurn()
	turnID := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		_, err := process.RunTurn(turnCtx, thread.ID, fmt.Sprintf("Run this exact local command in a shell and wait for it: bash %q. It writes a heartbeat for a stop/cleanup test; do not run any other command and do not stop it yourself.", script), settings, nil, func(event codexapp.Event) {
			if event.Kind == "turn/started" && event.TurnID != "" {
				select {
				case turnID <- event.TurnID:
				default:
				}
			}
			_, _ = probe.Capture()
		})
		done <- err
	}()
	var id string
	select {
	case id = <-turnID:
	case <-time.After(45 * time.Second):
		t.Fatal("runner did not acknowledge a turn")
	}
	for deadline := time.Now().Add(90 * time.Second); time.Now().Before(deadline); {
		_, _ = probe.Capture()
		if _, err := os.Stat(heartbeat); err == nil {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("provider turn ended before child fixture: %v", err)
		default:
		}
		time.Sleep(50 * time.Millisecond)
	}
	before, err := os.ReadFile(heartbeat)
	if err != nil {
		t.Fatalf("child did not write heartbeat: %v", err)
	}
	// The provider exposes a background handle only after its shell call yields.
	// Wait for that exact handle before testing the terminate/clean sequence.
	handlesReady := false
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); {
		_, _ = probe.Capture()
		handles, listErr := process.BackgroundTerminals(ctx, thread.ID)
		if listErr == nil && len(handles) > 0 {
			handlesReady = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !handlesReady {
		t.Fatal("provider never registered the child background terminal")
	}
	stopTurn()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("client turn did not return after cancellation")
	}
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 18*time.Second)
	defer stopCancel()
	result := Run(stopCtx, process.Client, probe, thread.ID, id, 2*time.Second)
	if result.Status != "stopped_safely" || len(result.Owned) == 0 || len(result.Handles) == 0 {
		rows, _ := processTable()
		for _, owned := range result.Owned {
			if row, ok := rows[owned.PID]; ok && row.Start == owned.Start && row.State != 'Z' {
				t.Logf("remaining owned process: %+v", row)
			}
		}
		t.Fatalf("installed-runner cleanup: %+v", result)
	}
	after, err := os.ReadFile(heartbeat)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(after)) == "" || strings.TrimSpace(string(before)) == "" {
		t.Fatal("missing heartbeat evidence")
	}
	resumed, err := process.ResumeThread(ctx, thread.ID, true, settings)
	if err != nil || resumed.ID != thread.ID {
		t.Fatalf("explicit settings on resume: %+v, %v", resumed, err)
	}
	time.Sleep(250 * time.Millisecond)
	final, err := os.ReadFile(heartbeat)
	if err != nil || string(final) != string(after) {
		t.Fatalf("stopped command replayed or heartbeat continued: %q versus %q, %v", final, after, err)
	}
}
