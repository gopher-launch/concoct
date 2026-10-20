package codexapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeServer struct {
	client  *Client
	reader  *io.PipeReader
	writer  *io.PipeWriter
	encoder *json.Encoder
	writeMu sync.Mutex
}

func fake(t *testing.T, handle func(*fakeServer, envelope)) *fakeServer {
	t.Helper()
	toServerR, toServerW := io.Pipe()
	fromServerR, fromServerW := io.Pipe()
	s := &fakeServer{reader: toServerR, writer: fromServerW, encoder: json.NewEncoder(fromServerW)}
	s.client = New(toServerW, fromServerR)
	t.Cleanup(func() { _ = toServerW.Close(); _ = toServerR.Close(); _ = fromServerW.Close(); _ = fromServerR.Close() })
	go func() {
		decoder := json.NewDecoder(toServerR)
		for {
			var msg envelope
			if err := decoder.Decode(&msg); err != nil {
				return
			}
			handle(s, msg)
		}
	}()
	return s
}

func (s *fakeServer) emit(v any) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_ = s.encoder.Encode(v)
}
func (s *fakeServer) reply(msg envelope, value any) {
	s.emit(map[string]any{"id": json.RawMessage(msg.ID), "result": value})
}
func (s *fakeServer) rpcError(msg envelope, text string) {
	s.emit(map[string]any{"id": json.RawMessage(msg.ID), "error": map[string]any{"code": -32000, "message": text}})
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)
	return ctx
}
func settings() RoleSettings {
	return RoleSettings{Role: "reagent", Model: "gpt-6.1-sol", Effort: "high", Sandbox: "danger-full-access", ApprovalPolicy: "never"}
}
func bindTestThread(t *testing.T, s *fakeServer) {
	t.Helper()
	if _, err := s.client.ResumeThread(testContext(t), "thread-1", true, settings()); err != nil {
		t.Fatal(err)
	}
}
func threadReplyValue(sandbox string) any {
	return map[string]any{"thread": map[string]any{"id": "thread-1", "turns": []any{}}, "model": "gpt-6.1-sol", "reasoningEffort": "high", "sandbox": map[string]string{"type": sandbox}, "approvalPolicy": "never"}
}

