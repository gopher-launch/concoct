// Package roadmap reads the bounded Markdown metadata used by the rebuild
// backlog. It does not select a task or change either source document.
package roadmap

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

var (
	idPattern      = regexp.MustCompile(`^[A-Z][A-Z0-9]*-[A-Z]*[0-9]{3,}$`)
	idLikePattern  = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*-[A-Za-z]*[0-9]{3,}$`)
	entryHeading   = regexp.MustCompile(`^## ([A-Za-z][A-Za-z0-9]*-[A-Za-z0-9]+) — (.+)$`)
	metadataBullet = regexp.MustCompile(`^- ([A-Za-z ]+): (.+)$`)
)

// Entry contains only the metadata needed for readiness and the original prose.
type Entry struct {
	ID        string
	Title     string
	Kind      string
	Status    string
	Priority  string
	Parent    string
	DependsOn []string
	Body      string
	Line      int
}

// Document is a validated snapshot of roadmap and capability metadata.
type Document struct {
	entries   []Entry
	delivered map[string]bool
}

// Entries returns entries in source order. Body retains objective and acceptance
// prose; the parser does not interpret it.
func (d *Document) Entries() []Entry {
	out := make([]Entry, len(d.entries))
	for i, entry := range d.entries {
		out[i] = copyEntry(entry)
	}
	return out
}

// EligibleTasks returns ready tasks whose dependencies are delivered. It keeps
// roadmap order and does not choose among tasks or interpret their prose.
func (d *Document) EligibleTasks() []Entry {
	var out []Entry
	for _, entry := range d.entries {
		if entry.Kind != "task" || entry.Status != "ready" {
			continue
		}
		ready := true
		for _, dependency := range entry.DependsOn {
			if !d.delivered[dependency] {
				ready = false
				break
			}
		}
		if ready {
			out = append(out, copyEntry(entry))
		}
	}
	return out
}

func copyEntry(entry Entry) Entry {
	entry.DependsOn = slices.Clone(entry.DependsOn)
	return entry
}

// Parse validates roadmap metadata and resolves delivered task IDs from both
// the roadmap and concise capability headings. It never reads archives.
func Parse(roadmapMarkdown, capabilitiesMarkdown []byte) (*Document, error) {
	sections, err := splitSections(string(roadmapMarkdown))
	if err != nil {
		return nil, fmt.Errorf("roadmap: %w", err)
	}
	entries := make([]Entry, 0)
	byID := make(map[string]Entry)
	for _, section := range sections {
		id, title, isEntry, err := parseHeading(section.lines[0], section.line)
		if err != nil {
			return nil, fmt.Errorf("roadmap: %w", err)
		}
		if !isEntry {
			continue
		}
		if previous, exists := byID[id]; exists {
			return nil, fmt.Errorf("roadmap line %d: duplicate ID %s (first at line %d)", section.line, id, previous.Line)
		}
		entry, err := parseEntry(section, id, title)
		if err != nil {
			return nil, fmt.Errorf("roadmap: %w", err)
		}
		entries = append(entries, entry)
		byID[id] = entry
	}

	capabilityIDs, err := parseCapabilities(string(capabilitiesMarkdown))
	if err != nil {
		return nil, err
	}
	delivered := make(map[string]bool, len(capabilityIDs))
	for id := range capabilityIDs {
		if entry, exists := byID[id]; exists && (entry.Kind != "task" || entry.Status != "delivered") {
			return nil, fmt.Errorf("capabilities: %s is recorded as delivered but roadmap status is %s %s", id, entry.Status, entry.Kind)
		}
		delivered[id] = true
	}
	for _, entry := range entries {
		if entry.Kind == "task" && entry.Status == "delivered" {
			delivered[entry.ID] = true
		}
	}
	if err := validate(entries, byID, delivered); err != nil {
		return nil, err
	}
	return &Document{entries: entries, delivered: delivered}, nil
}

type section struct {
	line  int
	lines []string
}

