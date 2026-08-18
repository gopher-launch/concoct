// Package adapter defines the executable-owned, workflow-neutral agent adapter registry.
package adapter

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gopher-launch/concoct/internal/config"
	"github.com/gopher-launch/concoct/internal/orchestration"
)

type Spec struct {
	Name     string
	Defaults config.Defaults
}

type Invocation struct {
	Adapter, AdapterVersion, Executable, Root, SchemaPath, CandidatePath, Safety string
	Args, Environment                                                            []string
}

var registry = []Spec{{
	Name: "codex",
	Defaults: config.Defaults{
		Adapter: "codex", Reasoning: "medium", Timeout: 30 * time.Minute,
		Roles: map[string]config.ProfileDefaults{
			"product-owner": {Reasoning: "medium"}, "task-planner": {Reasoning: "high"},
			"developer": {Reasoning: "high"}, "reviewer": {Reasoning: "high"},
			"archivist": {Reasoning: "medium"}, "integrator": {Reasoning: "medium"},
		},
	},
}}

func Registry() []Spec { return append([]Spec(nil), registry...) }
func Find(name string) (Spec, bool) {
	for _, spec := range registry {
		if spec.Name == name {
			return spec, true
		}
	}
	return Spec{}, false
}

func Resolve(root string, action orchestration.Action, settings config.Resolved, schemaPath, candidatePath string) (Invocation, error) {
	spec, ok := Find(settings.Adapter.Value)
	if !ok {
		return Invocation{}, fmt.Errorf("unsupported adapter %q", settings.Adapter.Value)
	}
	if spec.Name != "codex" {
		return Invocation{}, fmt.Errorf("adapter %q is not implemented", spec.Name)
	}
	executable, err := exec.LookPath("codex")
	if err != nil {
		return Invocation{}, fmt.Errorf("codex adapter is unavailable on PATH: %w", err)
	}
	if model := settings.Model.Value; model != "" && !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]*$`).MatchString(model) {
		return Invocation{}, fmt.Errorf("invalid codex model value %q", model)
	}
	if !map[string]bool{"minimal": true, "low": true, "medium": true, "high": true, "xhigh": true}[settings.Reasoning.Value] {
		return Invocation{}, fmt.Errorf("unsupported codex reasoning value %q", settings.Reasoning.Value)
	}
	args := []string{"exec", "--strict-config", "--ignore-user-config", "--sandbox", "workspace-write", "--cd", root, "--ephemeral", "--color", "never", "--json", "--output-schema", schemaPath, "--output-last-message", candidatePath}
	if settings.Model.Value != "" {
		args = append(args, "--model", settings.Model.Value)
	}
	args = append(args, "--config", "model_reasoning_effort="+strconv.Quote(settings.Reasoning.Value), "-")
	return Invocation{
		Adapter: spec.Name, Executable: executable, Root: root, SchemaPath: schemaPath,
		CandidatePath: candidatePath, Args: args, Environment: allowlistedEnvironment(),
		Safety: "workspace-write sandbox; project rules and Codex approval policy retained; user config ignored; no bypass flags; allowlisted inherited environment",
	}, nil
}

// Version records the adapter's self-reported version when available.  A
// probe failure intentionally does not prevent a normal invocation: version
// evidence is observational, never an execution or workflow authority.
func Version(executable string) (string, error) {
	output, err := exec.Command(executable, "--version").Output()
	if err != nil {
		return "", err
	}
	version := strings.TrimSpace(string(output))
	if len(version) > 256 {
		return "", fmt.Errorf("adapter version output exceeds 256 bytes")
	}
	return version, nil
}

func Schema(action orchestration.Action) ([]byte, error) {
	spec, ok := orchestration.Find(action.Kind)
	if !ok {
		return nil, fmt.Errorf("unknown action kind %q", action.Kind)
	}
	classes := make([]string, 0, len(spec.SupportedOutcomes))
	for _, class := range spec.SupportedOutcomes {
		classes = append(classes, string(class))
	}
	correlation := map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"invocation_id": constString(action.Correlation.InvocationID), "action_id": constString(action.Correlation.ActionID),
			"task_id": constString(action.Correlation.TaskID), "attempt_id": constString(action.Correlation.AttemptID), "role": constString(action.Correlation.Role),
		},
		"required": []string{"invocation_id", "action_id", "task_id", "attempt_id", "role"},
	}
	stringArray := map[string]any{"type": "array", "maxItems": 32, "items": map[string]any{"type": "string", "maxLength": 512}}
	diagnostics := map[string]any{"type": "array", "maxItems": 16, "items": map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{"code": map[string]any{"type": "string", "maxLength": 64}, "message": map[string]any{"type": "string", "maxLength": 512}},
		"required":   []string{"code", "message"},
	}}
	recommendationKinds := []string{""}
	productDecision := map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{}, "required": []string{}}
	if action.Kind == "product-owner-next" {
		recommendationKinds = []string{""}
		mutation := map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{
			"target":        map[string]any{"type": "string", "enum": []string{"roadmap", "capabilities"}},
			"id":            map[string]any{"type": "string", "minLength": 1, "maxLength": 128},
			"before_digest": map[string]any{"type": "string", "minLength": 64, "maxLength": 64},
			"before":        map[string]any{"type": "string", "minLength": 1, "maxLength": 8192},
			"after":         map[string]any{"type": "string", "minLength": 1, "maxLength": 8192},
		}, "required": []string{"target", "id", "before_digest", "before", "after"}}
		productDecision = map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{
			"version":             map[string]any{"type": "integer", "const": 1},
			"kind":                map[string]any{"type": "string", "enum": []string{orchestration.DecisionSelect, orchestration.DecisionReconcileAndSelect, orchestration.DecisionReconcile, orchestration.DecisionHumanRequired, orchestration.DecisionNoAction}},
			"selection":           map[string]any{"type": "string", "maxLength": 128},
			"rationale":           map[string]any{"type": "string", "minLength": 1, "maxLength": 512},
			"roadmap_digest":      map[string]any{"type": "string", "maxLength": 512},
			"capability_digest":   map[string]any{"type": "string", "maxLength": 512},
			"completion_evidence": map[string]any{"type": "string", "maxLength": 512},
			"mutations":           map[string]any{"type": "array", "maxItems": 8, "items": mutation},
		}, "required": []string{"version", "kind", "selection", "rationale", "roadmap_digest", "capability_digest", "completion_evidence"}}
	}
	schema := map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"protocol_version": constString(orchestration.ProtocolVersion), "correlation": correlation,
			"class": map[string]any{"type": "string", "enum": classes}, "summary": map[string]any{"type": "string", "minLength": 1, "maxLength": 1024},
			"artifacts": stringArray,
			"intervention": map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{
				"kind": enumString([]string{"", spec.Intervention.Kind}, "Use empty for completed; otherwise use the registered intervention kind."),
				"next": enumString([]string{"", spec.Intervention.Next}, "Use empty for completed; otherwise use the registered intervention next step."),
			}, "required": []string{"kind", "next"}},
			"diagnostics":      diagnostics,
			"recommendation":   map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{"kind": map[string]any{"type": "string", "enum": recommendationKinds}, "command": map[string]any{"type": "string", "maxLength": 512}, "reason": map[string]any{"type": "string", "maxLength": 512}}, "required": []string{"kind", "command", "reason"}},
			"product_decision": productDecision,
		},
		"required": []string{"protocol_version", "correlation", "class", "summary", "artifacts", "intervention", "diagnostics", "recommendation", "product_decision"},
	}
	return json.MarshalIndent(schema, "", "  ")
}

func DisplayCommand(inv Invocation) string {
	values := append([]string{inv.Executable}, inv.Args...)
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = shellQuote(value)
	}
	return strings.Join(quoted, " ")
}

func constString(value string) map[string]any {
	return map[string]any{"type": "string", "const": value}
}
func enumString(values []string, description string) map[string]any {
	return map[string]any{"type": "string", "enum": values, "description": description}
}
func shellQuote(value string) string {
	if value != "" && regexp.MustCompile(`^[A-Za-z0-9_./:=+-]+$`).MatchString(value) {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func allowlistedEnvironment() []string {
	allowed := map[string]bool{
		"PATH": true, "HOME": true, "USER": true, "LOGNAME": true, "TMPDIR": true,
		"LANG": true, "LC_ALL": true, "TERM": true, "COLORTERM": true,
		"XDG_CONFIG_HOME": true, "XDG_CACHE_HOME": true, "CODEX_HOME": true,
		"OPENAI_API_KEY": true, "OPENAI_ORG_ID": true, "OPENAI_PROJECT_ID": true,
		"SSL_CERT_FILE": true, "SSL_CERT_DIR": true, "GIT_AUTHOR_NAME": true,
		"GIT_AUTHOR_EMAIL": true, "GIT_COMMITTER_NAME": true, "GIT_COMMITTER_EMAIL": true,
	}
	var out []string
	for _, value := range os.Environ() {
		name, _, ok := strings.Cut(value, "=")
		if ok && (allowed[name] || strings.HasPrefix(name, "LC_")) {
			out = append(out, value)
		}
	}
	return out
}
