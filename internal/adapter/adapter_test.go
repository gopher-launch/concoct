package adapter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gopher-launch/concoct/internal/config"
	"github.com/gopher-launch/concoct/internal/orchestration"
)

func TestCodexInvocationKeepsPromptOffArguments(t *testing.T) {
	dir := t.TempDir()
	codex := filepath.Join(dir, "codex")
	if err := os.WriteFile(codex, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	action := orchestration.Action{ProtocolVersion: orchestration.ProtocolVersion, Kind: "development", Correlation: orchestration.Correlation{InvocationID: "inv", ActionID: "act", TaskID: "CON-010", AttemptID: "try", Role: "developer"}}
	settings := config.Resolved{Adapter: config.Value{Value: "codex"}, Model: config.Value{Value: "gpt-test"}, Reasoning: config.Value{Value: "high"}, Timeout: time.Minute}
	invocation, err := Resolve(dir, action, settings, "/private/schema.json", "/private/result.json")
	if err != nil {
		t.Fatal(err)
	}
	command := DisplayCommand(invocation)
	if !strings.Contains(command, "--sandbox workspace-write") || strings.Contains(command, "bypass") || !strings.HasSuffix(command, " -") {
		t.Fatalf("unsafe command: %s", command)
	}
	if strings.Contains(strings.Join(invocation.Args, " "), "CON-010 prompt") {
		t.Fatal("prompt leaked into arguments")
	}
	schema, err := Schema(action)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"\"const\": \"inv\"", "\"const\": \"act\"", "\"const\": \"CON-010\""} {
		if !strings.Contains(string(schema), value) {
			t.Fatalf("schema lacks %s", value)
		}
	}
}
