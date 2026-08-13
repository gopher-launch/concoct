// Package runstate owns the private, non-authoritative pending approval gate.
package runstate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gopher-launch/concoct/internal/orchestration"
	"github.com/gopher-launch/concoct/internal/workflow"
)

const maxRecordBytes = 16 * 1024

// Gate binds one approval to the exact forthcoming action and repository
// evidence. Selection is populated only for the invariant next gate.
type Gate struct {
	Version       int            `json:"version"`
	Name          string         `json:"gate"`
	Action        string         `json:"action"`
	TaskID        string         `json:"task_id,omitempty"`
	AttemptID     string         `json:"attempt_id"`
	ActionID      string         `json:"action_id,omitempty"`
	InvocationID  string         `json:"invocation_id,omitempty"`
	Evidence      string         `json:"evidence"`
	ConfigDigest  string         `json:"config_digest"`
	State         workflow.State `json:"state"`
	Selection     string         `json:"selection,omitempty"`
	Prerequisites []string       `json:"prerequisites,omitempty"`
	CreatedAt     string         `json:"created_at"`
}

func Path(root string) string {
	return filepath.Join(root, ".concoct", "runtime", "pending-gate.json")
}

// New constructs a bounded gate from current evidence and optional action
// correlation. It does not write anything.
func New(name, action, task, selection string, evidence orchestration.Evidence, correlation orchestration.Correlation, configDigest string) (Gate, error) {
	attempt := correlation.AttemptID
	if attempt == "" {
		var err error
		attempt, err = orchestration.NewID()
		if err != nil {
			return Gate{}, err
		}
	}
	g := Gate{Version: 1, Name: name, Action: action, TaskID: task, AttemptID: attempt, ActionID: correlation.ActionID, InvocationID: correlation.InvocationID, Evidence: evidence.Digest, ConfigDigest: configDigest, State: evidence.State, Selection: selection, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	return g, validate(g)
}

// Create publishes one gate atomically without replacing an existing gate.
func Create(root string, gate Gate) error {
	if err := validate(gate); err != nil {
		return err
	}
	data, err := json.MarshalIndent(gate, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if len(data) > maxRecordBytes {
		return fmt.Errorf("pending gate exceeds %d bytes", maxRecordBytes)
	}
	path := Path(root)
	if err := ensurePrivateDir(filepath.Dir(path)); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".pending-gate-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Chmod(0o600)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := os.Link(name, path); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("pending approval gate already exists")
		}
		return err
	}
	return os.Remove(name)
}

func Load(root string) (Gate, error) {
	if err := validateRuntimePath(root, false); err != nil {
		return Gate{}, err
	}
	return loadPath(Path(root))
}

