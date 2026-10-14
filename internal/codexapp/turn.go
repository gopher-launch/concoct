package codexapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
)

type Event struct {
	Kind      string
	ThreadID  string
	TurnID    string
	RequestID string
	Raw       json.RawMessage
	Err       error
}

type Usage struct {
	InputTokens           int64 `json:"inputTokens"`
	CachedInputTokens     int64 `json:"cachedInputTokens"`
	OutputTokens          int64 `json:"outputTokens"`
	ReasoningOutputTokens int64 `json:"reasoningOutputTokens"`
	TotalTokens           int64 `json:"totalTokens"`
}

type TurnOutcome struct {
	ThreadID      string
	TurnID        string
	Status        string // completed, interrupted or failed; no verdict is inferred
	FinalText     string
	Usage         *Usage // nil means no provider receipt observed
	ProviderError string
}

type turnRun struct {
	mu          sync.Mutex
	role        string
	turnID      string
	terminal    bool
	events      chan Event
	done        chan struct{}
	ready       chan struct{}
	operationMu sync.Mutex
}

func (r *turnRun) setTurnID(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.turnID == "" {
		r.turnID = id
		close(r.ready)
	}
}
func (r *turnRun) getTurnID() string { r.mu.Lock(); defer r.mu.Unlock(); return r.turnID }
func (r *turnRun) markTerminal()     { r.mu.Lock(); r.terminal = true; r.mu.Unlock() }
func (r *turnRun) isTerminal() bool  { r.mu.Lock(); defer r.mu.Unlock(); return r.terminal }
func (r *turnRun) close()            { r.operationMu.Lock(); close(r.done); r.operationMu.Unlock() }
func (r *turnRun) commit(ctx context.Context, fn func(context.Context) error) error {
	r.operationMu.Lock()
	defer r.operationMu.Unlock()
	select {
	case <-r.done:
		return fmt.Errorf("turn ended before host receipt commit")
	default:
	}
	return fn(ctx)
}
func (r *turnRun) emit(event Event) {
	select {
	case r.events <- event:
	case <-r.done:
	}
}

