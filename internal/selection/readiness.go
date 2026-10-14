package selection

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gopher-launch/concoct/internal/roadmap"
)

// ReadyCheckpoint is the structured final engineering checkpoint. Reagent
// writes it through save_checkpoint; the host never invents check results.
type ReadyCheckpoint struct {
	TaskID           string
	Plan             string
	Progress         string
	Criteria         []CriterionEvidence
	Checks           []CheckEvidence
	SelfReview       string
	NextAction       string
	Deliverables     []string
	GeneratedOutputs []string
}

type CriterionEvidence struct {
	Criterion string
	Result    string
	Evidence  string
}

type CheckEvidence struct {
	Command  string
	Source   string
	Result   string
	Evidence string
	Decision string
}

// RequiredCheck is an exact project check supplied by controller configuration.
type RequiredCheck struct{ Command, Source string }

func parseReadyCheckpoint(content string) (ReadyCheckpoint, error) {
	var checkpoint ReadyCheckpoint
	decoder := json.NewDecoder(strings.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&checkpoint); err != nil {
		return checkpoint, fmt.Errorf("ready checkpoint JSON: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return checkpoint, fmt.Errorf("ready checkpoint has trailing or invalid JSON")
	}
	if strings.TrimSpace(checkpoint.TaskID) == "" || strings.TrimSpace(checkpoint.Plan) == "" || strings.TrimSpace(checkpoint.Progress) == "" || strings.TrimSpace(checkpoint.SelfReview) == "" || strings.TrimSpace(checkpoint.NextAction) == "" {
		return checkpoint, fmt.Errorf("ready checkpoint lacks task, plan, progress, self-review or next action")
	}
	return checkpoint, nil
}

func validateReadyCheckpoint(checkpoint ReadyCheckpoint, entry roadmap.Entry, project projectSnapshot, workspace string, changed []string, acceptedUnavailable []string, configured []RequiredCheck) error {
	if checkpoint.TaskID != entry.ID {
		return fmt.Errorf("ready checkpoint task differs from claimed task")
	}
	_, acceptance, err := taskScope(entry)
	if err != nil {
		return err
	}
	criteria := acceptanceItems(acceptance)
	if len(checkpoint.Criteria) != len(criteria) {
		return fmt.Errorf("ready checkpoint covers %d of %d acceptance criteria", len(checkpoint.Criteria), len(criteria))
	}
	covered := make(map[string]bool, len(criteria))
	for _, item := range checkpoint.Criteria {
		if item.Result != "passed" || strings.TrimSpace(item.Evidence) == "" || !slices.Contains(criteria, item.Criterion) || covered[item.Criterion] {
			return fmt.Errorf("ready checkpoint has missing, duplicate or unverified criterion %q", item.Criterion)
		}
		covered[item.Criterion] = true
	}
	for _, path := range checkpoint.Deliverables {
		if !validRelativePath(path) || !slices.Contains(changed, path) {
			return fmt.Errorf("declared deliverable %q is absent from changed source", path)
		}
	}
	required := append(requiredChecks(project, changed), configured...)
	seen := make(map[string]bool)
	for _, check := range checkpoint.Checks {
		if strings.TrimSpace(check.Command) == "" || strings.TrimSpace(check.Source) == "" || strings.TrimSpace(check.Evidence) == "" || (check.Result != "passed" && check.Result != "unavailable") {
			return fmt.Errorf("check has incomplete command, source, result or evidence")
		}
		if check.Result == "unavailable" && strings.TrimSpace(check.Decision) == "" {
			return fmt.Errorf("unavailable check %q lacks an explicit acceptance decision", check.Command)
		}
		if check.Result == "unavailable" && !slices.Contains(acceptedUnavailable, check.Command) {
			return fmt.Errorf("unavailable check %q has no recorded operator acceptance", check.Command)
		}
		if !validRelativePath(check.Source) {
			return fmt.Errorf("invalid check source %q", check.Source)
		}
		info, err := os.Stat(filepath.Join(workspace, check.Source))
		if err != nil {
			return fmt.Errorf("check source %q: %w", check.Source, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("check source %q is not a regular file", check.Source)
		}
		key := check.Source + "\x00" + check.Command
		if seen[key] {
			return fmt.Errorf("duplicate check %q", check.Command)
		}
		seen[key] = true
	}
	for _, need := range required {
		found := false
		for _, check := range checkpoint.Checks {
			if check.Source == need.Source && commandMatches(need.Command, check.Command, changed, workspace) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("mandatory check %q from %s lacks candidate-bound evidence", need.Command, need.Source)
		}
	}
	return nil
}

func requiredChecks(project projectSnapshot, changed []string) []RequiredCheck {
	var checks []RequiredCheck
	goChanged, cliChanged := false, false
	for _, path := range changed {
		goChanged = goChanged || strings.HasSuffix(path, ".go")
		cliChanged = cliChanged || strings.HasPrefix(path, "cmd/")
	}
	if goChanged {
		for _, command := range []string{"gofmt", "go test ./...", "go vet ./...", "go build ./cmd/concoct", "git diff --check"} {
			if strings.Contains(project.Agents, string(rune(96))+command+string(rune(96))) {
				checks = append(checks, RequiredCheck{command, "AGENTS.md"})
			}
		}
	}
	if cliChanged && strings.Contains(project.Agents, "Run focused smoke checks for CLI behavior") {
		checks = append(checks, RequiredCheck{"cli-smoke", "AGENTS.md"})
	}
	return checks
}

func commandMatches(need, actual string, changed []string, workspace string) bool {
	actual = strings.TrimPrefix(actual, "GOTOOLCHAIN=local ")
	switch need {
	case "gofmt":
		if !strings.HasPrefix(actual, "gofmt -w ") {
			return false
		}
		fields := strings.Fields(actual)
		for _, path := range changed {
			if _, err := os.Stat(filepath.Join(workspace, path)); os.IsNotExist(err) {
				continue
			}
			if strings.HasSuffix(path, ".go") && !slices.Contains(fields, path) {
				return false
			}
		}
		return true
	case "cli-smoke":
		return strings.Contains(actual, "concoct") && (strings.Contains(actual, "help") || strings.Contains(actual, "version"))
	default:
		if strings.HasPrefix(need, "go ") {
			want, got := strings.Fields(need), strings.Fields(actual)
			return len(got) >= 3 && len(want) == 3 && got[0] == want[0] && got[1] == want[1] && slices.Contains(got[2:], want[2])
		}
		return actual == need
	}
}

func acceptanceItems(acceptance string) []string {
	var items []string
	var current string
	for _, line := range strings.Split(acceptance, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") {
			if current != "" {
				items = append(items, current)
			}
			current = strings.TrimSpace(strings.TrimPrefix(line, "- "))
		} else if line != "" && current != "" {
			current += " " + line
		}
	}
	if current != "" {
		items = append(items, current)
	}
	if len(items) == 0 && strings.TrimSpace(acceptance) != "" {
		items = append(items, strings.Join(strings.Fields(acceptance), " "))
	}
	return items
}

func validRelativePath(path string) bool {
	return path != "" && !filepath.IsAbs(path) && filepath.Clean(path) == path && path != "." && !strings.HasPrefix(path, "..") && !strings.ContainsRune(path, 0)
}
