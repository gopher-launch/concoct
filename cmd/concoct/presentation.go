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
		s.closeMessage()
		s.thread, s.turn, s.item = event.ThreadID, event.TurnID, ""
		s.role = roleForThread(event.ThreadID)
		s.resetItems()
		return
	}
	if event.ThreadID != "" && (event.ThreadID != s.thread || event.TurnID != "" && event.TurnID != s.turn) {
		s.closeMessage()
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
			result, parseErr := codexapp.ParseRoleResult(body.Item.Text)
			control := body.Item.Phase == "final" || body.Item.Phase == "" && (parseErr == nil || strings.HasPrefix(strings.TrimSpace(body.Item.Text), "{") && strings.Contains(body.Item.Text, `"disposition"`))
			if control {
				s.closeMessage()
				s.report = &agentReport{Role: s.role, TurnID: s.turn}
				if parseErr != nil {
					s.report.Error = "malformed structured agent result; inspect the role turn and run status"
				} else {
					s.report.Disposition, s.report.Summary = publicName(result.Disposition), result.Summary
				}
			} else if buffered := s.pending[body.Item.ID]; buffered != "" {
				s.emitDelta(body.Item.ID, buffered)
			} else if !s.emitted[body.Item.ID] && body.Item.Text != "" {
				s.emitDelta(body.Item.ID, body.Item.Text)
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
		if json.Unmarshal(event.Raw, &body) != nil || body.Delta == "" || s.phases[body.ItemID] == "final" {
			return
		}
		if s.phases[body.ItemID] == "" && !s.ordinary[body.ItemID] {
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
	}
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

func (s *messageStream) finish() {
	if s.seen && !s.json && !s.quiet && s.breaks == 0 {
		fmt.Fprint(s.w, "\n")
	}
	s.open = false
}