func splitSections(markdown string) ([]section, error) {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	var sections []section
	var current *section
	fenceChar := byte(0)
	fenceLength := 0
	for i, line := range lines {
		if ch, count, ok := fenceMarker(line); ok {
			if fenceChar == 0 {
				fenceChar, fenceLength = ch, count
			} else if ch == fenceChar && count >= fenceLength && strings.TrimSpace(strings.TrimLeft(line, " ")[count:]) == "" {
				fenceChar = 0
			}
		} else if fenceChar == 0 && strings.HasPrefix(line, "## ") {
			if current != nil {
				sections = append(sections, *current)
			}
			current = &section{line: i + 1}
		}
		if current != nil {
			current.lines = append(current.lines, line)
		}
	}
	if fenceChar != 0 {
		return nil, fmt.Errorf("unclosed code fence")
	}
	if current != nil {
		sections = append(sections, *current)
	}
	return sections, nil
}

func fenceMarker(line string) (byte, int, bool) {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 || len(trimmed) < 3 {
		return 0, 0, false
	}
	ch := trimmed[0]
	if ch != '`' && ch != '~' {
		return 0, 0, false
	}
	count := 0
	for count < len(trimmed) && trimmed[count] == ch {
		count++
	}
	if count < 3 {
		return 0, 0, false
	}
	return ch, count, true
}

func parseHeading(line string, number int) (id, title string, isEntry bool, err error) {
	match := entryHeading.FindStringSubmatch(line)
	if match != nil {
		if !idPattern.MatchString(match[1]) {
			return "", "", false, fmt.Errorf("line %d: malformed ID %q", number, match[1])
		}
		return match[1], match[2], true, nil
	}
	first := strings.Fields(strings.TrimPrefix(line, "## "))
	if len(first) > 0 && looksLikeID(first[0]) {
		return "", "", false, fmt.Errorf("line %d: malformed entry heading %q; expected '## ID — title'", number, line)
	}
	return "", "", false, nil
}

func looksLikeID(token string) bool {
	if !idLikePattern.MatchString(token) {
		return false
	}
	prefix, _, _ := strings.Cut(token, "-")
	return prefix == strings.ToUpper(prefix) || prefix == strings.ToLower(prefix)
}

func parseEntry(section section, id, title string) (Entry, error) {
	entry := Entry{ID: id, Title: title, Line: section.line}
	fields := make(map[string]bool)
	bodyStart := 1
	for bodyStart < len(section.lines) && strings.TrimSpace(section.lines[bodyStart]) == "" {
		bodyStart++
	}
	for bodyStart < len(section.lines) && strings.HasPrefix(section.lines[bodyStart], "- ") {
		line := section.lines[bodyStart]
		match := metadataBullet.FindStringSubmatch(line)
		if match == nil {
			return Entry{}, fmt.Errorf("line %d: malformed metadata bullet %q", section.line+bodyStart, line)
		}
		key, value := match[1], strings.TrimSpace(match[2])
		if fields[key] {
			return Entry{}, fmt.Errorf("line %d: duplicate %s metadata for %s", section.line+bodyStart, key, id)
		}
		fields[key] = true
		switch key {
		case "Kind":
			entry.Kind = value
		case "Status":
			entry.Status = value
		case "Priority":
			entry.Priority = value
		case "Parent":
			if value != "none" {
				entry.Parent = value
			}
		case "Depends on":
			if value != "none" {
				for _, part := range strings.Split(value, ",") {
					dependency := strings.TrimSpace(part)
					if dependency == "" {
						return Entry{}, fmt.Errorf("line %d: empty dependency for %s", section.line+bodyStart, id)
					}
					entry.DependsOn = append(entry.DependsOn, dependency)
				}
			}
		default:
			return Entry{}, fmt.Errorf("line %d: unsupported metadata field %q for %s", section.line+bodyStart, key, id)
		}
		bodyStart++
	}
	for _, key := range []string{"Kind", "Status", "Priority"} {
		if !fields[key] {
			return Entry{}, fmt.Errorf("line %d: missing %s metadata for %s", section.line, key, id)
		}
	}
	if !slices.Contains([]string{"epic", "feature", "task"}, entry.Kind) {
		return Entry{}, fmt.Errorf("line %d: unsupported kind %q for %s", section.line, entry.Kind, id)
	}
	if !slices.Contains([]string{"planned", "ready", "delivered"}, entry.Status) {
		return Entry{}, fmt.Errorf("line %d: unsupported status %q for %s", section.line, entry.Status, id)
	}
	if !slices.Contains([]string{"high", "medium", "low"}, entry.Priority) {
		return Entry{}, fmt.Errorf("line %d: unsupported priority %q for %s", section.line, entry.Priority, id)
	}
	if entry.Kind != "task" && entry.Status == "ready" {
		return Entry{}, fmt.Errorf("line %d: only tasks may be ready (%s)", section.line, id)
	}
	body := section.lines[bodyStart:]
	fenceChar := byte(0)
	fenceLength := 0
	for offset, line := range body {
		if ch, count, ok := fenceMarker(line); ok {
			if fenceChar == 0 {
				fenceChar, fenceLength = ch, count
			} else if ch == fenceChar && count >= fenceLength && strings.TrimSpace(strings.TrimLeft(line, " ")[count:]) == "" {
				fenceChar = 0
			}
			continue
		}
		if fenceChar != 0 {
			continue
		}
		for _, key := range []string{"Kind", "Status", "Priority", "Parent", "Depends on"} {
			if strings.HasPrefix(line, "- "+key+":") {
				return Entry{}, fmt.Errorf("line %d: %s metadata for %s is separated from the opening metadata block", section.line+bodyStart+offset, key, id)
			}
		}
	}
	for len(body) > 0 && strings.TrimSpace(body[0]) == "" {
		body = body[1:]
	}
	for len(body) > 0 && strings.TrimSpace(body[len(body)-1]) == "" {
		body = body[:len(body)-1]
	}
	entry.Body = strings.Join(body, "\n")
	return entry, nil
}

