package adapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Usage preserves optional token fields. Pointers distinguish reported zero
// from an unavailable field.
type Usage struct {
	Input           *int64 `json:"input_tokens,omitempty"`
	CachedInput     *int64 `json:"cached_input_tokens,omitempty"`
	Output          *int64 `json:"output_tokens,omitempty"`
	ReasoningOutput *int64 `json:"reasoning_output_tokens,omitempty"`
	Total           *int64 `json:"total_tokens,omitempty"`
}

// EventEvidence is compact, adapter-neutral evidence extracted from Codex JSONL.
type EventEvidence struct {
	Usage                Usage    `json:"usage"`
	Progress             []string `json:"progress,omitempty"`
	ProgressTruncated    bool     `json:"progress_truncated,omitempty"`
	Diagnostics          []string `json:"diagnostics,omitempty"`
	DiagnosticsTruncated bool     `json:"diagnostics_truncated,omitempty"`
	EventTruncated       bool     `json:"event_truncated,omitempty"`
	Events               int      `json:"events"`
	// Status is supported unless the stream contains an event sequence the
	// adapter cannot interpret as a complete, ordered Codex JSONL transcript.
	// Usage collected before degradation remains observational evidence.
	Status string `json:"status"`
}

// StreamDecoder incrementally consumes a Codex JSONL stream.  It is safe to
// use while the child process is still running: malformed lines are retained
// as diagnostics, not fatal parser errors, so valid evidence emitted later is
// still available after a non-zero exit or cancellation.
type StreamDecoder struct {
	buffer             []byte
	evidence           EventEvidence
	line               int
	terminal           bool
	maxProgressBytes   int
	progressBytes      int
	maxDiagnosticBytes int
	diagnosticBytes    int
	maxEventBytes      int
	discardingEvent    bool
}

const (
	defaultMaxProgressBytes = 2 * 1024
	maxProgressEntries      = 64
	maxDiagnosticEntries    = 64
)

// NewStreamDecoder creates a decoder whose selected progress, diagnostics, and
// in-progress event are each bounded independently from raw event retention. A
// non-positive bound uses the conservative standalone-decoder default.
func NewStreamDecoder(maxEvidenceBytes int64) StreamDecoder {
	if maxEvidenceBytes <= 0 {
		maxEvidenceBytes = defaultMaxProgressBytes
	}
	bound := int(maxEvidenceBytes)
	return StreamDecoder{
		maxProgressBytes:   bound,
		maxDiagnosticBytes: bound,
		maxEventBytes:      bound,
	}
}

// Evidence returns the events completed so far without consuming an
// unterminated final line.
func (d *StreamDecoder) Evidence() EventEvidence { return normalizedEvidence(d.evidence) }

func (d *StreamDecoder) Write(data []byte) (int, error) {
	written := len(data)
	for len(data) > 0 {
		if d.discardingEvent {
			index := bytes.IndexByte(data, '\n')
			if index < 0 {
				return written, nil
			}
			d.line++
			d.discardingEvent = false
			data = data[index+1:]
			continue
		}

		index := bytes.IndexByte(data, '\n')
		if index < 0 {
			if len(d.buffer)+len(data) > d.eventByteLimit() {
				d.discardOversizedEvent()
				return written, nil
			}
			d.buffer = append(d.buffer, data...)
			return written, nil
		}

		line := data[:index]
		if len(d.buffer)+len(line) > d.eventByteLimit() {
			d.discardOversizedEvent()
			d.line++
			d.discardingEvent = false
		} else {
			d.buffer = append(d.buffer, line...)
			d.consume(d.buffer)
			d.buffer = d.buffer[:0]
		}
		data = data[index+1:]
	}
	return written, nil
}

