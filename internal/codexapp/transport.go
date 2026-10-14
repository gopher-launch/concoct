// Package codexapp speaks the locally verified Codex 0.147.0 app-server
// subset. It is a transport, not an execution or approval controller.
package codexapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"sync"
)

const TestedVersion = "0.147.0"

var (
	ErrDisconnected        = errors.New("Codex app-server disconnected")
	ErrProvisionalThread   = errors.New("metadata-only thread has no confirmed turn history")
	ErrMalformedRoleResult = errors.New("malformed role result")
	ErrUncertainCallback   = errors.New("host callback acknowledgment is uncertain")
	ErrUncertainDispatch   = errors.New("turn dispatch acknowledgment is uncertain")
)

type rpcReply struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Err error
}

type rpcError struct {
	Method  string
	Code    int
	Message string
}

func (e *rpcError) Error() string {
	return fmt.Sprintf("app-server %s: %d %s", e.Method, e.Code, e.Message)
}

type envelope struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Result json.RawMessage `json:"result"`
}

// Client owns one JSON-lines connection. Calls and turn interruption may run
// concurrently; a single reader correlates replies, notifications and host calls.
type Client struct {
	input     io.Writer
	output    io.Reader
	writeMu   sync.Mutex
	mu        sync.Mutex
	nextID    int64
	pending   map[int64]chan rpcReply
	runs      map[string]*turnRun
	threads   map[string]RoleSettings
	handler   ToolHandler
	closedErr error
	done      chan struct{}
}

func New(input io.Writer, output io.Reader) *Client {
	c := &Client{input: input, output: output, pending: make(map[int64]chan rpcReply), runs: make(map[string]*turnRun), threads: make(map[string]RoleSettings), done: make(chan struct{})}
	go c.read()
	return c
}

// Launch checks the exact tested Linux runner before starting a provider.
// Other versions need explicit schema verification rather than an invented
// compatibility range. The caller closes the returned Process.
func Launch(ctx context.Context, executable string, stderr io.Writer) (*Process, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("Codex %s transport is verified only on Linux", TestedVersion)
	}
	if executable == "" {
		executable = "codex"
	}
	version, err := exec.CommandContext(ctx, executable, "--version").Output()
	if err != nil {
		return nil, fmt.Errorf("check Codex runner: %w", err)
	}
	if strings.TrimSpace(string(version)) != "codex-cli "+TestedVersion {
		return nil, fmt.Errorf("unsupported Codex runner %q; verified version is %s", strings.TrimSpace(string(version)), TestedVersion)
	}
	cmd := exec.Command(executable, "app-server", "--stdio")
	cmd.Stderr = stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start Codex app-server: %w", err)
	}
	p := &Process{cmd: cmd, input: in, Client: New(in, out)}
	if err := p.Client.Initialize(ctx); err != nil {
		p.Close()
		return nil, fmt.Errorf("initialize Codex app-server: %w", err)
	}
	return p, nil
}

type Process struct {
	*Client
	cmd   *exec.Cmd
	input io.Closer
}

func (p *Process) Close() error {
	if p == nil {
		return nil
	}
	_ = p.input.Close()
	if p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	err := p.cmd.Wait()
	if err != nil && !strings.Contains(err.Error(), "signal: killed") {
		return err
	}
	return nil
}

func (c *Client) SetToolHandler(handler ToolHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handler = handler
}