func parseCapabilities(markdown string) (map[string]bool, error) {
	sections, err := splitSections(markdown)
	if err != nil {
		return nil, fmt.Errorf("capabilities: %w", err)
	}
	ids := make(map[string]bool)
	for _, section := range sections {
		id, _, isEntry, err := parseHeading(section.lines[0], section.line)
		if err != nil {
			return nil, fmt.Errorf("capabilities: %w", err)
		}
		if !isEntry {
			continue
		}
		if ids[id] {
			return nil, fmt.Errorf("capabilities line %d: duplicate delivered ID %s", section.line, id)
		}
		ids[id] = true
	}
	return ids, nil
}

func validate(entries []Entry, byID map[string]Entry, delivered map[string]bool) error {
	for _, entry := range entries {
		if entry.Parent != "" {
			parent, exists := byID[entry.Parent]
			if !exists {
				return fmt.Errorf("roadmap line %d: unknown parent %s for %s", entry.Line, entry.Parent, entry.ID)
			}
			if entry.Kind == "epic" {
				return fmt.Errorf("roadmap line %d: epic %s cannot have a parent", entry.Line, entry.ID)
			}
			expected := map[string]string{"feature": "epic", "task": "feature"}[entry.Kind]
			if parent.Kind != expected {
				return fmt.Errorf("roadmap line %d: %s parent %s must be a %s", entry.Line, entry.ID, entry.Parent, expected)
			}
		}
		seen := make(map[string]bool)
		for _, dependency := range entry.DependsOn {
			if entry.Kind != "task" {
				return fmt.Errorf("roadmap line %d: only tasks may have dependencies (%s)", entry.Line, entry.ID)
			}
			if !idPattern.MatchString(dependency) {
				return fmt.Errorf("roadmap line %d: malformed dependency ID %q for %s", entry.Line, dependency, entry.ID)
			}
			if seen[dependency] {
				return fmt.Errorf("roadmap line %d: duplicate dependency %s for %s", entry.Line, dependency, entry.ID)
			}
			seen[dependency] = true
			if referenced, exists := byID[dependency]; exists {
				if referenced.Kind != "task" {
					return fmt.Errorf("roadmap line %d: dependency %s for %s is not a task", entry.Line, dependency, entry.ID)
				}
			} else if !delivered[dependency] {
				return fmt.Errorf("roadmap line %d: unknown dependency %s for %s", entry.Line, dependency, entry.ID)
			}
		}
	}
	state := make(map[string]uint8)
	var visit func(string) error
	visit = func(id string) error {
		if state[id] == 1 {
			return fmt.Errorf("roadmap: dependency cycle includes %s", id)
		}
		if state[id] == 2 {
			return nil
		}
		state[id] = 1
		for _, dependency := range byID[id].DependsOn {
			if _, exists := byID[dependency]; exists {
				if err := visit(dependency); err != nil {
					return err
				}
			}
		}
		state[id] = 2
		return nil
	}
	for _, entry := range entries {
		if err := visit(entry.ID); err != nil {
			return err
		}
	}
	return nil
}
