package codexapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

type RoleSettings struct {
	Role           string
	Model          string
	Effort         string
	Sandbox        string
	ApprovalPolicy string
}

func (s RoleSettings) validate() error {
	if (s.Role != "reagent" && s.Role != "iragent") || s.Model == "" || s.Effort == "" || s.Sandbox != "danger-full-access" || s.ApprovalPolicy != "never" {
		return fmt.Errorf("explicit role, model, effort and verified full-access settings are required")
	}
	return nil
}

type ToolSpec struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type Thread struct {
	ID          string
	Provisional bool // metadata-only identity until a turn is dispatched
}

type threadReply struct {
	Thread struct {
		ID    string            `json:"id"`
		Turns []json.RawMessage `json:"turns"`
	} `json:"thread"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoningEffort"`
	Sandbox         struct {
		Type string `json:"type"`
	} `json:"sandbox"`
	ApprovalPolicy string `json:"approvalPolicy"`
}

func verifyThreadReply(reply threadReply, expected RoleSettings) (Thread, error) {
	if reply.Thread.ID == "" {
		return Thread{}, fmt.Errorf("app-server returned no thread ID")
	}
	if reply.Model != expected.Model || reply.ReasoningEffort != expected.Effort || reply.Sandbox.Type != "dangerFullAccess" || reply.ApprovalPolicy != expected.ApprovalPolicy {
		return Thread{}, fmt.Errorf("app-server effective settings differ: model=%q effort=%q sandbox=%q approval=%q", reply.Model, reply.ReasoningEffort, reply.Sandbox.Type, reply.ApprovalPolicy)
	}
	return Thread{ID: reply.Thread.ID, Provisional: len(reply.Thread.Turns) == 0}, nil
}

func (c *Client) StartThread(ctx context.Context, cwd string, settings RoleSettings, tools []ToolSpec) (Thread, error) {
	if err := settings.validate(); err != nil {
		return Thread{}, err
	}
	if !filepath.IsAbs(cwd) {
		return Thread{}, fmt.Errorf("thread cwd must be absolute")
	}
	for _, tool := range tools {
		if tool.Type != "function" || tool.Name == "" || tool.Description == "" || !json.Valid(tool.InputSchema) {
			return Thread{}, fmt.Errorf("invalid dynamic tool specification")
		}
	}
	params := map[string]any{"cwd": cwd, "model": settings.Model, "sandbox": settings.Sandbox, "approvalPolicy": settings.ApprovalPolicy, "config": map[string]any{"model_reasoning_effort": settings.Effort}, "dynamicTools": tools}
	var reply threadReply
	if err := c.call(ctx, "thread/start", params, &reply); err != nil {
		return Thread{}, err
	}
	thread, err := verifyThreadReply(reply, settings)
	if err != nil {
		return Thread{}, err
	}
	thread.Provisional = true // thread/start is metadata, not persisted turn history
	c.mu.Lock()
	c.threads[thread.ID] = settings
	c.mu.Unlock()
	return thread, nil
}

// ResumeThread reapplies settings that 0.147.0 otherwise resets. If a thread
// never dispatched a turn, a missing rollout is a provisional identity loss.
func (c *Client) ResumeThread(ctx context.Context, id string, hadTurn bool, settings RoleSettings) (Thread, error) {
	if err := settings.validate(); err != nil {
		return Thread{}, err
	}
	if id == "" {
		return Thread{}, fmt.Errorf("thread ID is required")
	}
	params := map[string]any{"threadId": id, "model": settings.Model, "sandbox": settings.Sandbox, "approvalPolicy": settings.ApprovalPolicy}
	var reply threadReply
	if err := c.call(ctx, "thread/resume", params, &reply); err != nil {
		if !hadTurn {
			return Thread{}, fmt.Errorf("%w: %v", ErrProvisionalThread, err)
		}
		return Thread{}, err
	}
	if reply.Thread.ID != id {
		return Thread{}, fmt.Errorf("resumed thread ID differs from requested ID")
	}
	thread, err := verifyThreadReply(reply, settings)
	if err != nil {
		return Thread{}, err
	}
	thread.Provisional = !hadTurn && len(reply.Thread.Turns) == 0
	c.mu.Lock()
	c.threads[thread.ID] = settings
	c.mu.Unlock()
	return thread, nil
}

type Model struct {
	ID                        string `json:"id"`
	Model                     string `json:"model"`
	IsDefault                 bool   `json:"isDefault"`
	Hidden                    bool   `json:"hidden"`
	DefaultReasoningEffort    string `json:"defaultReasoningEffort"`
	SupportedReasoningEfforts []struct {
		ReasoningEffort string `json:"reasoningEffort"`
	} `json:"supportedReasoningEfforts"`
}

