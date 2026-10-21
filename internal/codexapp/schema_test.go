package codexapp

import (
	"encoding/json"
	"fmt"
	"testing"
)

// This is the response schema shipped before CON-063. Its valid JSON shape
// omitted declared properties from required at the root and in question.
var previousRoleResultSchema = json.RawMessage(`{"type":"object","properties":{"disposition":{"type":"string"},"summary":{"type":"string"},"task_id":{"type":"string"},"candidate":{"type":"string"},"findings":{"type":"array","items":{"type":"string"}},"question":{"type":"object","properties":{"objective":{"type":"string"},"evidence":{"type":"string"},"missing":{"type":"string"},"safe_work":{"type":"string"},"next_step":{"type":"string"},"source_change":{"type":"boolean"},"scope_change":{"type":"boolean"},"options":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"},"description":{"type":"string"},"tradeoff":{"type":"string"}},"required":["id","description","tradeoff"],"additionalProperties":false}}},"required":["objective","evidence","missing","safe_work","next_step"],"additionalProperties":false}},"required":["disposition","summary"],"additionalProperties":false}`)

func schemaObject(raw json.RawMessage) (map[string]any, error) {
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, err
	}
	return schema, nil
}

func schemaTypes(node map[string]any) []string {
	switch value := node["type"].(type) {
	case string:
		return []string{value}
	case []any:
		var result []string
		for _, item := range value {
			name, ok := item.(string)
			if !ok {
				return nil
			}
			result = append(result, name)
		}
		return result
	default:
		return nil
	}
}

func hasType(node map[string]any, name string) bool {
	for _, item := range schemaTypes(node) {
		if item == name {
			return true
		}
	}
	return false
}

// checkProviderSchema covers the provider rules exercised by this outbound
// schema, recursively through nullable objects and array items.
func checkProviderSchema(node map[string]any, path string) error {
	types := schemaTypes(node)
	if len(types) == 0 {
		return fmt.Errorf("%s has no type", path)
	}
	for _, kind := range types {
		if kind != "object" && kind != "array" && kind != "string" && kind != "boolean" && kind != "null" {
			return fmt.Errorf("%s has unsupported type %s", path, kind)
		}
	}
	if hasType(node, "object") {
		properties, ok := node["properties"].(map[string]any)
		if !ok || node["additionalProperties"] != false {
			return fmt.Errorf("%s lacks closed object properties", path)
		}
		required, ok := node["required"].([]any)
		if !ok || len(required) != len(properties) {
			return fmt.Errorf("%s does not require every property", path)
		}
		seen := map[string]bool{}
		for _, item := range required {
			name, ok := item.(string)
			if !ok || seen[name] || properties[name] == nil {
				return fmt.Errorf("%s has an invalid required key", path)
			}
			seen[name] = true
		}
		for name, child := range properties {
			if !seen[name] {
				return fmt.Errorf("%s.%s is not required", path, name)
			}
			value, ok := child.(map[string]any)
			if !ok {
				return fmt.Errorf("%s.%s has no schema", path, name)
			}
			if err := checkProviderSchema(value, path+"."+name); err != nil {
				return err
			}
		}
	}
	if hasType(node, "array") {
		items, ok := node["items"].(map[string]any)
		if !ok {
			return fmt.Errorf("%s has no item schema", path)
		}
		return checkProviderSchema(items, path+"[]")
	}
	return nil
}

func checkSchemaValue(node map[string]any, value any, path string) error {
	if value == nil {
		if hasType(node, "null") {
			return nil
		}
		return fmt.Errorf("%s does not allow null", path)
	}
	switch value := value.(type) {
	case string:
		if hasType(node, "string") {
			return nil
		}
	case bool:
		if hasType(node, "boolean") {
			return nil
		}
	case []any:
		if hasType(node, "array") {
			items := node["items"].(map[string]any)
			for i, item := range value {
				if err := checkSchemaValue(items, item, fmt.Sprintf("%s[%d]", path, i)); err != nil {
					return err
				}
			}
			return nil
		}
	case map[string]any:
		if hasType(node, "object") {
			properties := node["properties"].(map[string]any)
			if len(value) != len(properties) {
				return fmt.Errorf("%s has missing or extra properties", path)
			}
			for name, item := range value {
				child, ok := properties[name].(map[string]any)
				if !ok {
					return fmt.Errorf("%s.%s is not declared", path, name)
				}
				if err := checkSchemaValue(child, item, path+"."+name); err != nil {
					return err
				}
			}
			return nil
		}
	}
	return fmt.Errorf("%s has an unsupported value", path)
}