func TestHandshakeCatalogSettingsAndBackgroundControls(t *testing.T) {
	var seen []string
	var mu sync.Mutex
	s := fake(t, func(s *fakeServer, msg envelope) {
		mu.Lock()
		seen = append(seen, msg.Method)
		mu.Unlock()
		switch msg.Method {
		case "initialize":
			var params struct {
				Capabilities struct {
					Experimental bool `json:"experimentalApi"`
				} `json:"capabilities"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			if !params.Capabilities.Experimental {
				t.Error("experimental API not negotiated")
			}
			s.reply(msg, map[string]string{"userAgent": "codex-cli 0.147.0", "platformOs": "linux"})
		case "initialized":
		case "model/list":
			var params struct {
				Cursor *string `json:"cursor"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			if params.Cursor == nil {
				s.reply(msg, map[string]any{"data": []map[string]any{{"id": "a", "model": "gpt-6.1-sol", "isDefault": true}}, "nextCursor": "next"})
			} else {
				s.reply(msg, map[string]any{"data": []map[string]any{{"id": "b", "model": "gpt-6-astra"}}, "nextCursor": nil})
			}
		case "account/rateLimits/read":
			s.reply(msg, map[string]any{"rateLimits": map[string]any{"primary": nil}})
		case "thread/start":
			var params map[string]any
			_ = json.Unmarshal(msg.Params, &params)
			config, _ := params["config"].(map[string]any)
			if config["model_reasoning_effort"] != "high" || params["sandbox"] != "danger-full-access" || params["approvalPolicy"] != "never" {
				t.Errorf("missing start settings: %v", params)
			}
			if len(params["dynamicTools"].([]any)) != 1 {
				t.Error("dynamic tool not declared")
			}
			s.reply(msg, threadReplyValue("dangerFullAccess"))
		case "thread/resume":
			var params map[string]any
			_ = json.Unmarshal(msg.Params, &params)
			if params["model"] != "gpt-6.1-sol" || params["sandbox"] != "danger-full-access" || params["approvalPolicy"] != "never" {
				t.Errorf("missing resume settings: %v", params)
			}
			if _, exists := params["config"]; exists {
				t.Error("resume sent unsupported effort config")
			}
			s.reply(msg, threadReplyValue("dangerFullAccess"))
		case "thread/read":
			s.reply(msg, map[string]any{"thread": map[string]string{"id": "thread-1"}})
		case "turn/interrupt":
			s.reply(msg, map[string]any{})
		case "thread/backgroundTerminals/list":
			s.reply(msg, map[string]any{"data": []map[string]any{{"processId": "handle-1", "itemId": "item-1", "command": "sleep", "cwd": "/tmp", "osPid": nil}}, "nextCursor": nil})
		case "thread/backgroundTerminals/terminate":
			s.reply(msg, map[string]bool{"terminated": true})
		case "thread/backgroundTerminals/clean":
			s.reply(msg, map[string]any{})
		default:
			t.Errorf("unexpected request %q", msg.Method)
		}
	})
	ctx := testContext(t)
	if err := s.client.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	models, err := s.client.Models(ctx)
	if err != nil || len(models) != 2 {
		t.Fatalf("models=%v, %v", models, err)
	}
	quota, err := s.client.RateLimits(ctx)
	if err != nil || !strings.Contains(string(quota), "rateLimits") {
		t.Fatalf("quota=%s, %v", quota, err)
	}
	tool := ToolSpec{Type: "function", Name: "checkpoint", Description: "Record work", InputSchema: json.RawMessage(`{"type":"object"}`)}
	thread, err := s.client.StartThread(ctx, "/tmp", settings(), []ToolSpec{tool})
	if err != nil || !thread.Provisional {
		t.Fatalf("start=%#v, %v", thread, err)
	}
	thread, err = s.client.ResumeThread(ctx, "thread-1", false, settings())
	if err != nil || !thread.Provisional {
		t.Fatalf("resume=%#v, %v", thread, err)
	}
	wrongRole := settings()
	wrongRole.Role = "iragent"
	if _, err := s.client.RunTurn(ctx, "thread-1", "work", wrongRole, nil, nil); err == nil || !strings.Contains(err.Error(), "not bound") {
		t.Fatalf("wrong role dispatch=%v", err)
	}
	if _, err := s.client.ReadThread(ctx, "thread-1"); err != nil {
		t.Fatal(err)
	}
	if err := s.client.Interrupt(ctx, "thread-1", "turn-1"); err != nil {
		t.Fatal(err)
	}
	terminals, err := s.client.BackgroundTerminals(ctx, "thread-1")
	if err != nil || len(terminals) != 1 || terminals[0].ProcessID != "handle-1" || terminals[0].OSPID != nil {
		t.Fatalf("terminals=%v, %v", terminals, err)
	}
	terminated, err := s.client.TerminateBackgroundTerminal(ctx, "thread-1", "handle-1")
	if err != nil || !terminated {
		t.Fatalf("terminate=%v, %v", terminated, err)
	}
	if err := s.client.CleanBackgroundTerminals(ctx, "thread-1"); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) < 10 {
		t.Fatalf("incomplete request sequence: %v", seen)
	}
}