func (c *Client) Models(ctx context.Context) ([]Model, error) {
	var all []Model
	cursor := ""
	for {
		var reply struct {
			Data       []Model `json:"data"`
			NextCursor *string `json:"nextCursor"`
		}
		if err := c.call(ctx, "model/list", map[string]any{"cursor": nilIfEmpty(cursor)}, &reply); err != nil {
			return nil, err
		}
		all = append(all, reply.Data...)
		if reply.NextCursor == nil || *reply.NextCursor == "" {
			return all, nil
		}
		if *reply.NextCursor == cursor {
			return nil, fmt.Errorf("model catalog repeated cursor")
		}
		cursor = *reply.NextCursor
	}
}

func nilIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// RateLimits returns the provider's bounded snapshot as JSON. The controller
// does not reinterpret quota balances as exact per-task spend.
func (c *Client) RateLimits(ctx context.Context) (json.RawMessage, error) {
	var reply json.RawMessage
	if err := c.call(ctx, "account/rateLimits/read", nil, &reply); err != nil {
		return nil, err
	}
	if len(reply) == 0 || string(reply) == "null" {
		return nil, fmt.Errorf("empty rate-limit snapshot")
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(reply, &fields) != nil || len(fields["rateLimits"]) == 0 || string(fields["rateLimits"]) == "null" {
		return nil, fmt.Errorf("malformed rate-limit snapshot")
	}
	return reply, nil
}

func (c *Client) ReadThread(ctx context.Context, id string) (json.RawMessage, error) {
	if id == "" {
		return nil, fmt.Errorf("thread ID is required")
	}
	var reply struct {
		Thread json.RawMessage `json:"thread"`
	}
	if err := c.call(ctx, "thread/read", map[string]any{"threadId": id, "includeTurns": true}, &reply); err != nil {
		return nil, err
	}
	if len(reply.Thread) == 0 {
		return nil, fmt.Errorf("thread/read returned no thread")
	}
	return reply.Thread, nil
}

func (c *Client) Interrupt(ctx context.Context, threadID, turnID string) error {
	if threadID == "" || turnID == "" {
		return fmt.Errorf("thread and turn IDs are required")
	}
	return c.call(ctx, "turn/interrupt", map[string]string{"threadId": threadID, "turnId": turnID}, nil)
}

type BackgroundTerminal struct {
	ProcessID string  `json:"processId"`
	ItemID    string  `json:"itemId"`
	Command   string  `json:"command"`
	Cwd       string  `json:"cwd"`
	OSPID     *uint32 `json:"osPid"`
}

func (c *Client) BackgroundTerminals(ctx context.Context, threadID string) ([]BackgroundTerminal, error) {
	if threadID == "" {
		return nil, fmt.Errorf("thread ID is required")
	}
	var all []BackgroundTerminal
	cursor := ""
	for {
		var reply struct {
			Data       []BackgroundTerminal `json:"data"`
			NextCursor *string              `json:"nextCursor"`
		}
		if err := c.call(ctx, "thread/backgroundTerminals/list", map[string]any{"threadId": threadID, "cursor": nilIfEmpty(cursor)}, &reply); err != nil {
			return nil, err
		}
		for _, terminal := range reply.Data {
			if terminal.ProcessID == "" {
				return nil, fmt.Errorf("background terminal has no provider process handle")
			}
		}
		all = append(all, reply.Data...)
		if reply.NextCursor == nil || *reply.NextCursor == "" {
			return all, nil
		}
		if *reply.NextCursor == cursor {
			return nil, fmt.Errorf("background terminal list repeated cursor")
		}
		cursor = *reply.NextCursor
	}
}

func (c *Client) TerminateBackgroundTerminal(ctx context.Context, threadID, processID string) (bool, error) {
	if threadID == "" || processID == "" {
		return false, fmt.Errorf("thread ID and provider process handle are required")
	}
	var reply struct {
		Terminated bool `json:"terminated"`
	}
	if err := c.call(ctx, "thread/backgroundTerminals/terminate", map[string]string{"threadId": threadID, "processId": processID}, &reply); err != nil {
		return false, err
	}
	return reply.Terminated, nil
}

func (c *Client) CleanBackgroundTerminals(ctx context.Context, threadID string) error {
	if strings.TrimSpace(threadID) == "" {
		return fmt.Errorf("thread ID is required")
	}
	return c.call(ctx, "thread/backgroundTerminals/clean", map[string]string{"threadId": threadID}, nil)
}

func IsProvisionalThread(err error) bool { return errors.Is(err, ErrProvisionalThread) }
