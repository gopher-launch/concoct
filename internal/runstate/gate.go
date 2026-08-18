// Package runstate owns the private, non-authoritative pending approval gate.
package runstate

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
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

// DecisionRecord is the private, executable-owned durable boundary between a
// Product Owner proposal and later approval/application. It is intentionally
// not accepted product truth: roadmap and capability changes remain canonical
// only after their dedicated application transaction.
type DecisionRecord struct {
	Version       int                           `json:"version"`
	Status        string                        `json:"status"`
	Decision      orchestration.ProductDecision `json:"decision"`
	Evidence      string                        `json:"evidence"`
	State         workflow.State                `json:"state"`
	Correlation   orchestration.Correlation     `json:"correlation"`
	CreatedAt     string                        `json:"created_at"`
	InvalidReason string                        `json:"invalid_reason,omitempty"`
}

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

func DecisionPath(root string) string {
	return filepath.Join(root, ".concoct", "runtime", "product-owner-decision.json")
}

// NewDecision constructs a proposed decision bound to the exact authorization
// snapshot. It does not create a pending approval gate or mutate canonical
// workflow artifacts.
func NewDecision(decision orchestration.ProductDecision, evidence orchestration.Evidence, correlation orchestration.Correlation) (DecisionRecord, error) {
	record := DecisionRecord{Version: 1, Status: "proposed", Decision: decision, Evidence: evidence.Digest, State: evidence.State, Correlation: correlation, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if err := validateDecision(record); err != nil {
		return DecisionRecord{}, err
	}
	return record, nil
}

// CreateDecision publishes a decision once. A caller must explicitly inspect,
// invalidate, or apply the existing record; a new Product Owner invocation may
// not silently replace it.
func CreateDecision(root string, record DecisionRecord) error {
	if err := validateDecision(record); err != nil {
		return err
	}
	return createJSON(DecisionPath(root), record, "Product Owner decision")
}

func LoadDecision(root string) (DecisionRecord, error) {
	if err := validateRuntimePath(root, false); err != nil {
		return DecisionRecord{}, err
	}
	path := DecisionPath(root)
	info, err := os.Lstat(path)
	if err != nil {
		return DecisionRecord{}, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o077 != 0 || info.Size() > maxRecordBytes {
		return DecisionRecord{}, fmt.Errorf("Product Owner decision must be a bounded private regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return DecisionRecord{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var record DecisionRecord
	if err := decoder.Decode(&record); err != nil {
		return DecisionRecord{}, fmt.Errorf("malformed Product Owner decision: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return DecisionRecord{}, fmt.Errorf("malformed Product Owner decision: trailing JSON data")
	}
	if err := validateDecision(record); err != nil {
		return DecisionRecord{}, err
	}
	return record, nil
}

// UpdateDecision atomically replaces one validated private decision record.
// Unlike CreateDecision it is deliberately limited to the record lifecycle;
// it is never a generic product-data writer.
func UpdateDecision(root string, record DecisionRecord) error {
	if err := validateDecision(record); err != nil {
		return err
	}
	path := DecisionPath(root)
	if _, err := LoadDecision(root); err != nil {
		return err
	}
	return replaceJSON(path, record, "Product Owner decision")
}

func InvalidateDecision(root string, record DecisionRecord, reason string) error {
	if strings.TrimSpace(reason) == "" || len(reason) > 512 || strings.Contains(reason, "\x00") {
		return errors.New("Product Owner decision invalidation requires a bounded reason")
	}
	record.Status, record.InvalidReason = "invalidated", reason
	return UpdateDecision(root, record)
}

// ApplyDecision applies only the exact record replacements retained in a
// validated Product Owner decision. Each replacement must still match its
// digest and occur exactly once; this prevents a decision from acting as a
// general Markdown patch after evidence has drifted.
func ApplyDecision(root string, record DecisionRecord) error {
	if record.Status != "proposed" && record.Status != "approved" {
		return fmt.Errorf("Product Owner decision %s cannot be applied", record.Status)
	}
	if len(record.Decision.Mutations) == 0 {
		// A reconciliation may record that accepted evidence was inspected but
		// needs no canonical record replacement. Its semantic evidence remains
		// bounded by the decision record; there is simply no mutation to apply.
		return nil
	}
	// Validate every retained replacement before changing either canonical file.
	// This keeps a rejected multi-record reconciliation entirely non-mutating.
	original := map[string][]byte{}
	updated := map[string][]byte{}
	for _, mutation := range record.Decision.Mutations {
		path := filepath.Join(root, ".concoct", map[string]string{"roadmap": "roadmap.md", "capabilities": "capabilities.md"}[mutation.Target])
		data, ok := updated[path]
		if !ok {
			var err error
			data, err = os.ReadFile(path)
			if err != nil {
				return err
			}
			original[path] = append([]byte(nil), data...)
		}
		before := []byte(mutation.Before)
		digest := fmt.Sprintf("%x", sha256.Sum256(before))
		if digest != mutation.BeforeDigest || bytes.Count(data, before) != 1 {
			return fmt.Errorf("Product Owner %s mutation for %s is stale or ambiguous", mutation.Target, mutation.ID)
		}
		if !strings.HasPrefix(mutation.Before, "## "+mutation.ID+" ") || !strings.HasPrefix(mutation.After, "## "+mutation.ID+" ") {
			return fmt.Errorf("Product Owner mutation for %s is not an exact canonical record", mutation.ID)
		}
		updated[path] = bytes.Replace(data, before, []byte(mutation.After), 1)
	}
	// Digest and heading checks above bind replacements to exact bytes. The
	// workflow layer additionally constrains the semantic transition and its
	// accepted delivery provenance before either canonical file is written.
	roadmapPath := filepath.Join(root, ".concoct", "roadmap.md")
	capabilityPath := filepath.Join(root, ".concoct", "capabilities.md")
	roadmapCandidate := original[roadmapPath]
	if candidate, ok := updated[roadmapPath]; ok {
		roadmapCandidate = candidate
	}
	if roadmapCandidate == nil {
		var err error
		roadmapCandidate, err = os.ReadFile(roadmapPath)
		if err != nil {
			return err
		}
	}
	capabilityCandidate := original[capabilityPath]
	if candidate, ok := updated[capabilityPath]; ok {
		capabilityCandidate = candidate
	}
	if capabilityCandidate == nil {
		var err error
		capabilityCandidate, err = os.ReadFile(capabilityPath)
		if err != nil {
			return err
		}
	}
	changes := make([]workflow.ReconciliationChange, 0, len(record.Decision.Mutations))
	for _, mutation := range record.Decision.Mutations {
		changes = append(changes, workflow.ReconciliationChange{Target: mutation.Target, ID: mutation.ID})
	}
	if err := workflow.ValidateReconciliationCandidate(root, roadmapCandidate, capabilityCandidate, changes); err != nil {
		return err
	}
	paths := []string{roadmapPath, capabilityPath}
	var written []string
	for _, path := range paths {
		data, ok := updated[path]
		if !ok {
			continue
		}
		if err := writeCanonicalAtomic(path, data); err != nil {
			var rollbackErrs []string
			for _, prior := range written {
				if rollbackErr := writeCanonicalAtomic(prior, original[prior]); rollbackErr != nil {
					rollbackErrs = append(rollbackErrs, rollbackErr.Error())
				}
			}
			if len(rollbackErrs) > 0 {
				return fmt.Errorf("apply Product Owner decision: %w; rollback failed: %s", err, strings.Join(rollbackErrs, "; "))
			}
			return fmt.Errorf("apply Product Owner decision: %w; no canonical mutation retained", err)
		}
		written = append(written, path)
	}
	return nil
}

func writeCanonicalAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".product-owner-apply-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Chmod(0o644)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
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
	return createJSON(Path(root), gate, "pending approval gate")
}

func createJSON(path string, value any, name string) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if len(data) > maxRecordBytes {
		return fmt.Errorf("%s exceeds %d bytes", name, maxRecordBytes)
	}
	if err := ensurePrivateDir(filepath.Dir(path)); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".pending-gate-*")
	if err != nil {
		return err
	}
	temporary := tmp.Name()
	defer os.Remove(temporary)
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Chmod(0o600)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := os.Link(temporary, path); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("%s already exists", name)
		}
		return err
	}
	return os.Remove(temporary)
}