func TestInterleavedStreamingCallbackAndTerminalReceipt(t *testing.T) {
	var committed atomic.Int32
	var callbackReply atomic.Bool
	large := strings.Repeat("ordinary output ", 20000)
	s := fake(t, func(s *fakeServer, msg envelope) {
		if msg.Method == "thread/resume" {
			s.reply(msg, threadReplyValue("dangerFullAccess"))
			return
		}
		if msg.Method == "turn/start" {
			s.emit(map[string]any{"id": 900, "method": "item/tool/call", "params": map[string]any{"threadId": "thread-1", "turnId": "turn-1", "callId": "call-1", "tool": "checkpoint", "arguments": map[string]string{"text": "done"}}})
			s.emit(map[string]any{"method": "turn/started", "params": map[string]any{"threadId": "thread-1", "turn": map[string]any{"id": "turn-1", "status": "inProgress", "items": []any{}}}})
			s.emit(map[string]any{"method": "item/agentMessage/delta", "params": map[string]string{"threadId": "thread-1", "turnId": "turn-1", "itemId": "a", "delta": large}})
			s.reply(msg, map[string]any{"turn": map[string]any{"id": "turn-1", "status": "inProgress", "items": []any{}}})
			return
		}
		if len(msg.ID) > 0 && msg.Method == "" {
			var result struct {
				Success      bool `json:"success"`
				ContentItems []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"contentItems"`
			}
			_ = json.Unmarshal(msg.Result, &result)
			if result.Success && len(result.ContentItems) == 1 && result.ContentItems[0].Type == "inputText" && committed.Load() == 1 {
				callbackReply.Store(true)
			}
			s.emit(map[string]any{"method": "item/completed", "params": map[string]any{"threadId": "thread-1", "turnId": "turn-1", "item": map[string]any{"id": "call-1", "type": "dynamicToolCall", "tool": "checkpoint", "namespace": nil, "status": "completed", "success": true}}})
			s.emit(map[string]any{"method": "item/completed", "params": map[string]any{"threadId": "thread-1", "turnId": "turn-1", "item": map[string]string{"id": "final", "type": "agentMessage", "phase": "final_answer", "text": `{"disposition":"ready","summary":"done"}`}}})
			usage := map[string]int{"inputTokens": 4, "cachedInputTokens": 0, "outputTokens": 3, "reasoningOutputTokens": 1, "totalTokens": 7}
			s.emit(map[string]any{"method": "thread/tokenUsage/updated", "params": map[string]any{"threadId": "thread-1", "turnId": "turn-1", "tokenUsage": map[string]any{"last": usage, "total": usage}}})
			s.emit(map[string]any{"method": "turn/completed", "params": map[string]any{"threadId": "thread-1", "turn": map[string]any{"id": "turn-1", "status": "completed", "items": []any{}}}})
		}
	})
	s.client.SetToolHandler(func(ctx context.Context, call ToolCall) (PreparedToolResponse, error) {
		if call.Role != "reagent" || call.ThreadID != "thread-1" || call.TurnID != "turn-1" || call.CallID != "call-1" || call.Tool != "checkpoint" {
			return PreparedToolResponse{}, fmt.Errorf("wrong callback binding")
		}
		return PreparedToolResponse{Success: true, Text: "recorded", Commit: func(context.Context) error { committed.Add(1); return nil }}, nil
	})
	if _, err := s.client.ResumeThread(testContext(t), "thread-1", true, settings()); err != nil {
		t.Fatal(err)
	}
	var streamed int
	result, err := s.client.RunTurn(testContext(t), "thread-1", "continue", settings(), nil, func(event Event) {
		if event.Kind == "item/agentMessage/delta" {
			streamed += len(event.Raw)
		}
	})
	if err != nil || result.Status != "completed" || result.TurnID != "turn-1" || result.Usage == nil || result.Usage.TotalTokens != 7 || streamed < len(large) || !callbackReply.Load() || len(result.Callbacks) != 1 || !result.Callbacks[0].Success {
		t.Fatalf("turn=%#v err=%v streamed=%d callback=%v", result, err, streamed, callbackReply.Load())
	}
	role, err := ParseRoleResult(result.FinalText)
	if err != nil || role.Disposition != "ready" {
		t.Fatalf("role=%#v, %v", role, err)
	}
	if _, err := ParseRoleResult(`{"summary":"no disposition"}`); !errors.Is(err, ErrMalformedRoleResult) {
		t.Fatalf("malformed role result=%v", err)
	}
}

func TestRevertedSettingsZeroTurnLossAndUnsupportedRunner(t *testing.T) {
	s := fake(t, func(s *fakeServer, msg envelope) {
		switch msg.Method {
		case "thread/start":
			s.reply(msg, threadReplyValue("workspaceWrite"))
		case "thread/resume":
			s.rpcError(msg, "no rollout found")
		}
	})
	ctx := testContext(t)
	if _, err := s.client.StartThread(ctx, "/tmp", settings(), nil); err == nil || !strings.Contains(err.Error(), "effective settings differ") {
		t.Fatalf("reverted access error=%v", err)
	}
	if _, err := s.client.ResumeThread(ctx, "thread-1", false, settings()); !errors.Is(err, ErrProvisionalThread) {
		t.Fatalf("zero-turn loss=%v", err)
	}
	if _, err := s.client.ResumeThread(ctx, "thread-1", true, settings()); errors.Is(err, ErrProvisionalThread) || err == nil {
		t.Fatalf("dispatched-turn loss=%v", err)
	}
	reverted := fake(t, func(s *fakeServer, msg envelope) {
		if msg.Method == "thread/resume" {
			s.reply(msg, threadReplyValue("workspaceWrite"))
		}
	})
	if _, err := reverted.client.ResumeThread(ctx, "thread-1", true, settings()); err == nil || !strings.Contains(err.Error(), "effective settings differ") {
		t.Fatalf("resume access reversion=%v", err)
	}
	script := filepath.Join(t.TempDir(), "fake-codex")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf 'codex-cli 0.146.0\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := Launch(ctx, script, io.Discard); err == nil || !strings.Contains(err.Error(), "unsupported Codex runner") {
		t.Fatalf("unsupported runner=%v", err)
	}
}

func TestZeroTurnResumePreservesOtherFailures(t *testing.T) {
	t.Run("disconnect", func(t *testing.T) {
		s := fake(t, func(s *fakeServer, msg envelope) {
			if msg.Method == "thread/resume" {
				_ = s.writer.Close()
			}
		})
		_, err := s.client.ResumeThread(testContext(t), "thread-1", false, settings())
		if !errors.Is(err, ErrDisconnected) || errors.Is(err, ErrProvisionalThread) {
			t.Fatalf("zero-turn disconnect=%v", err)
		}
	})
	t.Run("other provider error", func(t *testing.T) {
		s := fake(t, func(s *fakeServer, msg envelope) {
			if msg.Method == "thread/resume" {
				s.rpcError(msg, "provider unavailable")
			}
		})
		_, err := s.client.ResumeThread(testContext(t), "thread-1", false, settings())
		var providerErr *rpcError
		if !errors.As(err, &providerErr) || providerErr.Message != "provider unavailable" || errors.Is(err, ErrProvisionalThread) {
			t.Fatalf("zero-turn provider error=%v", err)
		}
	})
	t.Run("timeout", func(t *testing.T) {
		s := fake(t, func(*fakeServer, envelope) {})
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		_, err := s.client.ResumeThread(ctx, "thread-1", false, settings())
		if !errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrProvisionalThread) {
			t.Fatalf("zero-turn timeout=%v", err)
		}
	})
}

func TestMalformedHostRequestCannotCompleteCleanly(t *testing.T) {
	var rejected atomic.Bool
	replied := make(chan struct{})
	s := fake(t, func(s *fakeServer, msg envelope) {
		switch msg.Method {
		case "thread/resume":
			s.reply(msg, threadReplyValue("dangerFullAccess"))
		case "turn/start":
			s.emit(map[string]any{"method": "turn/started", "params": map[string]any{"threadId": "thread-1", "turn": map[string]string{"id": "turn-1"}}})
			s.reply(msg, map[string]any{"turn": map[string]string{"id": "turn-1", "status": "inProgress"}})
			s.emit(map[string]any{"id": 903, "method": "item/tool/call", "params": map[string]any{"threadId": 123, "turnId": "turn-1", "callId": "call-3", "tool": "checkpoint", "arguments": map[string]string{}}})
		case "":
			if len(msg.ID) > 0 {
				var reply struct {
					Success bool `json:"success"`
				}
				_ = json.Unmarshal(msg.Result, &reply)
				rejected.Store(!reply.Success)
				s.emit(map[string]any{"method": "turn/completed", "params": map[string]any{"threadId": "thread-1", "turn": map[string]string{"id": "turn-1", "status": "completed"}}})
				close(replied)
			}
		}
	})
	bindTestThread(t, s)
	_, err := s.client.RunTurn(testContext(t), "thread-1", "work", settings(), nil, nil)
	select {
	case <-replied:
	case <-time.After(3 * time.Second):
		t.Fatal("provider did not receive malformed callback rejection")
	}
	if err == nil || !strings.Contains(err.Error(), "malformed host request") || !rejected.Load() {
		t.Fatalf("malformed callback error=%v rejected=%v", err, rejected.Load())
	}
}

func TestProviderEOFAndFailedCallbackStayDistinct(t *testing.T) {
	t.Run("EOF", func(t *testing.T) {
		s := fake(t, func(s *fakeServer, msg envelope) {
			if msg.Method == "thread/resume" {
				s.reply(msg, threadReplyValue("dangerFullAccess"))
				return
			}
			if msg.Method == "turn/start" {
				_ = s.writer.Close()
			}
		})
		bindTestThread(t, s)
		_, err := s.client.RunTurn(testContext(t), "thread-1", "work", settings(), nil, nil)
		if !errors.Is(err, ErrDisconnected) || !errors.Is(err, ErrUncertainDispatch) {
			t.Fatalf("EOF=%v", err)
		}
	})
	t.Run("invalid callback", func(t *testing.T) {
		var committed atomic.Bool
		s := fake(t, func(s *fakeServer, msg envelope) {
			if msg.Method == "thread/resume" {
				s.reply(msg, threadReplyValue("dangerFullAccess"))
				return
			}
			if msg.Method == "turn/start" {
				s.emit(map[string]any{"method": "turn/started", "params": map[string]any{"threadId": "thread-1", "turn": map[string]string{"id": "turn-1"}}})
				s.reply(msg, map[string]any{"turn": map[string]string{"id": "turn-1", "status": "inProgress"}})
				s.emit(map[string]any{"id": 901, "method": "item/tool/call", "params": map[string]any{"threadId": "thread-1", "turnId": "turn-1", "callId": "call-1", "tool": "checkpoint", "arguments": map[string]string{}}})
				return
			}
			if msg.Method == "" && len(msg.ID) > 0 {
				s.emit(map[string]any{"method": "turn/completed", "params": map[string]any{"threadId": "thread-1", "turn": map[string]string{"id": "turn-1", "status": "completed"}}})
			}
		})
		bindTestThread(t, s)
		s.client.SetToolHandler(func(context.Context, ToolCall) (PreparedToolResponse, error) {
			return PreparedToolResponse{Success: true, Text: "", Commit: func(context.Context) error { committed.Store(true); return nil }}, nil
		})
		outcome, err := s.client.RunTurn(testContext(t), "thread-1", "work", settings(), nil, nil)
		if err == nil || committed.Load() || outcome.Status != "completed" {
			t.Fatalf("invalid callback outcome=%#v error=%v committed=%v", outcome, err, committed.Load())
		}
	})
}

func TestInterruptedTurnKeepsUsageUnknown(t *testing.T) {
	s := fake(t, func(s *fakeServer, msg envelope) {
		if msg.Method == "thread/resume" {
			s.reply(msg, threadReplyValue("dangerFullAccess"))
			return
		}
		if msg.Method != "turn/start" {
			return
		}
		s.emit(map[string]any{"method": "turn/started", "params": map[string]any{"threadId": "thread-1", "turn": map[string]string{"id": "turn-1"}}})
		s.reply(msg, map[string]any{"turn": map[string]string{"id": "turn-1", "status": "inProgress"}})
		s.emit(map[string]any{"method": "turn/completed", "params": map[string]any{"threadId": "thread-1", "turn": map[string]string{"id": "turn-1", "status": "interrupted"}}})
	})
	bindTestThread(t, s)
	outcome, err := s.client.RunTurn(testContext(t), "thread-1", "work", settings(), nil, nil)
	if err == nil || !strings.Contains(err.Error(), "interrupted") || outcome.Status != "interrupted" || outcome.Usage != nil {
		t.Fatalf("interrupted outcome=%#v error=%v", outcome, err)
	}
}

func TestMalformedTerminalIdentityFailsPromptly(t *testing.T) {
	s := fake(t, func(s *fakeServer, msg envelope) {
		if msg.Method == "thread/resume" {
			s.reply(msg, threadReplyValue("dangerFullAccess"))
			return
		}
		if msg.Method != "turn/start" {
			return
		}
		s.emit(map[string]any{"method": "turn/started", "params": map[string]any{"threadId": "thread-1", "turn": map[string]string{"id": "turn-1"}}})
		s.reply(msg, map[string]any{"turn": map[string]string{"id": "turn-1", "status": "inProgress"}})
		s.emit(map[string]any{"method": "turn/completed", "params": map[string]any{"threadId": "thread-1", "turn": map[string]string{"id": "wrong-turn", "status": "completed"}}})
	})
	bindTestThread(t, s)
	_, err := s.client.RunTurn(testContext(t), "thread-1", "work", settings(), nil, nil)
	if err == nil || !strings.Contains(err.Error(), "differs from active turn") {
		t.Fatalf("terminal mismatch=%v", err)
	}
}

func TestUnacknowledgedHostCallbackBlocksCompletion(t *testing.T) {
	s := fake(t, func(s *fakeServer, msg envelope) {
		if msg.Method == "thread/resume" {
			s.reply(msg, threadReplyValue("dangerFullAccess"))
			return
		}
		if msg.Method == "turn/start" {
			s.emit(map[string]any{"method": "turn/started", "params": map[string]any{"threadId": "thread-1", "turn": map[string]string{"id": "turn-1"}}})
			s.reply(msg, map[string]any{"turn": map[string]string{"id": "turn-1", "status": "inProgress"}})
			s.emit(map[string]any{"id": 902, "method": "item/tool/call", "params": map[string]any{"threadId": "thread-1", "turnId": "turn-1", "callId": "call-2", "tool": "checkpoint", "arguments": map[string]string{}}})
			return
		}
		if msg.Method == "" && len(msg.ID) > 0 {
			s.emit(map[string]any{"method": "turn/completed", "params": map[string]any{"threadId": "thread-1", "turn": map[string]string{"id": "turn-1", "status": "completed"}}})
		}
	})
	bindTestThread(t, s)
	s.client.SetToolHandler(func(context.Context, ToolCall) (PreparedToolResponse, error) {
		return PreparedToolResponse{Success: true, Text: "recorded", Commit: func(context.Context) error { return nil }}, nil
	})
	outcome, err := s.client.RunTurn(testContext(t), "thread-1", "work", settings(), nil, nil)
	if !errors.Is(err, ErrUncertainCallback) || outcome.Status != "completed" {
		t.Fatalf("unacknowledged callback outcome=%#v error=%v", outcome, err)
	}
}

func TestDynamicToolCompletionContract(t *testing.T) {
	for _, tc := range []struct {
		name                                             string
		requestID                                        any
		itemID, tool, status                             string
		success, duplicate, itemFirst, unrelatedResolved bool
		wantOK                                           bool
	}{
		{"numeric successful item", 17, "call-1", "checkpoint", "completed", true, false, false, false, true},
		{"string request ID and reordered item", "req-17", "call-1", "checkpoint", "completed", true, false, true, false, true},
		{"duplicate item", 17, "call-1", "checkpoint", "completed", true, true, false, false, true},
		{"unrelated resolved notification", 17, "wrong-call", "checkpoint", "completed", true, false, false, true, false},
		{"failed item", 17, "call-1", "checkpoint", "failed", false, false, false, false, false},
		{"wrong tool", 17, "call-1", "claim_task", "completed", true, false, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var committed atomic.Int32
			s := fake(t, func(s *fakeServer, msg envelope) {
				switch msg.Method {
				case "thread/resume":
					s.reply(msg, threadReplyValue("dangerFullAccess"))
				case "turn/start":
					s.emit(map[string]any{"method": "turn/started", "params": map[string]any{"threadId": "thread-1", "turn": map[string]any{"id": "turn-1"}}})
					s.reply(msg, map[string]any{"turn": map[string]any{"id": "turn-1", "status": "inProgress"}})
					s.emit(map[string]any{"id": tc.requestID, "method": "item/tool/call", "params": map[string]any{"threadId": "thread-1", "turnId": "turn-1", "callId": "call-1", "tool": "checkpoint", "arguments": map[string]any{}}})
				case "":
					if len(msg.ID) == 0 {
						return
					}
					item := map[string]any{"method": "item/completed", "params": map[string]any{"threadId": "thread-1", "turnId": "turn-1", "item": map[string]any{"type": "dynamicToolCall", "id": tc.itemID, "namespace": nil, "tool": tc.tool, "status": tc.status, "success": tc.success}}}
					if tc.itemFirst {
						s.emit(item)
					}
					if tc.unrelatedResolved {
						s.emit(map[string]any{"method": "serverRequest/resolved", "params": map[string]any{"threadId": "thread-1", "requestId": tc.requestID}})
					}
					if !tc.itemFirst {
						s.emit(item)
					}
					if tc.duplicate {
						s.emit(item)
					}
					s.emit(map[string]any{"method": "turn/completed", "params": map[string]any{"threadId": "thread-1", "turn": map[string]any{"id": "turn-1", "status": "completed"}}})
				}
			})
			bindTestThread(t, s)
			s.client.SetToolHandler(func(context.Context, ToolCall) (PreparedToolResponse, error) {
				return PreparedToolResponse{Success: true, Text: "saved", Commit: func(context.Context) error { committed.Add(1); return nil }}, nil
			})
			outcome, err := s.client.RunTurn(testContext(t), "thread-1", "work", settings(), nil, nil)
			if (err == nil) != tc.wantOK || committed.Load() != 1 || len(outcome.Callbacks) != 1 || (outcome.Callbacks[0].RequestID == "\"req-17\"") != (tc.requestID == "req-17") {
				t.Fatalf("outcome=%+v err=%v committed=%d", outcome, err, committed.Load())
			}
		})
	}
}

func TestReadTurnRetainsDynamicToolCompletion(t *testing.T) {
	s := fake(t, func(s *fakeServer, msg envelope) {
		if msg.Method != "thread/read" {
			t.Errorf("unexpected method %s", msg.Method)
			return
		}
		s.reply(msg, map[string]any{"thread": map[string]any{"id": "thread-1", "turns": []any{map[string]any{"id": "turn-1", "status": "completed", "items": []any{map[string]any{"type": "dynamicToolCall", "id": "call-1", "namespace": nil, "tool": "save_checkpoint", "status": "completed", "success": true}, map[string]any{"type": "agentMessage", "phase": nil, "text": `{"disposition":"ready"}`}, map[string]string{"type": "agentMessage", "phase": "commentary", "text": "working"}, map[string]string{"type": "agentMessage", "phase": "final_answer", "text": `{"disposition":"ready_for_review"}`}, map[string]string{"type": "agentMessage", "phase": "commentary", "text": "later"}}}}}})
	})
	observed, err := s.client.ReadTurn(testContext(t), "thread-1", "turn-1")
	if err != nil || !observed.ItemsKnown || len(observed.DynamicTools) != 1 || observed.DynamicTools[0].ID != "call-1" || observed.DynamicTools[0].Success == nil || !*observed.DynamicTools[0].Success || observed.FinalText != `{"disposition":"ready_for_review"}` {
		t.Fatalf("observed=%+v err=%v", observed, err)
	}
}

func TestInstalledRunnerReadOnlySmoke(t *testing.T) {
	if os.Getenv("CONCOCT_CODEX_LIVE") != "1" {
		t.Skip("opt-in installed runner smoke")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	process, err := Launch(ctx, "codex", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer process.Close()
	models, err := process.Models(ctx)
	if err != nil || len(models) == 0 {
		t.Fatalf("installed model catalog: %d models, %v", len(models), err)
	}
	if _, err := process.RateLimits(ctx); err != nil {
		t.Fatalf("installed rate limits: %v", err)
	}
}
