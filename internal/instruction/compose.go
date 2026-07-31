// Package instruction composes the owned instruction layers used by Concoct.
package instruction

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	ProtocolPath = ".concoct/protocol.md"
	PolicyPath   = ".concoct/policy.md"
	GuidancePath = "AGENTS.md"
)

var protected = []string{
	"completed-review-immutability",
	"evidence-integrity",
	"invalid-state-refusal",
	"workflow-artifact-ownership",
}

var allowedDeclarations = map[string][]string{
	"protocol":         {"instruction-layer", "protected-controls"},
	"policy":           {"instruction-layer", "required-phases", "approval-gates", "git-strategy", "strengthen-controls", "weaken-controls"},
	"project-guidance": {"instruction-layer", "strengthen-controls", "weaken-controls"},
}

var policyDeclarations = []string{"required-phases", "approval-gates", "git-strategy"}

// Source is one attributed instruction layer. Content is preserved byte-for-byte.
type Source struct {
	Layer, Path string
	Content     []byte
}

// Effective is the validated, deterministic instruction composition.
type Effective struct{ Sources []Source }

// Compose loads protocol, policy, and project guidance in precedence order.
// It returns no partial composition when any declaration is invalid.
func Compose(root string) (Effective, error) {
	paths := []struct{ layer, path string }{
		{"protocol", ProtocolPath}, {"policy", PolicyPath}, {"project-guidance", GuidancePath},
	}
	var out Effective
	for _, item := range paths {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(item.path)))
		if err != nil {
			return Effective{}, fmt.Errorf("compose %s layer from %s: %w; restore the required source before rendering guidance", item.layer, item.path, err)
		}
		decl, err := declarations(data)
		if err != nil {
			return Effective{}, fmt.Errorf("compose %s layer from %s: %w; correct the declaration before rendering guidance", item.layer, item.path, err)
		}
		if got := decl.scalar["instruction-layer"]; got != item.layer {
			return Effective{}, fmt.Errorf("compose %s layer from %s: instruction-layer is %q; set it to %q before rendering guidance", item.layer, item.path, got, item.layer)
		}
		if err := validateDeclarationOwnership(item.layer, item.path, decl); err != nil {
			return Effective{}, err
		}
		if item.layer == "protocol" {
			if err := exactControls(decl.lists["protected-controls"], protected); err != nil {
				return Effective{}, fmt.Errorf("compose protocol layer from %s: %w; restore the Concoct-owned protocol before rendering guidance", item.path, err)
			}
		}
		if item.layer != "protocol" {
			for _, control := range decl.lists["weaken-controls"] {
				if contains(protected, control) {
					return Effective{}, fmt.Errorf("%s layer %s weakens protocol invariant %q owned by protocol layer %s; remove weaken-controls and use strengthen-controls for compatible stricter guidance", item.layer, item.path, control, ProtocolPath)
				}
			}
			for _, control := range decl.lists["strengthen-controls"] {
				if !contains(protected, control) {
					return Effective{}, fmt.Errorf("%s layer %s strengthens unknown protocol control %q; use a control declared by protocol layer %s", item.layer, item.path, control, ProtocolPath)
				}
			}
		}
		if item.layer == "policy" {
			for _, key := range policyDeclarations {
				if len(decl.lists[key]) == 0 && decl.scalar[key] == "" {
					return Effective{}, fmt.Errorf("policy layer %s is missing %s; restore the default policy before rendering guidance", item.path, key)
				}
			}
		}
		out.Sources = append(out.Sources, Source{Layer: item.layer, Path: item.path, Content: data})
	}
	return out, nil
}

func validateDeclarationOwnership(layer, path string, decl declarationSet) error {
	for _, key := range decl.keys() {
		if contains(allowedDeclarations[layer], key) {
			continue
		}
		if layer == "project-guidance" && contains(policyDeclarations, key) {
			return fmt.Errorf("project-guidance layer %s declares policy-owned key %q, conflicting with policy layer %s; remove %s from %s and select workflow policy in %s before rendering guidance", path, key, PolicyPath, key, path, PolicyPath)
		}
		return fmt.Errorf("%s layer %s contains unsupported declaration %q; remove it or move the setting to its owning instruction layer before rendering guidance", layer, path, key)
	}
	return nil
}

type declarationSet struct {
	scalar map[string]string
	lists  map[string][]string
}

func (d declarationSet) keys() []string {
	keys := make(map[string]struct{}, len(d.scalar)+len(d.lists))
	for key := range d.scalar {
		keys[key] = struct{}{}
	}
	for key := range d.lists {
		keys[key] = struct{}{}
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	return ordered
}

func declarations(data []byte) (declarationSet, error) {
	d := declarationSet{map[string]string{}, map[string][]string{}}
	lines := strings.Split(string(data), "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return d, fmt.Errorf("missing declaration front matter")
	}
	current, closed := "", false
	for _, raw := range lines[1:] {
		line := strings.TrimSpace(raw)
		if line == "---" {
			closed = true
			break
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "- ") {
			if current == "" {
				return d, fmt.Errorf("list item without a declaration key")
			}
			d.lists[current] = append(d.lists[current], strings.TrimSpace(strings.TrimPrefix(line, "- ")))
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return d, fmt.Errorf("malformed declaration %q", line)
		}
		current = strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), "`\"")
		if value != "" {
			d.scalar[current] = value
			current = ""
		}
	}
	if !closed {
		return d, fmt.Errorf("unterminated declaration front matter")
	}
	return d, nil
}

func exactControls(got, want []string) error {
	a, b := append([]string(nil), got...), append([]string(nil), want...)
	sort.Strings(a)
	sort.Strings(b)
	if strings.Join(a, "\x00") != strings.Join(b, "\x00") {
		return fmt.Errorf("protected-controls are %v, want %v", a, b)
	}
	return nil
}
func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}
