// Package contract validates the executable-owned installed project contract.
package contract

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gopher-launch/concoct/internal/buildinfo"
	"gopkg.in/yaml.v3"
)

const Version = 1
const minReadable, maxReadable = 1, 1
const minMutable, maxMutable = 1, 1

type Provenance struct {
	Version  string `yaml:"version"`
	Revision string `yaml:"revision"`
}
type Record struct {
	ContractVersion  int        `yaml:"contract-version"`
	CreatedWith      Provenance `yaml:"created-with"`
	LastUpgradedWith Provenance `yaml:"last-upgraded-with"`
}

func New() Record {
	i := buildinfo.Current()
	p := Provenance{Version: i.Version, Revision: i.Revision}
	return Record{Version, p, p}
}
func Write(path string) error {
	b, err := yaml.Marshal(New())
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
func Read(root string) (Record, error) {
	var r Record
	b, err := os.ReadFile(filepath.Join(root, ".concoct", "project.yaml"))
	if os.IsNotExist(err) {
		return r, fmt.Errorf("legacy/unversioned project: .concoct/project.yaml is missing; run a future explicit Concoct upgrade before workflow commands")
	}
	if err != nil {
		return r, err
	}
	decoder := yaml.NewDecoder(strings.NewReader(string(b)))
	decoder.KnownFields(true)
	if err = decoder.Decode(&r); err != nil {
		return r, fmt.Errorf("malformed project contract: %w", err)
	}
	var trailing any
	if err = decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return r, fmt.Errorf("malformed project contract: multiple YAML documents are not allowed")
		}
		return r, fmt.Errorf("malformed project contract: %w", err)
	}
	if r.ContractVersion < 1 {
		return r, fmt.Errorf("malformed project contract: contract-version must be positive")
	}
	if err := validateProvenance("created-with", r.CreatedWith); err != nil {
		return r, err
	}
	if err := validateProvenance("last-upgraded-with", r.LastUpgradedWith); err != nil {
		return r, err
	}
	return r, nil
}

func validateProvenance(name string, p Provenance) error {
	if strings.TrimSpace(p.Version) == "" {
		return fmt.Errorf("malformed project contract: %s.version is required", name)
	}
	if strings.TrimSpace(p.Revision) == "" {
		return fmt.Errorf("malformed project contract: %s.revision is required", name)
	}
	return nil
}
func CheckRead(root string) (Record, error) {
	r, err := Read(root)
	if err != nil {
		return r, err
	}
	if r.ContractVersion < minReadable || r.ContractVersion > maxReadable {
		return r, fmt.Errorf("unsupported project contract-version %d; this executable reads %d-%d", r.ContractVersion, minReadable, maxReadable)
	}
	return r, nil
}
func CheckMutate(root string) error {
	r, err := CheckRead(root)
	if err != nil {
		return err
	}
	if r.ContractVersion < minMutable || r.ContractVersion > maxMutable {
		return fmt.Errorf("project contract-version %d is read-compatible but not mutation-compatible; this executable mutates %d-%d", r.ContractVersion, minMutable, maxMutable)
	}
	return nil
}
func Describe(root string) string {
	r, err := CheckRead(root)
	if err != nil {
		return "Project contract: incompatible\nReason: " + err.Error() + "\nRecommended action: inspect with this command only; use a future explicit upgrade.\n"
	}
	return fmt.Sprintf("Project contract: supported\nContract version: %d\nCreated with: %s (%s)\nLast upgraded with: %s (%s)\n", r.ContractVersion, r.CreatedWith.Version, r.CreatedWith.Revision, r.LastUpgradedWith.Version, r.LastUpgradedWith.Revision)
}