func replaceJSON(path string, value any, name string) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if len(data) > maxRecordBytes {
		return fmt.Errorf("%s exceeds %d bytes", name, maxRecordBytes)
	}
	if err := ensurePrivateDir(filepath.Dir(path)); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".product-owner-decision-*")
	if err != nil {
		return err
	}
	temporary := tmp.Name()
	defer os.Remove(temporary)
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Chmod(0o600)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(temporary, path)
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
	if g.Name == "next" && g.Action == "task-planning" && g.Selection == "" {
		return fmt.Errorf("next gate requires a selected roadmap item")
	}
	return nil
}

func validateDecision(record DecisionRecord) error {
	if record.Version != 1 {
		return fmt.Errorf("unsupported Product Owner decision record version %d", record.Version)
	}
	if record.Status != "proposed" && record.Status != "approved" && record.Status != "applied" && record.Status != "invalidated" {
		return fmt.Errorf("unsupported Product Owner decision record status %q", record.Status)
	}
	if err := orchestration.ValidateProductDecision(record.Decision); err != nil {
		return err
	}
	if record.Evidence == "" || len(record.Evidence) > 512 || record.State != workflow.Ready || record.CreatedAt == "" {
		return errors.New("Product Owner decision record lacks bounded ready-state evidence")
	}
	if record.Correlation.InvocationID == "" || record.Correlation.ActionID == "" || record.Correlation.AttemptID == "" || record.Correlation.Role != "product-owner" {
		return errors.New("Product Owner decision record has invalid correlation")
	}
	if len(record.InvalidReason) > 512 || strings.Contains(record.InvalidReason, "\x00") {
		return errors.New("Product Owner decision record has invalid invalidation reason")
	}
	if record.Status == "invalidated" && record.InvalidReason == "" {
		return errors.New("invalidated Product Owner decision requires a reason")
	}
	if record.Status != "invalidated" && record.InvalidReason != "" {
		return errors.New("only an invalidated Product Owner decision may have an invalidation reason")
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
