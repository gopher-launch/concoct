package contract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompatibility(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".concoct")
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckRead(root); err == nil || !strings.Contains(err.Error(), "legacy/unversioned") {
		t.Fatalf("missing = %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, "project.yaml"), []byte("contract-version: 2\ncreated-with: {version: development, revision: unknown}\nlast-upgraded-with: {version: development, revision: unknown}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckRead(root); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("newer = %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, "project.yaml"), []byte("contract-version: 1\ncreated-with: {version: development, revision: unknown}\nlast-upgraded-with: {version: development, revision: unknown}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := CheckMutate(root); err != nil {
		t.Fatal(err)
	}
}

func TestReadRejectsIncompleteAndMultiDocumentRecords(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".concoct")
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}
	valid := "contract-version: 1\ncreated-with: {version: development, revision: unknown}\nlast-upgraded-with: {version: development, revision: unknown}\n"
	for name, body := range map[string]string{
		"empty":                "",
		"missing-created":      "contract-version: 1\nlast-upgraded-with: {version: development, revision: unknown}\n",
		"missing-last-upgrade": "contract-version: 1\ncreated-with: {version: development, revision: unknown}\n",
		"empty-provenance":     "contract-version: 1\ncreated-with: {version: '', revision: unknown}\nlast-upgraded-with: {version: development, revision: unknown}\n",
		"multiple-documents":   valid + "---\ncontract-version: 1\n",
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(path, "project.yaml"), []byte(body), 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := CheckRead(root); err == nil || !strings.Contains(err.Error(), "malformed project contract") {
				t.Fatalf("CheckRead error = %v", err)
			}
			if err := CheckMutate(root); err == nil || !strings.Contains(err.Error(), "malformed project contract") {
				t.Fatalf("CheckMutate error = %v", err)
			}
		})
	}
}