// RunTurn streams events as received, including while the provider is still
// executing. A completed provider turn is not an approved role result.
func (c *Client) RunTurn(ctx context.Context, threadID, prompt string, settings RoleSettings, outputSchema json.RawMessage, onEvent func(Event)) (TurnOutcome, error) {
	result := TurnOutcome{ThreadID: threadID}
	if err := settings.validate(); err != nil {
		return result, err
	}
	if threadID == "" || prompt == "" {
		return result, fmt.Errorf("thread ID and prompt are required")
	}
	if len(outputSchema) > 0 && !json.Valid(outputSchema) {
		return result, fmt.Errorf("invalid output schema")
	}
	run := &turnRun{role: settings.Role, events: make(chan Event, 256), done: make(chan struct{}), ready: make(chan struct{})}
	c.mu.Lock()
	if bound, ok := c.threads[threadID]; !ok || bound != settings {
		c.mu.Unlock()
		return result, fmt.Errorf("thread is not bound to verified role settings")
	}
	if _, exists := c.runs[threadID]; exists {
		c.mu.Unlock()
		return result, fmt.Errorf("thread already has an active turn")
	}
	c.runs[threadID] = run
	c.mu.Unlock()
	defer func() { c.mu.Lock(); delete(c.runs, threadID); c.mu.Unlock(); run.close() }()
	params := map[string]any{"threadId": threadID, "input": []map[string]string{{"type": "text", "text": prompt}}, "effort": settings.Effort, "model": settings.Model}
	if len(outputSchema) > 0 {
		params["outputSchema"] = outputSchema
	}
	startCh, err := c.request("turn/start", params)
	if err != nil {
		return result, err
	}
	started := false
	terminal := false
	pendingCallbacks := 0
	callbackAcknowledged := make(map[string]bool)
	var callbackErr error
	for {
		if terminal && started && pendingCallbacks == 0 {
			if result.TurnID == "" {
				return result, fmt.Errorf("turn completed without identity")
			}
			if callbackErr != nil {
				return result, callbackErr
			}
			for id, acknowledged := range callbackAcknowledged {
				if !acknowledged {
					return result, fmt.Errorf("%w: provider did not resolve callback %s", ErrUncertainCallback, id)
				}
			}
			switch result.Status {
			case "completed":
				return result, nil
			case "interrupted":
				return result, fmt.Errorf("provider turn interrupted")
			case "failed":
				return result, fmt.Errorf("provider turn failed: %s", result.ProviderError)
			default:
				return result, fmt.Errorf("unknown provider turn status %q", result.Status)
			}
		}
		select {
		case <-ctx.Done():
			if !started {
				return result, fmt.Errorf("%w: %w", ErrUncertainDispatch, ctx.Err())
			}
			return result, ctx.Err()
		case reply := <-startCh:
			startCh = nil
			if reply.Err != nil {
				return result, fmt.Errorf("%w: %w", ErrUncertainDispatch, reply.Err)
			}
			if reply.Error != nil {
				return result, fmt.Errorf("app-server turn/start: %d %s", reply.Error.Code, reply.Error.Message)
			}
			var value struct {
				Turn struct {
					ID     string `json:"id"`
					Status string `json:"status"`
				} `json:"turn"`
			}
			if err := json.Unmarshal(reply.Result, &value); err != nil || value.Turn.ID == "" {
				return result, fmt.Errorf("invalid turn/start result: %v", err)
			}
			if known := run.getTurnID(); known != "" && known != value.Turn.ID {
				return result, fmt.Errorf("turn/start identity differs from turn/started event")
			}
			run.setTurnID(value.Turn.ID)
			result.TurnID = value.Turn.ID
			started = true
		case event := <-run.events:
			if event.Kind == "disconnected" {
				if !started {
					return result, fmt.Errorf("%w: %w", ErrUncertainDispatch, event.Err)
				}
				return result, event.Err
			}
			if event.Kind == "protocol/error" {
				return result, event.Err
			}
			if onEvent != nil && event.Kind != "callback/done" && event.Kind != "callback/started" {
				onEvent(event)
			}
			switch event.Kind {
			case "turn/started":
				if event.TurnID != "" {
					result.TurnID = event.TurnID
				}
			case "item/completed":
				var value struct {
					Item struct {
						Type  string `json:"type"`
						Text  string `json:"text"`
						Phase string `json:"phase"`
					} `json:"item"`
				}
				if json.Unmarshal(event.Raw, &value) == nil && value.Item.Type == "agentMessage" && (value.Item.Phase == "final" || value.Item.Phase == "") {
					result.FinalText = value.Item.Text
				}
			case "thread/tokenUsage/updated":
				var value struct {
					TokenUsage struct {
						Last json.RawMessage `json:"last"`
					} `json:"tokenUsage"`
				}
				if json.Unmarshal(event.Raw, &value) == nil {
					var fields map[string]json.RawMessage
					if json.Unmarshal(value.TokenUsage.Last, &fields) == nil && fields["inputTokens"] != nil && fields["totalTokens"] != nil && fields["outputTokens"] != nil && fields["cachedInputTokens"] != nil && fields["reasoningOutputTokens"] != nil {
						var usage Usage
						if json.Unmarshal(value.TokenUsage.Last, &usage) == nil {
							result.Usage = &usage
						}
					}
				}
			case "callback/started":
				pendingCallbacks++
				callbackAcknowledged[event.RequestID] = false
			case "callback/done":
				pendingCallbacks--
				if event.Err != nil && callbackErr == nil {
					callbackErr = event.Err
				}
			case "callback/error":
				if callbackErr == nil {
					callbackErr = event.Err
				}
			case "serverRequest/resolved":
				var value struct {
					RequestID json.RawMessage `json:"requestId"`
				}
				if json.Unmarshal(event.Raw, &value) == nil {
					if _, ok := callbackAcknowledged[string(value.RequestID)]; ok {
						callbackAcknowledged[string(value.RequestID)] = true
					}
				}
			case "turn/completed":
				var value struct {
					Turn struct {
						ID     string `json:"id"`
						Status string `json:"status"`
						Error  *struct {
							Message string `json:"message"`
						} `json:"error"`
					} `json:"turn"`
				}
				if err := json.Unmarshal(event.Raw, &value); err != nil || value.Turn.ID == "" {
					return result, fmt.Errorf("invalid terminal turn event: %v", err)
				}
				if result.TurnID != "" && result.TurnID != value.Turn.ID {
					return result, fmt.Errorf("terminal turn identity differs")
				}
				result.TurnID = value.Turn.ID
				result.Status = value.Turn.Status
				if value.Turn.Error != nil {
					result.ProviderError = value.Turn.Error.Message
				}
				terminal = true
			}
		}
	}
}