// Close processes a final unterminated JSONL event and returns the observed
// evidence.  Calling it more than once is harmless.
func (d *StreamDecoder) Close() EventEvidence {
	if d.discardingEvent {
		d.line++
		d.discardingEvent = false
	} else if len(bytes.TrimSpace(d.buffer)) != 0 {
		d.consume(d.buffer)
	}
	d.buffer = nil
	d.evidence = normalizedEvidence(d.evidence)
	return d.evidence
}

func (d *StreamDecoder) consume(data []byte) {
	d.line++
	text := bytes.TrimSpace(data)
	if len(text) == 0 {
		return
	}
	d.evidence.Events++
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(text, &raw); err != nil {
		d.appendDiagnostic(fmt.Sprintf("event line %d is malformed JSON", d.line))
		return
	}
	var kind string
	_ = json.Unmarshal(raw["type"], &kind)
	if kind == "" {
		d.appendDiagnostic(fmt.Sprintf("event line %d has no type", d.line))
		return
	}
	if d.terminal {
		d.degrade(fmt.Sprintf("event line %d follows a terminal event", d.line))
	}
	if terminalEvent(kind) {
		d.terminal = true
	}
	d.appendProgress(kind)
	usageRaw := raw["usage"]
	if len(usageRaw) == 0 {
		if item, ok := raw["item"]; ok {
			var nested map[string]json.RawMessage
			if json.Unmarshal(item, &nested) == nil {
				usageRaw = nested["usage"]
			}
		}
	}
	if len(usageRaw) == 0 {
		return
	}
	usage, ok := decodeUsage(usageRaw)
	if !ok {
		d.appendDiagnostic(fmt.Sprintf("event line %d has malformed usage", d.line))
		return
	}
	if hasUsage(d.evidence.Usage) {
		if !sameUsage(d.evidence.Usage, usage) {
			d.appendDiagnostic("contradictory duplicate usage event")
		} else {
			d.appendDiagnostic("duplicate usage event")
		}
		return
	}
	d.evidence.Usage = usage
}

func (d *StreamDecoder) degrade(message string) {
	d.appendDiagnostic(message)
	d.evidence.Status = "degraded"
}

// AddDiagnostic records execution-owned diagnostic evidence through the same
// count and byte ceilings as adapter diagnostics.
func (d *StreamDecoder) AddDiagnostic(message string) {
	d.appendDiagnostic(message)
}

func (d *StreamDecoder) appendDiagnostic(message string) {
	if d.maxDiagnosticBytes == 0 {
		d.maxDiagnosticBytes = defaultMaxProgressBytes
	}
	if len(d.evidence.Diagnostics) >= maxDiagnosticEntries || d.diagnosticBytes+len(message)+1 > d.maxDiagnosticBytes {
		d.evidence.DiagnosticsTruncated = true
		d.evidence.Status = "degraded"
		return
	}
	d.evidence.Diagnostics = append(d.evidence.Diagnostics, message)
	d.diagnosticBytes += len(message) + 1
}

func (d *StreamDecoder) eventByteLimit() int {
	if d.maxEventBytes == 0 {
		d.maxEventBytes = defaultMaxProgressBytes
	}
	return d.maxEventBytes
}

func (d *StreamDecoder) discardOversizedEvent() {
	d.buffer = d.buffer[:0]
	d.discardingEvent = true
	d.evidence.EventTruncated = true
	d.degrade(fmt.Sprintf("event line %d exceeds the %d-byte decoder limit", d.line+1, d.eventByteLimit()))
}

// appendProgress deliberately records only an allowlisted lifecycle label.
// Codex event payloads can contain model, tool, or repository text, so they
// are never copied into progress evidence, display, or measurements.
func (d *StreamDecoder) appendProgress(kind string) {
	if !progressEvent(kind) {
		return
	}
	if d.maxProgressBytes == 0 {
		d.maxProgressBytes = defaultMaxProgressBytes
	}
	if len(d.evidence.Progress) >= maxProgressEntries || d.progressBytes+len(kind)+1 > d.maxProgressBytes {
		d.evidence.ProgressTruncated = true
		return
	}
	d.evidence.Progress = append(d.evidence.Progress, kind)
	// Account for the newline added by the live display path as well as the
	// persisted JSON string. This keeps the selected representation bounded.
	d.progressBytes += len(kind) + 1
}

