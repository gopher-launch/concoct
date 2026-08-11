package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveProfilePrecedenceAndProvenance(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".concoct"), 0o755); err != nil {
		t.Fatal(err)
	}
	user := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", user)
	writeConfig(t, filepath.Join(user, "concoct", "config.yaml"), "exec:\n  adapter: codex\n  roles:\n    developer:\n      model: user-model\n      reasoning: low\n      timeout: 8m\n")
	writeConfig(t, filepath.Join(root, ".concoct", "config.yaml"), "git:\n  auto-push: true\nexec:\n  roles:\n    developer:\n      reasoning: high\n      timeout: 5m\n  retention:\n    max-completed: 7\n")
	defaults := Defaults{Adapter: "codex", Model: "default-model", Reasoning: "medium", Timeout: 30 * time.Minute, Roles: map[string]ProfileDefaults{"developer": {Model: "role-model", Reasoning: "medium"}}}
	resolved, err := Resolve(root, "developer", Overrides{Model: "flag-model"}, defaults)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Model.Value != "flag-model" || resolved.Model.Source != "invocation override" {
		t.Fatalf("model = %#v", resolved.Model)
	}
	if resolved.Reasoning.Value != "high" || resolved.Reasoning.Source != "project role configuration" {
		t.Fatalf("reasoning = %#v", resolved.Reasoning)
	}
	if resolved.Timeout != 5*time.Minute || resolved.Retention.MaxCompleted != 7 {
		t.Fatalf("resolved = %#v", resolved)
	}
	auto, err := AutoPush(root)
	if err != nil || !auto {
		t.Fatalf("auto=%t err=%v", auto, err)
	}
}

func TestResolveRejectsUnknownAndInvalidConfiguration(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".concoct"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	defaults := Defaults{Adapter: "codex", Reasoning: "medium", Timeout: 30 * time.Minute}
	for _, body := range []string{"exec:\n  mystery: true\n", "exec:\n  roles:\n    developer:\n      timeout: forever\n", "exec:\n  roles:\n    stranger:\n      model: x\n", "exec:\n  adapter: codex\n---\nexec:\n  adapter: codex\n"} {
		writeConfig(t, filepath.Join(root, ".concoct", "config.yaml"), body)
		if _, err := Resolve(root, "developer", Overrides{}, defaults); err == nil {
			t.Fatalf("configuration accepted: %s", body)
		}
		if err := os.Remove(filepath.Join(root, ".concoct", "config.yaml")); err != nil {
			t.Fatal(err)
		}
	}
}

func writeConfig(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimSpace(body)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
