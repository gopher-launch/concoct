// Package defaults provides the executable-owned Concoct guidance resources.
package defaults

import (
	"fmt"
	"io/fs"
	"sort"

	concoct "github.com/gopher-launch/concoct"
	"github.com/gopher-launch/concoct/internal/buildinfo"
)

func Provenance() string { return buildinfo.Current().Provenance() }

type Resource struct{ ID, Kind, Path string }

var resources = []Resource{
	{"protocol", "protocol", ".concoct/protocol.md"},
	{"persona-api-writer", "persona", ".concoct/personas/api-writer.md"}, {"persona-archivist", "persona", ".concoct/personas/archivist.md"}, {"persona-code-writer", "persona", ".concoct/personas/code-writer.md"}, {"persona-developer", "persona", ".concoct/personas/developer.md"}, {"persona-product-owner", "persona", ".concoct/personas/product-owner.md"}, {"persona-reviewer", "persona", ".concoct/personas/reviewer.md"}, {"persona-task-planner", "persona", ".concoct/personas/task-planner.md"}, {"persona-user-writer", "persona", ".concoct/personas/user-writer.md"},
	{"prompt-readme", "prompt-documentation", ".concoct/prompts/README.md"}, {"handoff-archivist-to-product-owner", "handoff", ".concoct/prompts/handoffs/archivist-to-product-owner.md"}, {"handoff-developer-to-reviewer", "handoff", ".concoct/prompts/handoffs/developer-to-reviewer.md"}, {"handoff-product-owner-to-task-planner", "handoff", ".concoct/prompts/handoffs/product-owner-to-task-planner.md"}, {"handoff-reviewer-blocked", "handoff", ".concoct/prompts/handoffs/reviewer-blocked.md"}, {"handoff-reviewer-to-archivist", "handoff", ".concoct/prompts/handoffs/reviewer-to-archivist.md"}, {"handoff-reviewer-to-developer", "handoff", ".concoct/prompts/handoffs/reviewer-to-developer.md"}, {"handoff-task-planner-to-developer", "handoff", ".concoct/prompts/handoffs/task-planner-to-developer.md"}, {"prompt-human-roadmap-input", "prompt", ".concoct/prompts/roadmap/human-roadmap-input.md"}, {"prompt-next-action-recommendation", "prompt", ".concoct/prompts/roadmap/next-action-recommendation.md"},
}

func List() []Resource {
	out := append([]Resource(nil), resources...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func Read(id, operation string) ([]byte, error) {
	for _, r := range resources {
		if r.ID == id {
			b, err := fs.ReadFile(concoct.Templates, "templates/"+r.Path)
			if err != nil {
				return nil, fmt.Errorf("required built-in resource %q for %s is unavailable in %s: %w; reinstall or rebuild Concoct", id, operation, Provenance(), err)
			}
			return b, nil
		}
	}
	return nil, fmt.Errorf("unknown built-in resource %q; run concoct defaults list", id)
}