func loadPath(path string) (Gate, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return Gate{}, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return Gate{}, fmt.Errorf("pending gate must be a regular non-symlink file")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return Gate{}, fmt.Errorf("pending gate permissions must not grant group or other access")
	}
	if info.Size() > maxRecordBytes {
		return Gate{}, fmt.Errorf("pending gate exceeds %d bytes", maxRecordBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Gate{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var gate Gate
	if err := dec.Decode(&gate); err != nil {
		return Gate{}, fmt.Errorf("malformed pending gate: %w", err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return Gate{}, fmt.Errorf("malformed pending gate: trailing JSON data")
	}
	if err := validate(gate); err != nil {
		return Gate{}, err
	}
	return gate, nil
}

// ValidateCurrent rejects stale or wrong-gate approval without mutation.
func ValidateCurrent(gate Gate, requested string, evidence orchestration.Evidence, configDigest string) error {
	if gate.Name != requested {
		return fmt.Errorf("pending gate is %q, not requested approval %q", gate.Name, requested)
	}
	if gate.Evidence != evidence.Digest || gate.State != evidence.State {
		return fmt.Errorf("pending %s approval is stale; repository or workflow evidence changed", gate.Name)
	}
	if gate.ConfigDigest != configDigest {
		return fmt.Errorf("pending %s approval is stale; execution configuration changed", gate.Name)
	}
	return nil
}

// Consume atomically claims and removes the already-validated one-use gate.
func Consume(root string, gate Gate) error {
	if err := validateRuntimePath(root, false); err != nil {
		return err
	}
	path := Path(root)
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".claim-*")
	if err != nil {
		return err
	}
	claimed := tmp.Name()
	defer os.Remove(claimed)
	if closeErr := tmp.Close(); closeErr != nil {
		return closeErr
	}
	if err := os.Remove(claimed); err != nil {
		return err
	}
	if err := os.Rename(path, claimed); err != nil {
		return fmt.Errorf("consume pending %s approval: %w", gate.Name, err)
	}
	actual, err := loadPath(claimed)
	if err == nil && !sameGate(actual, gate) {
		err = fmt.Errorf("pending %s approval changed before consumption", gate.Name)
	}
	if err != nil {
		if restoreErr := os.Rename(claimed, path); restoreErr != nil {
			return fmt.Errorf("%v; restore changed pending gate: %w", err, restoreErr)
		}
		return err
	}
	return os.Remove(claimed)
}

func validate(g Gate) error {
	known := map[string]bool{"next": true, "plan": true, "development": true, "review": true, "archive": true, "integration": true}
	if g.Version != 1 {
		return fmt.Errorf("unsupported pending gate version %d", g.Version)
	}
	if !known[g.Name] {
		return fmt.Errorf("unsupported pending gate %q", g.Name)
	}
	for name, value := range map[string]string{"action": g.Action, "attempt_id": g.AttemptID, "evidence": g.Evidence, "config_digest": g.ConfigDigest, "created_at": g.CreatedAt} {
		if strings.TrimSpace(value) == "" || len(value) > 512 || strings.Contains(value, "\x00") {
			return fmt.Errorf("pending gate has invalid %s", name)
		}
	}
	for _, value := range []string{g.TaskID, g.ActionID, g.InvocationID, g.Selection} {
		if len(value) > 512 || strings.Contains(value, "\x00") {
			return fmt.Errorf("pending gate contains an invalid bounded field")
		}
	}
	seenPrerequisite := map[string]bool{}
	for _, prerequisite := range g.Prerequisites {
		if prerequisite != "plan" || g.Action != "development" || seenPrerequisite[prerequisite] || prerequisite == g.Name {
			return fmt.Errorf("pending gate contains an invalid prerequisite %q", prerequisite)
		}
		seenPrerequisite[prerequisite] = true
	}
	if g.Name == "next" && g.Selection == "" {
		return fmt.Errorf("next gate requires a selected roadmap item")
	}
	return nil
}

func sameGate(a, b Gate) bool {
	return a.Version == b.Version && a.Name == b.Name && a.Action == b.Action && a.TaskID == b.TaskID && a.AttemptID == b.AttemptID && a.ActionID == b.ActionID && a.InvocationID == b.InvocationID && a.Evidence == b.Evidence && a.ConfigDigest == b.ConfigDigest && a.State == b.State && a.Selection == b.Selection && strings.Join(a.Prerequisites, "\x00") == strings.Join(b.Prerequisites, "\x00") && a.CreatedAt == b.CreatedAt
}

func ensurePrivateDir(path string) error {
	root := filepath.Dir(filepath.Dir(path))
	if err := validateRuntimePath(root, true); err != nil {
		return err
	}
	return nil
}

// validateRuntimePath rejects redirected or non-directory project/runtime
// components before any private-state mutation. Only the final runtime
// directory may be created, after the project-owned .concoct directory has
// been verified with Lstat.
func validateRuntimePath(root string, create bool) error {
	concoct := filepath.Join(root, ".concoct")
	info, err := os.Lstat(concoct)
	if os.IsNotExist(err) && create {
		if err := os.Mkdir(concoct, 0o755); err != nil {
			return err
		}
		info, err = os.Lstat(concoct)
	}
	if err != nil {
		return fmt.Errorf("inspect private state parent: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("private state parent .concoct must be a non-symlink directory")
	}
	runtime := filepath.Join(concoct, "runtime")
	info, err = os.Lstat(runtime)
	if os.IsNotExist(err) && create {
		if err := os.Mkdir(runtime, 0o700); err != nil {
			return err
		}
		info, err = os.Lstat(runtime)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("private runtime path must be a non-symlink directory")
	}
	if info.Mode().Perm() != 0o700 {
		if !create {
			return fmt.Errorf("private runtime directory permissions must be 0700")
		}
		if err := os.Chmod(runtime, 0o700); err != nil {
			return err
		}
	}
	return nil
}
