package selection

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

// ReadyCheckpoint retains the agent's narrative for the operator and reviewer.
// Only explicit file exclusions are read by candidate capture. Engineering
// coverage and test sufficiency belong to the reagent and iragent.
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

// RequiredCheck is guidance supplied to the reagent, not a host acceptance gate.
type RequiredCheck struct{ Command, Source string }

func checkpointPaths(content string) (deliverables, generated []string) {
	var checkpoint ReadyCheckpoint
	if json.Unmarshal([]byte(content), &checkpoint) == nil {
		return checkpoint.Deliverables, checkpoint.GeneratedOutputs
	}
	return nil, nil
}

func validRelativePath(path string) bool {
	return path != "" && !filepath.IsAbs(path) && filepath.Clean(path) == path && path != "." && !strings.HasPrefix(path, "..") && !strings.ContainsRune(path, 0)
}