type ToolCall struct {
	Role      string          `json:"-"`
	ThreadID  string          `json:"threadId"`
	TurnID    string          `json:"turnId"`
	CallID    string          `json:"callId"`
	Tool      string          `json:"tool"`
	Namespace string          `json:"namespace"`
	Arguments json.RawMessage `json:"arguments"`
}

// PreparedToolResponse separates response validation from durable mutation.
// Commit should record an idempotent receipt before the transport acknowledges
// the call. Its error is sent as a failed inputText response.
type PreparedToolResponse struct {
	Success bool
	Text    string
	Commit  func(context.Context) error
}

type ToolHandler func(context.Context, ToolCall) (PreparedToolResponse, error)

func (c *Client) onRequest(msg envelope) {
	id := json.RawMessage(msg.ID)
	if msg.Method != "item/tool/call" {
		_ = c.send(map[string]any{"id": id, "error": map[string]any{"code": -32601, "message": "unsupported host request"}})
		return
	}
	var call ToolCall
	if err := json.Unmarshal(msg.Params, &call); err != nil {
		c.mu.Lock()
		runs := make([]*turnRun, 0, 1)
		if run := c.runs[call.ThreadID]; run != nil {
			runs = append(runs, run)
		} else {
			for _, active := range c.runs {
				runs = append(runs, active)
			}
		}
		c.mu.Unlock()
		for _, run := range runs {
			run.emit(Event{Kind: "protocol/error", Err: fmt.Errorf("malformed host request %s: %w", id, err)})
		}
		c.replyTool(id, false, "invalid tool call payload")
		return
	}
	c.mu.Lock()
	run := c.runs[call.ThreadID]
	handler := c.handler
	c.mu.Unlock()
	if run == nil || handler == nil || run.isTerminal() || call.TurnID == "" || call.CallID == "" || call.Tool == "" || !json.Valid(call.Arguments) {
		c.replyTool(id, false, "tool call has no active matching thread, turn or handler")
		if run != nil {
			run.emit(Event{Kind: "callback/error", Err: fmt.Errorf("unmatched host callback %q", call.CallID)})
		}
		return
	}
	call.Role = run.role
	run.emit(Event{Kind: "callback/started", ThreadID: call.ThreadID, TurnID: call.TurnID, RequestID: string(id)})
	go func() {
		select {
		case <-run.ready:
		case <-run.done:
			_ = c.replyTool(id, false, "turn ended before callback binding was confirmed")
			return
		}
		if call.TurnID != run.getTurnID() {
			_ = c.replyTool(id, false, "callback turn ID differs from active turn")
			run.emit(Event{Kind: "callback/done", RequestID: string(id), Err: fmt.Errorf("callback turn ID differs from active turn")})
			return
		}
		prepared, err := handler(context.Background(), call)
		if err != nil {
			prepared = PreparedToolResponse{Text: err.Error()}
		}
		if strings.TrimSpace(prepared.Text) == "" {
			prepared = PreparedToolResponse{Text: "invalid empty host response"}
			err = fmt.Errorf("host returned empty tool response")
		}
		if err == nil && prepared.Success && prepared.Commit == nil {
			err = fmt.Errorf("successful host callback has no durable receipt operation")
			prepared = PreparedToolResponse{Text: err.Error()}
		}
		if err == nil && prepared.Commit != nil {
			if commitErr := run.commit(context.Background(), prepared.Commit); commitErr != nil {
				err = commitErr
				prepared = PreparedToolResponse{Text: commitErr.Error()}
			}
		}
		if sendErr := c.replyTool(id, err == nil && prepared.Success, prepared.Text); sendErr != nil {
			err = fmt.Errorf("%w: %v", ErrUncertainCallback, sendErr)
		}
		if err == nil && !prepared.Success {
			err = fmt.Errorf("host callback returned failure")
		}
		if err != nil && !errors.Is(err, ErrUncertainCallback) {
			err = fmt.Errorf("host callback failed: %w", err)
		}
		run.emit(Event{Kind: "callback/done", ThreadID: call.ThreadID, TurnID: call.TurnID, RequestID: string(id), Err: err})
		if errors.Is(err, ErrUncertainCallback) {
			c.fail(err)
		}
	}()
}

func (c *Client) replyTool(id any, success bool, text string) error {
	return c.send(map[string]any{"id": id, "result": map[string]any{"success": success, "contentItems": []map[string]string{{"type": "inputText", "text": text}}}})
}

func IsUncertainCallback(err error) bool { return errors.Is(err, ErrUncertainCallback) }