func (c *Client) send(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	data = append(data, '\n')
	for len(data) > 0 {
		n, err := c.input.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

func (c *Client) request(method string, params any) (<-chan rpcReply, error) {
	c.mu.Lock()
	if c.closedErr != nil {
		err := c.closedErr
		c.mu.Unlock()
		return nil, err
	}
	c.nextID++
	id := c.nextID
	ch := make(chan rpcReply, 1)
	c.pending[id] = ch
	c.mu.Unlock()
	if err := c.send(map[string]any{"id": id, "method": method, "params": params}); err != nil {
		c.fail(fmt.Errorf("write app-server request: %w", err))
		return nil, err
	}
	return ch, nil
}

func (c *Client) call(ctx context.Context, method string, params any, result any) error {
	ch, err := c.request(method, params)
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case reply := <-ch:
		if reply.Err != nil {
			return reply.Err
		}
		if reply.Error != nil {
			return &rpcError{Method: method, Code: reply.Error.Code, Message: reply.Error.Message}
		}
		if len(reply.Result) == 0 {
			return fmt.Errorf("app-server %s returned no result", method)
		}
		if result == nil {
			return nil
		}
		if err := json.Unmarshal(reply.Result, result); err != nil {
			return fmt.Errorf("app-server %s result: %w", method, err)
		}
		return nil
	}
}

func (c *Client) Initialize(ctx context.Context) error {
	var result struct {
		UserAgent  string `json:"userAgent"`
		PlatformOS string `json:"platformOs"`
	}
	err := c.call(ctx, "initialize", map[string]any{"clientInfo": map[string]string{"name": "concoct", "version": "0.0.0-dev"}, "capabilities": map[string]bool{"experimentalApi": true}}, &result)
	if err != nil {
		return err
	}
	if result.UserAgent == "" || result.PlatformOS != "linux" {
		return fmt.Errorf("unsupported app-server initialize result: platform=%q userAgent=%q", result.PlatformOS, result.UserAgent)
	}
	if err := c.send(map[string]any{"method": "initialized"}); err != nil {
		c.fail(err)
		return err
	}
	return nil
}

func (c *Client) read() {
	decoder := json.NewDecoder(c.output) // no whole-stream or ordinary-output cap
	for {
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			c.fail(fmt.Errorf("%w: %v", ErrDisconnected, err))
			return
		}
		var msg envelope
		if err := json.Unmarshal(raw, &msg); err != nil {
			c.fail(fmt.Errorf("invalid app-server message: %w", err))
			return
		}
		if msg.Method == "" && len(msg.ID) > 0 {
			var id int64
			if err := json.Unmarshal(msg.ID, &id); err != nil {
				c.fail(fmt.Errorf("invalid app-server response ID: %w", err))
				return
			}
			var reply rpcReply
			if err := json.Unmarshal(raw, &reply); err != nil {
				c.fail(err)
				return
			}
			c.mu.Lock()
			ch := c.pending[id]
			delete(c.pending, id)
			c.mu.Unlock()
			if ch != nil {
				ch <- reply
			}
			continue
		}
		if msg.Method == "" {
			c.fail(fmt.Errorf("invalid app-server message without method or response ID"))
			return
		}
		if len(msg.ID) > 0 {
			c.onRequest(msg)
			continue
		}
		c.onNotification(msg)
	}
}

func (c *Client) fail(err error) {
	c.mu.Lock()
	if c.closedErr != nil {
		c.mu.Unlock()
		return
	}
	c.closedErr = err
	for _, ch := range c.pending {
		ch <- rpcReply{Err: err}
	}
	clear(c.pending)
	runs := make([]*turnRun, 0, len(c.runs))
	for _, run := range c.runs {
		runs = append(runs, run)
	}
	close(c.done)
	c.mu.Unlock()
	for _, run := range runs {
		select {
		case run.events <- Event{Kind: "disconnected", Err: err}:
		case <-run.done:
		default:
		}
	}
}

func (c *Client) onNotification(msg envelope) {
	var binding struct {
		ThreadID string `json:"threadId"`
		TurnID   string `json:"turnId"`
		Turn     struct {
			ID string `json:"id"`
		} `json:"turn"`
	}
	if err := json.Unmarshal(msg.Params, &binding); err != nil {
		if msg.Method == "turn/started" || msg.Method == "turn/completed" {
			c.fail(fmt.Errorf("invalid %s notification: %w", msg.Method, err))
		}
		return
	}
	if binding.TurnID == "" {
		binding.TurnID = binding.Turn.ID
	}
	if (msg.Method == "turn/started" || msg.Method == "turn/completed") && (binding.ThreadID == "" || binding.TurnID == "") {
		c.fail(fmt.Errorf("%s notification lacks thread or turn ID", msg.Method))
		return
	}
	c.mu.Lock()
	run := c.runs[binding.ThreadID]
	c.mu.Unlock()
	if run == nil {
		return
	}
	if known := run.getTurnID(); known != "" && binding.TurnID != "" && binding.TurnID != known {
		if msg.Method == "turn/started" || msg.Method == "turn/completed" {
			run.emit(Event{Kind: "protocol/error", Err: fmt.Errorf("%s turn ID differs from active turn", msg.Method)})
		}
		return
	}
	if msg.Method == "turn/started" && binding.TurnID != "" {
		run.setTurnID(binding.TurnID)
	}
	if msg.Method == "turn/completed" {
		run.markTerminal()
	}
	run.emit(Event{Kind: msg.Method, ThreadID: binding.ThreadID, TurnID: binding.TurnID, Raw: msg.Params})
}

// RoleResult is the minimal structured result; detailed disposition policy
// belongs to the later controller.
type RoleResult struct {
	Disposition string `json:"disposition"`
	Summary     string `json:"summary"`
}

// ParseRoleResult keeps provider completion distinct from a valid role verdict.
func ParseRoleResult(text string) (RoleResult, error) {
	var result RoleResult
	if strings.TrimSpace(text) == "" || json.Unmarshal([]byte(text), &result) != nil || strings.TrimSpace(result.Disposition) == "" || strings.TrimSpace(result.Summary) == "" {
		return RoleResult{}, ErrMalformedRoleResult
	}
	return result, nil
}
