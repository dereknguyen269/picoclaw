package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sipeed/picoclaw/pkg/logger"
)

// JSON-RPC types for MCP protocol
type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      *int64 `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
	Method  string          `json:"method,omitempty"` // For server notifications
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCP protocol types
type MCPToolInfo struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema,omitempty"`
}

type MCPToolsResult struct {
	Tools      []MCPToolInfo `json:"tools"`
	NextCursor string        `json:"nextCursor,omitempty"`
}

type MCPCallToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type MCPToolContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`     // base64 for image content
	MimeType string `json:"mimeType,omitempty"` // e.g. "image/png"
}

type MCPCallToolResult struct {
	Content []MCPToolContent `json:"content"`
	IsError bool             `json:"isError,omitempty"`
}

// NotifyHandler is called when the server sends a notification.
type NotifyHandler func(method string, params json.RawMessage)

// Client communicates with an MCP server over stdio (JSON-RPC).
type Client struct {
	serverName string
	command    string
	args       []string
	env        []string

	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Reader
	writeMu sync.Mutex

	nextID    atomic.Int64
	pending   map[int64]chan *jsonRPCResponse
	pendingMu sync.Mutex

	done     chan struct{}
	alive    atomic.Bool
	onNotify NotifyHandler

	// stderr capture
	stderrBuf *limitedBuffer
}

// limitedBuffer captures stderr up to a max size, ring-buffer style.
type limitedBuffer struct {
	mu   sync.Mutex
	data []byte
	max  int
}

func newLimitedBuffer(max int) *limitedBuffer {
	return &limitedBuffer{data: make([]byte, 0, max), max: max}
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = append(b.data, p...)
	if len(b.data) > b.max {
		b.data = b.data[len(b.data)-b.max:]
	}
	return len(p), nil
}

func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.data)
}

// NewClient starts an MCP server process and establishes JSON-RPC communication.
func NewClient(serverName, command string, args []string, env []string) (*Client, error) {
	c := &Client{
		serverName: serverName,
		command:    command,
		args:       args,
		env:        env,
		pending:    make(map[int64]chan *jsonRPCResponse),
		done:       make(chan struct{}),
		stderrBuf:  newLimitedBuffer(8192),
	}

	if err := c.startProcess(); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Client) startProcess() error {
	cmd := exec.Command(c.command, c.args...)
	cmd.Env = append(cmd.Environ(), c.env...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("mcp: stdin pipe: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("mcp: stdout pipe: %w", err)
	}

	// Capture stderr for debugging instead of discarding
	cmd.Stderr = c.stderrBuf

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("mcp: start %s: %w", c.command, err)
	}

	c.cmd = cmd
	c.stdin = stdin
	c.stdout = bufio.NewReaderSize(stdoutPipe, 1024*1024) // 1MB buffer for large responses
	c.done = make(chan struct{})
	c.pending = make(map[int64]chan *jsonRPCResponse)
	c.alive.Store(true)

	go c.readLoop()

	return nil
}

// IsAlive returns whether the server process is still running.
func (c *Client) IsAlive() bool {
	return c.alive.Load()
}

// Stderr returns captured stderr output for debugging.
func (c *Client) Stderr() string {
	return c.stderrBuf.String()
}

// SetNotifyHandler sets a callback for server-initiated notifications.
func (c *Client) SetNotifyHandler(h NotifyHandler) {
	c.onNotify = h
}
func (c *Client) readLoop() {
	defer func() {
		c.alive.Store(false)
		close(c.done)
		// Fail all pending requests
		c.pendingMu.Lock()
		for id, ch := range c.pending {
			ch <- &jsonRPCResponse{
				ID:    id,
				Error: &jsonRPCError{Code: -1, Message: "server closed"},
			}
			delete(c.pending, id)
		}
		c.pendingMu.Unlock()
	}()

	for {
		line, err := c.stdout.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				logger.ErrorCF("mcp", fmt.Sprintf("[%s] read error: %v", c.serverName, err), nil)
			}
			return
		}

		// Skip empty lines and non-JSON lines (some servers log to stdout)
		trimmed := strings.TrimSpace(string(line))
		if trimmed == "" || trimmed[0] != '{' {
			continue
		}

		var resp jsonRPCResponse
		if err := json.Unmarshal([]byte(trimmed), &resp); err != nil {
			continue
		}

		// Server notification (no ID, has method)
		if resp.Method != "" {
			if c.onNotify != nil {
				c.onNotify(resp.Method, resp.Result)
			}
			continue
		}

		// Route response to waiting caller
		c.pendingMu.Lock()
		ch, ok := c.pending[resp.ID]
		if ok {
			delete(c.pending, resp.ID)
		}
		c.pendingMu.Unlock()

		if ok {
			ch <- &resp
		}
	}
}

func (c *Client) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if !c.alive.Load() {
		return nil, fmt.Errorf("mcp: server %s is not running", c.serverName)
	}

	id := c.nextID.Add(1)

	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("mcp: marshal: %w", err)
	}
	data = append(data, '\n')

	ch := make(chan *jsonRPCResponse, 1)
	c.pendingMu.Lock()
	c.pending[id] = ch
	c.pendingMu.Unlock()

	c.writeMu.Lock()
	_, err = c.stdin.Write(data)
	c.writeMu.Unlock()
	if err != nil {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("mcp: write: %w", err)
	}

	select {
	case <-ctx.Done():
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, ctx.Err()
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("mcp: rpc error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	case <-c.done:
		return nil, fmt.Errorf("mcp: server %s closed", c.serverName)
	}
}

func (c *Client) notify(method string) {
	notif := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
	}
	data, _ := json.Marshal(notif)
	data = append(data, '\n')
	c.writeMu.Lock()
	c.stdin.Write(data)
	c.writeMu.Unlock()
}

// Initialize performs the MCP initialize handshake.
func (c *Client) Initialize(ctx context.Context) error {
	params := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "picoclaw",
			"version": "1.0.0",
		},
	}

	_, err := c.call(ctx, "initialize", params)
	if err != nil {
		return fmt.Errorf("mcp: initialize: %w", err)
	}

	c.notify("notifications/initialized")
	return nil
}

// ListTools calls tools/list with pagination support.
func (c *Client) ListTools(ctx context.Context) ([]MCPToolInfo, error) {
	var allTools []MCPToolInfo
	var cursor string

	for {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}

		result, err := c.call(ctx, "tools/list", params)
		if err != nil {
			return nil, err
		}

		var toolsResult MCPToolsResult
		if err := json.Unmarshal(result, &toolsResult); err != nil {
			return nil, fmt.Errorf("mcp: parse tools/list: %w", err)
		}

		allTools = append(allTools, toolsResult.Tools...)

		if toolsResult.NextCursor == "" {
			break
		}
		cursor = toolsResult.NextCursor
	}

	return allTools, nil
}

// CallTool invokes a tool on the MCP server with a per-call timeout.
func (c *Client) CallTool(ctx context.Context, name string, arguments map[string]any, timeout time.Duration) (string, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	params := MCPCallToolParams{
		Name:      name,
		Arguments: arguments,
	}

	result, err := c.call(ctx, "tools/call", params)
	if err != nil {
		return "", err
	}

	var callResult MCPCallToolResult
	if err := json.Unmarshal(result, &callResult); err != nil {
		return "", fmt.Errorf("mcp: parse tools/call: %w", err)
	}

	// Build combined output from all content blocks
	var sb strings.Builder
	for i, content := range callResult.Content {
		if i > 0 {
			sb.WriteString("\n")
		}
		switch content.Type {
		case "text":
			sb.WriteString(content.Text)
		case "image":
			// Include image metadata so the agent knows an image was returned
			mime := content.MimeType
			if mime == "" {
				mime = "image/unknown"
			}
			sb.WriteString(fmt.Sprintf("[image: %s, %d bytes base64]", mime, len(content.Data)))
		default:
			if content.Text != "" {
				sb.WriteString(content.Text)
			}
		}
	}

	combined := sb.String()
	if callResult.IsError {
		return "", fmt.Errorf("mcp tool error: %s", combined)
	}

	return combined, nil
}

// Reconnect restarts the server process and re-initializes.
func (c *Client) Reconnect(ctx context.Context) error {
	logger.InfoCF("mcp", fmt.Sprintf("[%s] Reconnecting...", c.serverName), nil)

	// Kill old process
	c.stdin.Close()
	c.cmd.Process.Kill()
	c.cmd.Wait()

	// Wait for readLoop to finish
	<-c.done

	// Start fresh
	if err := c.startProcess(); err != nil {
		return fmt.Errorf("reconnect start: %w", err)
	}

	if err := c.Initialize(ctx); err != nil {
		return fmt.Errorf("reconnect initialize: %w", err)
	}

	logger.InfoCF("mcp", fmt.Sprintf("[%s] Reconnected successfully", c.serverName), nil)
	return nil
}

// GracefulClose sends a shutdown request, waits briefly, then kills.
func (c *Client) GracefulClose(timeout time.Duration) error {
	if !c.alive.Load() {
		return nil
	}

	// Try graceful shutdown via closing stdin
	c.stdin.Close()

	// Wait for process to exit
	done := make(chan error, 1)
	go func() {
		done <- c.cmd.Wait()
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		logger.InfoCF("mcp", fmt.Sprintf("[%s] Graceful shutdown timed out, killing", c.serverName), nil)
		return c.cmd.Process.Kill()
	}
}

// Close shuts down the MCP server process (hard kill, backward compat).
func (c *Client) Close() error {
	return c.GracefulClose(5 * time.Second)
}
