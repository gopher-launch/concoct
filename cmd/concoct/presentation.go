package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/gopher-launch/concoct/internal/codexapp"
)

// publicName changes Concoct's presentation vocabulary, never durable records
// or provider messages.
func publicName(value string) string {
	runes := []rune(value)
	var out strings.Builder
	for i, r := range runes {
		if r == '_' {
			out.WriteByte('-')
			continue
		}
		if r >= 'A' && r <= 'Z' {
			if i > 0 && runes[i-1] != '-' && runes[i-1] != '_' && ((runes[i-1] >= 'a' && runes[i-1] <= 'z') || (runes[i-1] >= '0' && runes[i-1] <= '9') || (runes[i-1] >= 'A' && runes[i-1] <= 'Z' && i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z')) {
				out.WriteByte('-')
			}
			out.WriteRune(r + ('a' - 'A'))
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

func writePublicJSON(w io.Writer, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	var object any
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	if decoder.Decode(&object) == nil {
		_ = json.NewEncoder(w).Encode(publicJSON(object))
	}
}

func publicJSON(value any) any {
	switch value := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(value))
		for key, child := range value {
			if key == "profiles" || key == "quota_windows" {
				if named, ok := child.(map[string]any); ok {
					preserved := make(map[string]any, len(named))
					for name, entry := range named {
						preserved[name] = publicJSON(entry)
					}
					out[publicName(key)] = preserved
					continue
				}
			}
			out[publicName(key)] = publicJSON(child)
		}
		return out
	case []any:
		for i := range value {
			value[i] = publicJSON(value[i])
		}
	}
	return value
}

type agentReport struct {
	Role        string `json:"role"`
	TurnID      string `json:"turn-id,omitempty"`
	Disposition string `json:"disposition,omitempty"`
	Summary     string `json:"summary,omitempty"`
	Error       string `json:"error,omitempty"`
}

type messageStream struct {
	w        io.Writer
	json     bool
	quiet    bool
	role     string
	thread   string
	turn     string
	item     string
	open     bool
	seen     bool
	breaks   int
	phases   map[string]string
	pending  map[string]string
	ordinary map[string]bool
	emitted  map[string]bool
	report   *agentReport
	explicit bool
}

func newMessageStream(w io.Writer, jsonMode, quiet bool) *messageStream {
	return &messageStream{w: w, json: jsonMode, quiet: quiet, role: "agent", phases: make(map[string]string), pending: make(map[string]string), ordinary: make(map[string]bool), emitted: make(map[string]bool)}
}

func (s *messageStream) resetItems() {
	s.phases = make(map[string]string)
	s.pending = make(map[string]string)
	s.ordinary = make(map[string]bool)
	s.emitted = make(map[string]bool)
}

func (s *messageStream) event(event codexapp.Event, roleForThread func(string) string) {
	if event.Kind == "turn/started" {
		s.showReport()
		s.closeMessage()
		s.report = nil
		s.explicit = false
		s.thread, s.turn, s.item = event.ThreadID, event.TurnID, ""
		s.role = roleForThread(event.ThreadID)
		s.resetItems()
		return
	}
	if event.ThreadID != "" && (event.ThreadID != s.thread || event.TurnID != "" && event.TurnID != s.turn) {
		s.closeMessage()
		s.report = nil
		s.explicit = false
		s.thread, s.turn, s.item = event.ThreadID, event.TurnID, ""
		s.role = roleForThread(event.ThreadID)
		s.resetItems()
	}
	switch event.Kind {
	case "item/started", "item/completed":
		var body struct {
			Item struct {
				ID, Type, Phase, Text string
			} `json:"item"`
		}
		if json.Unmarshal(event.Raw, &body) != nil || body.Item.Type != "agentMessage" {
			return
		}
		if body.Item.ID != "" {
			s.phases[body.Item.ID] = body.Item.Phase
		}
		if event.Kind == "item/completed" {
			value := body.Item.Text
			if value == "" {
				value = s.pending[body.Item.ID]
			}
			if body.Item.Phase != "final_answer" {
				if summary, ok := commentarySummary(value); ok {
					s.emitDelta(body.Item.ID, summary)
					delete(s.pending, body.Item.ID)
					if body.Item.ID == s.item {
						s.closeMessage()
					}
					return
				}
			}
			result, parseErr := codexapp.ParseRoleResult(value)
			control := body.Item.Phase == "final_answer" || body.Item.Phase == "" && (parseErr == nil || malformedOrResultShapedObject(value))
			if control {
				if body.Item.Phase == "final_answer" || !s.explicit {
					s.closeMessage()
					if body.Item.Phase == "final_answer" {
						s.explicit = true
					}
					s.report = &agentReport{Role: s.role, TurnID: s.turn}
					if parseErr != nil {
						s.report.Error = "malformed structured agent result; inspect the role turn and run status"
					} else {
						s.report.Disposition, s.report.Summary = publicName(result.Disposition), result.Summary
					}
				}
			} else if buffered := s.pending[body.Item.ID]; buffered != "" {
				s.emitDelta(body.Item.ID, buffered)
			} else if !s.emitted[body.Item.ID] && value != "" {
				s.emitDelta(body.Item.ID, value)
			}
			delete(s.pending, body.Item.ID)
			if body.Item.ID == s.item {
				s.closeMessage()
			}
		}
	case "item/agentMessage/delta":
		var body struct {
			ItemID string `json:"itemId"`
			Delta  string `json:"delta"`
		}
		if json.Unmarshal(event.Raw, &body) != nil || body.Delta == "" || s.phases[body.ItemID] == "final_answer" {
			return
		}
		if !s.ordinary[body.ItemID] {
			s.pending[body.ItemID] += body.Delta
			trimmed := strings.TrimSpace(s.pending[body.ItemID])
			if trimmed == "" || trimmed[0] == '{' {
				return
			}
			s.ordinary[body.ItemID] = true
			body.Delta = s.pending[body.ItemID]
			delete(s.pending, body.ItemID)
		}
		s.emitDelta(body.ItemID, body.Delta)
	case "turn/completed":
		var body struct {
			Turn struct {
				Status string `json:"status"`
			} `json:"turn"`
		}
		if json.Unmarshal(event.Raw, &body) == nil && body.Turn.Status == "completed" {
			s.showReport()
		} else {
			s.report = nil
		}
	}
}

// Null-phase objects may be ordinary JSON commentary or a role result. A
// valid object with no result fields is commentary; malformed objects and
// result-shaped objects stay out of progress even when role parsing fails.
func malformedOrResultShapedObject(text string) bool {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "{") {
		return false
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal([]byte(trimmed), &fields) != nil {
		return true
	}
	_, disposition := fields["disposition"]
	_, summary := fields["summary"]
	return disposition || summary
}

func (s *messageStream) emitDelta(itemID, delta string) {
	if itemID != s.item {
		s.closeMessage()
		s.item = itemID
	}
	s.emitted[itemID] = true
	if s.quiet {
		return
	}
	if s.json {
		writePublicJSON(s.w, map[string]string{"type": "agent-progress", "role": s.role, "thread-id": s.thread, "turn-id": s.turn, "item-id": s.item, "text": delta})
		return
	}
	if !s.open {
		if s.seen {
			for s.breaks < 2 {
				fmt.Fprint(s.w, "\n")
				s.breaks++
			}
		} else {
			fmt.Fprint(s.w, "\n")
		}
		fmt.Fprintf(s.w, "[%s] ", s.role)
		s.open, s.seen = true, true
	}
	fmt.Fprint(s.w, delta)
	for _, char := range delta {
		if char == '\n' {
			s.breaks++
		} else {
			s.breaks = 0
		}
	}
}

func (s *messageStream) closeMessage() { s.open = false }

func (s *messageStream) showReport() {
	if s.report == nil {
		return
	}
	report := s.report
	s.report = nil
	if s.quiet {
		return
	}
	if s.json {
		writePublicJSON(s.w, map[string]any{"type": "agent-result", "role": report.Role, "turn-id": report.TurnID, "disposition": report.Disposition, "summary": report.Summary, "error": report.Error})
		return
	}
	if s.seen {
		for s.breaks < 2 {
			fmt.Fprint(s.w, "\n")
			s.breaks++
		}
	}
	if report.Error != "" {
		fmt.Fprintf(s.w, "[%s result] %s\n", report.Role, report.Error)
	} else {
		fmt.Fprintf(s.w, "[%s result] %s: %s\n", report.Role, report.Disposition, report.Summary)
	}
	s.seen, s.breaks, s.open = true, 1, false
}

func (s *messageStream) finish() {
	s.showReport()
	if s.seen && !s.json && !s.quiet && s.breaks == 0 {
		fmt.Fprint(s.w, "\n")
	}
	s.open = false
}

// commentarySummary recognizes the provider's human-summary envelope without
// using its intermediate disposition or question for coordination.
func commentarySummary(value string) (string, bool) {
	var fields map[string]json.RawMessage
	if json.Unmarshal([]byte(value), &fields) != nil || len(fields) == 0 {
		return "", false
	}
	allowed := map[string]bool{"disposition": true, "summary": true, "task_id": true, "candidate": true, "findings": true, "evidence": true, "question": true}
	for key := range fields {
		if !allowed[key] {
			return "", false
		}
	}
	var summary string
	if json.Unmarshal(fields["summary"], &summary) != nil || strings.TrimSpace(summary) == "" {
		return "", false
	}
	return summary, true
}
