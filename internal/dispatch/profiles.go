package dispatch

import (
	"fmt"
	"strings"

	"github.com/gopher-launch/concoct/internal/codexapp"
)

type Role string

const (
	Reagent Role = "reagent"
	Iragent Role = "iragent"
)

type Choice struct {
	Model  string `json:"model"`
	Effort string `json:"effort"`
}

// ProfileSources are Concoct-specific inputs. Ambient editor or Codex settings
// are deliberately absent. Each field follows invocation, project, user,
// then provider-recommended model and Concoct role effort precedence.
type ProfileSources struct {
	Invocation map[Role]Choice
	Project    map[Role]Choice
	User       map[Role]Choice
}

func ResolveProfiles(models []codexapp.Model, sources ProfileSources) (map[Role]codexapp.RoleSettings, error) {
	defaultModel := ""
	byName := make(map[string]codexapp.Model, len(models))
	for _, model := range models {
		if model.Model == "" || model.Hidden {
			continue
		}
		if _, exists := byName[model.Model]; exists {
			return nil, fmt.Errorf("duplicate provider model %q", model.Model)
		}
		byName[model.Model] = model
		if model.IsDefault {
			if defaultModel != "" {
				return nil, fmt.Errorf("provider reported multiple default models")
			}
			defaultModel = model.Model
		}
	}
	if defaultModel == "" {
		return nil, fmt.Errorf("provider reported no recommended default model")
	}
	resolved := make(map[Role]codexapp.RoleSettings, 2)
	for _, role := range []Role{Reagent, Iragent} {
		choice := Choice{Model: defaultModel, Effort: "medium"}
		if role == Iragent {
			choice.Effort = "high"
		}
		for _, layer := range []map[Role]Choice{sources.User, sources.Project, sources.Invocation} {
			if value, ok := layer[role]; ok {
				if strings.TrimSpace(value.Model) != "" {
					choice.Model = value.Model
				}
				if strings.TrimSpace(value.Effort) != "" {
					choice.Effort = value.Effort
				}
			}
		}
		model, ok := byName[choice.Model]
		if !ok {
			return nil, fmt.Errorf("%s model %q is unavailable", role, choice.Model)
		}
		supported := false
		for _, effort := range model.SupportedReasoningEfforts {
			if effort.ReasoningEffort == choice.Effort {
				supported = true
				break
			}
		}
		if !supported {
			return nil, fmt.Errorf("%s model %q does not support effort %q", role, choice.Model, choice.Effort)
		}
		resolved[role] = codexapp.RoleSettings{Role: string(role), Model: choice.Model, Effort: choice.Effort, Sandbox: "danger-full-access", ApprovalPolicy: "never"}
	}
	return resolved, nil
}