func TestOutboundRoleResultSchemaCompatibility(t *testing.T) {
	old, err := schemaObject(previousRoleResultSchema)
	if err != nil {
		t.Fatal(err)
	}
	if err := checkProviderSchema(old, "$old"); err == nil {
		t.Fatal("previously shipped schema unexpectedly satisfied provider rules")
	}
	schema, err := schemaObject(RoleResultSchema)
	if err != nil {
		t.Fatal(err)
	}
	if err := checkProviderSchema(schema, "$outbound"); err != nil {
		t.Fatal(err)
	}
	root := schema["properties"].(map[string]any)
	question := root["question"].(map[string]any)["properties"].(map[string]any)
	for _, name := range []string{"task_id", "candidate", "findings", "question"} {
		if !hasType(root[name].(map[string]any), "null") {
			t.Fatalf("optional result field %s is not nullable", name)
		}
	}
	for _, name := range []string{"source_change", "scope_change", "options"} {
		if !hasType(question[name].(map[string]any), "null") {
			t.Fatalf("optional question field %s is not nullable", name)
		}
	}
}

func TestSchemaProducedRoleResults(t *testing.T) {
	schema, err := schemaObject(RoleResultSchema)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, value, disposition string
		question                 bool
	}{
		{"no work", `{"disposition":"no_suitable_work","summary":"nothing ready","task_id":null,"candidate":null,"findings":null,"question":null}`, "no_suitable_work", false},
		{"engineering", `{"disposition":"engineering","summary":"work in progress","task_id":"APP-002","candidate":null,"findings":null,"question":null}`, "engineering", false},
		{"ready", `{"disposition":"ready_for_review","summary":"ready","task_id":"APP-002","candidate":null,"findings":null,"question":null}`, "ready_for_review", false},
		{"approved", `{"disposition":"approved","summary":"checked","task_id":"APP-002","candidate":"abc123","findings":null,"question":null}`, "approved", false},
		{"changes", `{"disposition":"changes_requested","summary":"fix it","task_id":"APP-002","candidate":"abc123","findings":["Correct feature.go"],"question":null}`, "changes_requested", false},
		{"question", `{"disposition":"decision_required","summary":"input needed","task_id":"APP-002","candidate":null,"findings":null,"question":{"objective":"Choose behavior","evidence":"Two options work","missing":"Required choice","safe_work":"Source is frozen","next_step":"Answer the question","source_change":true,"scope_change":null,"options":[{"id":"A","description":"Use A","tradeoff":"More work"}]}}`, "decision_required", true},
		{"question without options", `{"disposition":"decision_required","summary":"input needed","task_id":null,"candidate":null,"findings":null,"question":{"objective":"Complete task","evidence":"Condition unknown","missing":"Provide evidence","safe_work":"Wait","next_step":"Answer","source_change":null,"scope_change":null,"options":null}}`, "decision_required", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var value any
			if err := json.Unmarshal([]byte(tc.value), &value); err != nil {
				t.Fatal(err)
			}
			if err := checkSchemaValue(schema, value, "$result"); err != nil {
				t.Fatal(err)
			}
			parsed, err := ParseRoleResult(tc.value)
			if err != nil || parsed.Disposition != tc.disposition || (parsed.Question != nil) != tc.question {
				t.Fatalf("parsed result = %+v, %v", parsed, err)
			}
			if tc.name == "question" && (!parsed.Question.SourceChange || parsed.Question.ScopeChange || len(parsed.Question.Options) != 1 || parsed.Question.Options[0].ID != "A") {
				t.Fatalf("question fields were not parsed: %+v", parsed.Question)
			}
			if tc.name == "question without options" && (parsed.Question.SourceChange || parsed.Question.ScopeChange || len(parsed.Question.Options) != 0) {
				t.Fatalf("nullable question fields were not parsed: %+v", parsed.Question)
			}
		})
	}
}
