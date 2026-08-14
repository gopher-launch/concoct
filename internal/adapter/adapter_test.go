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
	if !strings.Contains(strings.Join(invocation.Args, " "), " --json ") {
		t.Fatal("Codex invocation does not request structured JSONL events")
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

func TestDecodeCodexJSONLPreservesNativeUsageWithoutSummingDuplicates(t *testing.T) {
	evidence := DecodeCodexJSONL([]byte("{\"type\":\"item.completed\",\"usage\":{\"input_tokens\":0,\"cached_input_tokens\":3,\"output_tokens\":7,\"total_tokens\":10}}\n{\"type\":\"item.completed\",\"usage\":{\"input_tokens\":0,\"cached_input_tokens\":3,\"output_tokens\":7,\"total_tokens\":10}}\nnot-json\n"))
	if evidence.Usage.Input == nil || *evidence.Usage.Input != 0 || evidence.Usage.Total == nil || *evidence.Usage.Total != 10 {
		t.Fatalf("usage = %#v", evidence.Usage)
	}
	if len(evidence.Diagnostics) != 2 || !strings.Contains(evidence.Diagnostics[0], "duplicate") || !strings.Contains(evidence.Diagnostics[1], "malformed") {
		t.Fatalf("diagnostics = %#v", evidence.Diagnostics)
	}
}

func TestStreamDecoderPreservesLateUsageAcrossChunkBoundaries(t *testing.T) {
	var decoder StreamDecoder
	for _, chunk := range []string{
		`{"type":"item.started","message":"working"}` + "\n",
		`{"type":"item.completed","usage":{"input_tokens":12,`,
		`"cached_input_tokens":0,"output_tokens":4,"total_tokens":16}}`,
	} {
		if _, err := decoder.Write([]byte(chunk)); err != nil {
			t.Fatal(err)
		}
	}
	evidence := decoder.Close()
	if evidence.Usage.Total == nil || *evidence.Usage.Total != 16 || len(evidence.Progress) != 1 || evidence.Progress[0] != "item.started" {
		t.Fatalf("evidence = %#v", evidence)
	}
}

func TestStreamDecoderUsesBoundedPayloadFreeProgress(t *testing.T) {
	decoder := NewStreamDecoder(96)
	for i := 0; i < 20; i++ {
		if _, err := decoder.Write([]byte(`{"type":"item.started","message":"OPENAI_API_KEY=sk-secret repository content"}` + "\n")); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := decoder.Write([]byte(`{"type":"turn.completed","usage":{"total_tokens":9}}`)); err != nil {
		t.Fatal(err)
	}
	evidence := decoder.Close()
	if evidence.Usage.Total == nil || *evidence.Usage.Total != 9 || !evidence.ProgressTruncated {
		t.Fatalf("evidence = %#v", evidence)
	}
	joined := strings.Join(evidence.Progress, "\n")
	if strings.Contains(joined, "secret") || strings.Contains(joined, "repository") || strings.Contains(joined, "message") {
		t.Fatalf("unsafe progress = %q", joined)
	}
}

func TestStreamDecoderBoundsDiagnosticsAndRecoversAfterOversizedLine(t *testing.T) {
	decoder := NewStreamDecoder(128)
	for i := 0; i < 100; i++ {
		if _, err := decoder.Write([]byte("not-json\n")); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 20; i++ {
		if _, err := decoder.Write([]byte(strings.Repeat("x", 32))); err != nil {
			t.Fatal(err)
		}
		if len(decoder.buffer) > 128 {
			t.Fatalf("partial event buffer = %d bytes", len(decoder.buffer))
		}
	}
	if _, err := decoder.Write([]byte("\n" + `{"type":"turn.completed","usage":{"total_tokens":23}}` + "\n")); err != nil {
		t.Fatal(err)
	}
	evidence := decoder.Close()
	if evidence.Usage.Total == nil || *evidence.Usage.Total != 23 {
		t.Fatalf("late usage = %#v", evidence.Usage)
	}
	if !evidence.DiagnosticsTruncated || !evidence.EventTruncated || evidence.Compatibility() != "degraded" {
		t.Fatalf("truncation evidence = %#v", evidence)
	}
	if len(evidence.Diagnostics) > maxDiagnosticEntries {
		t.Fatalf("diagnostic count = %d", len(evidence.Diagnostics))
	}
	diagnosticBytes := 0
	for _, diagnostic := range evidence.Diagnostics {
		diagnosticBytes += len(diagnostic) + 1
	}
	if diagnosticBytes > 128 {
		t.Fatalf("diagnostic bytes = %d", diagnosticBytes)
	}
}

func TestStreamDecoderBoundsPostTerminalDiagnostics(t *testing.T) {
	decoder := NewStreamDecoder(256)
	if _, err := decoder.Write([]byte(`{"type":"turn.completed","usage":{"total_tokens":31}}` + "\n")); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1000; i++ {
		if _, err := decoder.Write([]byte(`{"type":"item.started"}` + "\n")); err != nil {
			t.Fatal(err)
		}
	}
	evidence := decoder.Close()
	if evidence.Usage.Total == nil || *evidence.Usage.Total != 31 || !evidence.DiagnosticsTruncated || evidence.Compatibility() != "degraded" {
		t.Fatalf("evidence = %#v", evidence)
	}
	if len(evidence.Diagnostics) > maxDiagnosticEntries {
		t.Fatalf("diagnostic count = %d", len(evidence.Diagnostics))
	}
}

func TestDecodeCodexJSONLDiagnosesDegradedAndPartialStreams(t *testing.T) {
	tests := []struct {
		name, stream, diagnostic string
		usage                    bool
		status                   string
	}{
		{"missing usage", `{"type":"turn.completed"}` + "\n", "", false, "supported"},
		{"malformed usage", `{"type":"turn.completed","usage":{"total_tokens":-1}}` + "\n", "malformed usage", false, "supported"},
		{"contradictory usage", `{"type":"item.completed","usage":{"total_tokens":1}}` + "\n" + `{"type":"turn.completed","usage":{"total_tokens":2}}` + "\n", "contradictory duplicate", true, "supported"},
		{"out of order", `{"type":"turn.completed","usage":{"total_tokens":3}}` + "\n" + `{"type":"item.started","message":"late"}` + "\n", "follows a terminal", true, "degraded"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			evidence := DecodeCodexJSONL([]byte(test.stream))
			if evidence.HasUsage() != test.usage || evidence.Compatibility() != test.status {
				t.Fatalf("evidence = %#v", evidence)
			}
			if test.diagnostic != "" && !strings.Contains(strings.Join(evidence.Diagnostics, "; "), test.diagnostic) {
				t.Fatalf("diagnostics = %#v", evidence.Diagnostics)
			}
		})
	}
}
