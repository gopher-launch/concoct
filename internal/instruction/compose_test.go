package instruction

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComposeOrderAttributionAndBytePreservation(t *testing.T) {
	root := fixture(t, "strengthen-controls:\n  - evidence-integrity\n")
	before, _ := os.ReadFile(filepath.Join(root, GuidancePath))
	got, err := Compose(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Sources) != 3 || got.Sources[0].Layer != "protocol" || got.Sources[1].Layer != "policy" || got.Sources[2].Path != GuidancePath {
		t.Fatalf("sources = %#v", got.Sources)
	}
	if !bytes.Equal(before, got.Sources[2].Content) {
		t.Fatal("project guidance bytes changed")
	}
	after, _ := os.ReadFile(filepath.Join(root, GuidancePath))
	if !bytes.Equal(before, after) {
		t.Fatal("composition modified project guidance")
	}
}

func TestComposeRejectsWeakeningMalformedAndMissingWithoutPartialResult(t *testing.T) {
	tests := []struct {
		name, guidance string
		mutate         func(string)
	}{
		{"weakening", "weaken-controls:\n  - evidence-integrity\n", nil},
		{"malformed", "bad declaration\n", nil},
		{"missing", "", func(root string) { os.Remove(filepath.Join(root, PolicyPath)) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := fixture(t, tt.guidance)
			if tt.mutate != nil {
				tt.mutate(root)
			}
			got, err := Compose(root)
			if err == nil {
				t.Fatal("expected error")
			}
			if len(got.Sources) != 0 {
				t.Fatal("partial composition returned")
			}
			if tt.name == "weakening" {
				for _, want := range []string{"evidence-integrity", "project-guidance", GuidancePath, ProtocolPath, "remove weaken-controls"} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("error missing %q: %v", want, err)
					}
				}
			}
		})
	}
}

func TestComposeRejectsPolicyDeclarationInProjectGuidanceWithoutPartialResult(t *testing.T) {
	root := fixture(t, "git-strategy: direct-main\n")

	got, err := Compose(root)
	if err == nil {
		t.Fatal("expected error")
	}
	if len(got.Sources) != 0 {
		t.Fatal("partial composition returned")
	}
	for _, want := range []string{"project-guidance", GuidancePath, "git-strategy", "policy layer", PolicyPath, "remove"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q: %v", want, err)
		}
	}
}

func TestComposeRejectsUnsupportedDeclarationsByLayer(t *testing.T) {
	root := fixture(t, "custom-setting: enabled\n")
	got, err := Compose(root)
	if err == nil || !strings.Contains(err.Error(), `unsupported declaration "custom-setting"`) {
		t.Fatalf("unsupported declaration error = %v", err)
	}
	if len(got.Sources) != 0 {
		t.Fatal("partial composition returned")
	}
}

func fixture(t *testing.T, guidance string) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		path := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(path), 0755)
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(ProtocolPath, "---\ninstruction-layer: protocol\nprotected-controls:\n  - evidence-integrity\n  - completed-review-immutability\n  - workflow-artifact-ownership\n  - invalid-state-refusal\n---\n# Protocol\n")
	write(PolicyPath, "---\ninstruction-layer: policy\nrequired-phases:\n  - planning\napproval-gates:\n  - independent-review\ngit-strategy: task-branch\n---\n# Policy\n")
	write(GuidancePath, "---\ninstruction-layer: project-guidance\n"+guidance+"---\n# Agents\ncustom bytes\n")
	return root
}