func normalizedEvidence(e EventEvidence) EventEvidence {
	if e.Status == "" {
		e.Status = "supported"
	}
	return e
}

// HasUsage reports whether the adapter supplied any native token field. It is
// deliberately not an estimate: callers must not turn absent fields into zero.
func (e EventEvidence) HasUsage() bool { return hasUsage(e.Usage) }

// Compatibility returns the explicit state of the parsed event evidence.
// Empty status is retained as supported for records created by older builds.
func (e EventEvidence) Compatibility() string {
	if e.Status == "" {
		return "supported"
	}
	return e.Status
}

// UsageSummary is a compact, stable presentation of the values the adapter
// actually reported. Cached input remains a separate native field.
func (e EventEvidence) UsageSummary() string {
	if !e.HasUsage() {
		return "usage unavailable"
	}
	var fields []string
	for _, field := range []struct {
		name  string
		value *int64
	}{{"input", e.Usage.Input}, {"cached-input", e.Usage.CachedInput}, {"output", e.Usage.Output}, {"reasoning-output", e.Usage.ReasoningOutput}, {"total", e.Usage.Total}} {
		if field.value != nil {
			fields = append(fields, fmt.Sprintf("%s=%d", field.name, *field.value))
		}
	}
	return strings.Join(fields, " ")
}

// DecodeCodexJSONL preserves one usage object and diagnoses malformed or
// repeated usage rather than inventing an aggregate.
func DecodeCodexJSONL(data []byte) EventEvidence {
	decoder := NewStreamDecoder(0)
	_, _ = decoder.Write(data)
	return decoder.Close()
}

func progressEvent(kind string) bool {
	switch kind {
	case "turn.started", "item.started", "turn.failed", "turn.cancelled":
		return true
	default:
		return false
	}
}

// turn.* events are the structured stream lifecycle boundary. item.completed
// is intentionally not terminal: Codex may emit several completed items
// before a turn-completed event, and older fixtures attach usage to it.
func terminalEvent(kind string) bool {
	return kind == "turn.completed" || kind == "turn.failed" || kind == "turn.cancelled"
}
func decodeUsage(raw json.RawMessage) (Usage, bool) {
	var values map[string]json.RawMessage
	if json.Unmarshal(raw, &values) != nil {
		return Usage{}, false
	}
	var u Usage
	for _, field := range []struct {
		keys []string
		dst  **int64
	}{{[]string{"input_tokens", "input"}, &u.Input}, {[]string{"cached_input_tokens", "cached_input"}, &u.CachedInput}, {[]string{"output_tokens", "output"}, &u.Output}, {[]string{"reasoning_output_tokens", "reasoning_output"}, &u.ReasoningOutput}, {[]string{"total_tokens", "total"}, &u.Total}} {
		for _, key := range field.keys {
			if rawValue, found := values[key]; found {
				var n int64
				if json.Unmarshal(rawValue, &n) != nil || n < 0 {
					return Usage{}, false
				}
				*field.dst = &n
				break
			}
		}
	}
	return u, hasUsage(u)
}
func hasUsage(u Usage) bool {
	return u.Input != nil || u.CachedInput != nil || u.Output != nil || u.ReasoningOutput != nil || u.Total != nil
}
func sameUsage(a, b Usage) bool {
	return same(a.Input, b.Input) && same(a.CachedInput, b.CachedInput) && same(a.Output, b.Output) && same(a.ReasoningOutput, b.ReasoningOutput) && same(a.Total, b.Total)
}
func same(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
func DecodeCodexJSONLReader(r io.Reader) EventEvidence {
	decoder := NewStreamDecoder(0)
	_, _ = io.Copy(&decoder, r)
	return decoder.Close()
}
