package adapter

import (
	"encoding/json"
	"fmt"
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

func TestProductOwnerSchemaPermitsOnlyBoundedRecordMutations(t *testing.T) {
	action := orchestration.Action{ProtocolVersion: orchestration.ProtocolVersion, Kind: "product-owner-next", Correlation: orchestration.Correlation{InvocationID: "inv", ActionID: "act", AttemptID: "try", Role: "product-owner"}}
	schema, err := Schema(action)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"\"mutations\"", "\"before_digest\"", "\"capabilities\""} {
		if !strings.Contains(string(schema), field) {
			t.Fatalf("Product Owner schema lacks bounded mutation field %s:\n%s", field, schema)
		}
	}
}

func TestEveryGeneratedActionSchemaSatisfiesRecursiveContract(t *testing.T) {
	for _, spec := range orchestration.Registry() {
		t.Run(spec.Kind, func(t *testing.T) {
			action := orchestration.Action{Kind: spec.Kind, Correlation: orchestration.Correlation{Role: spec.Role}}
			data, err := Schema(action)
			if err != nil {
				t.Fatal(err)
			}
			var schema map[string]any
			if err := json.Unmarshal(data, &schema); err != nil {
				t.Fatal(err)
			}
			if err := ValidateSchemaContract(schema); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSchemaContractReportsCompleteMissingAndDuplicatePaths(t *testing.T) {
	base := map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{
		"outer": map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{"value": map[string]any{"type": "string"}}, "required": []string{}},
	}, "required": []string{"outer"}}
	if err := ValidateSchemaContract(base); err == nil || !strings.Contains(err.Error(), `$.properties.outer.required is missing property "value"`) {
		t.Fatalf("missing error = %v", err)
	}
	base["properties"].(map[string]any)["outer"].(map[string]any)["required"] = []string{"value", "value"}
	if err := ValidateSchemaContract(base); err == nil || !strings.Contains(err.Error(), `$.properties.outer.required contains duplicate property "value"`) {
		t.Fatalf("duplicate error = %v", err)
	}
}

func TestSchemaContractRecursesThroughArbitraryMapsAndArrays(t *testing.T) {
	schema := map[string]any{"oneOf": []any{map[string]any{"$defs": map[string]any{
		"nested": map[string]any{"type": "object", "additionalProperties": true, "properties": map[string]any{"value": map[string]any{"type": "string"}}, "required": []string{"value"}},
	}}}}
	err := ValidateSchemaContract(schema)
	if err == nil || !strings.Contains(err.Error(), `$.oneOf[0].$defs.nested.additionalProperties must be false`) {
		t.Fatalf("nested schema error = %v", err)
	}
}

func TestPostCON040ProductOwnerSchemaRequiresMutations(t *testing.T) {
	action := orchestration.Action{Kind: "product-owner-next", Correlation: orchestration.Correlation{Role: "product-owner"}}
	data, err := Schema(action)
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties map[string]struct {
			Required []string `json:"required"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, name := range schema.Properties["product_decision"].Required {
		found = found || name == "mutations"
	}
	if !found {
		t.Fatal("product_decision.mutations is not required")
	}
}

func TestTurnFailedRetainsOnlyBoundedDiagnostic(t *testing.T) {
	stream := `{"type":"turn.started"}` + "\n" + `{"type":"turn.failed","error":{"type":"invalid_request_error","message":"schema rejected","raw":"secret"},"arbitrary":"do not retain"}` + "\n"
	evidence := DecodeCodexJSONL([]byte(stream))
	if evidence.TerminalFailure == nil || evidence.TerminalFailure.Type != "invalid_request_error" || evidence.TerminalFailure.Message != "schema rejected" || evidence.HasUsage() {
		t.Fatalf("evidence = %#v", evidence)
	}
	encoded, _ := json.Marshal(evidence)
	if strings.Contains(string(encoded), "secret") || strings.Contains(string(encoded), "arbitrary") {
		t.Fatalf("raw payload retained: %s", encoded)
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

func TestDecodeCodexJSONLNormalizesPayloadSafeActivity(t *testing.T) {
	secret := "OPENAI_API_KEY=sk-secret"
	stream := strings.Join([]string{
		`{"type":"item.completed","item":{"type":"command_execution","command":"sed -n '1,20p' secret.txt","aggregated_output":"` + secret + `"}}`,
		`{"type":"item.completed","item":{"type":"command_execution","command":"sed -n '1,20p' secret.txt","aggregated_output":"more"}}`,
		`{"type":"item.completed","item":{"type":"command_execution","command":"rg token . | head","aggregated_output":"combined"}}`,
		`{"type":"item.completed","item":{"type":"file_change"}}`,
		`{"type":"turn.completed","usage":{"input_tokens":20,"cached_input_tokens":12,"output_tokens":4,"total_tokens":24}}`,
	}, "\n") + "\n"
	evidence := DecodeCodexJSONL([]byte(stream))
	encoded := fmt.Sprintf("%#v", evidence)
	if strings.Contains(encoded, secret) || strings.Contains(encoded, "secret.txt") {
		t.Fatalf("payload leaked into normalized evidence: %s", encoded)
	}
	if evidence.Commands.Count != 3 || evidence.Commands.OutputBytes != int64(len(secret)+len("more")+len("combined")) {
		t.Fatalf("commands = %#v", evidence.Commands)
	}
	if len(evidence.Repeated) != 1 || evidence.Repeated[0].Category != "file-read" || evidence.Repeated[0].Count != 2 || len(evidence.Repeated[0].Fingerprint) != 16 {
		t.Fatalf("repeated = %#v", evidence.Repeated)
	}
	if evidence.Availability["exchange-usage"].Availability != "unavailable" || evidence.Availability["usage"].Availability != "available" {
		t.Fatalf("availability = %#v", evidence.Availability)
	}
}

func TestClassifyCommandRequiresAffirmativeCheckEvidence(t *testing.T) {
	tests := []struct {
		command, want string
	}{
		{"go test ./...", "test-check"},
		{"go vet ./...", "test-check"},
		{"npm run lint", "test-check"},
		{"cargo check", "test-check"},
		{"make verify", "test-check"},
		{"go env", "command-other"},
		{"bash migration.sh", "command-other"},
		{"sh read-data.sh", "command-other"},
		{"npm publish", "command-other"},
		{"make release", "command-other"},
		{"go test ./... | tee result", "command-other"},
	}
	for _, test := range tests {
		if got := classifyCommand(test.command); got != test.want {
			t.Errorf("classifyCommand(%q) = %q, want %q", test.command, got, test.want)
		}
	}
}

func TestDecodeCodexJSONLRetainsBoundedUsageSnapshots(t *testing.T) {
	var stream strings.Builder
	for i := 1; i <= maxUsageSnapshots+5; i++ {
		fmt.Fprintf(&stream, `{"type":"item.completed","usage":{"total_tokens":%d}}`+"\n", i)
	}
	evidence := DecodeCodexJSONL([]byte(stream.String()))
	if len(evidence.UsageSnapshots) != maxUsageSnapshots || !evidence.UsageSnapshotsTruncated || evidence.Usage.Total == nil || *evidence.Usage.Total != maxUsageSnapshots+5 {
		t.Fatalf("usage snapshots = %#v latest=%#v", evidence.UsageSnapshots, evidence.Usage)
	}
}
