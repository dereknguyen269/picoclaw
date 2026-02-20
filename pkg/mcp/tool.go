package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/sipeed/picoclaw/pkg/logger"
)

// MCPTool wraps a single MCP server tool as a picoclaw Tool.
// Supports auto-reconnect when the server process dies.
type MCPTool struct {
	client      *Client
	serverName  string
	toolName    string
	description string
	inputSchema map[string]any
	callTimeout time.Duration
}

func NewMCPTool(client *Client, serverName string, info MCPToolInfo, callTimeout time.Duration) *MCPTool {
	desc := info.Description
	if desc == "" {
		desc = fmt.Sprintf("MCP tool from %s", serverName)
	}

	schema := info.InputSchema
	if schema == nil {
		schema = map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
	}

	if callTimeout <= 0 {
		callTimeout = 60 * time.Second
	}

	return &MCPTool{
		client:      client,
		serverName:  serverName,
		toolName:    info.Name,
		description: desc,
		inputSchema: schema,
		callTimeout: callTimeout,
	}
}

func (t *MCPTool) Name() string {
	return fmt.Sprintf("mcp_%s_%s", t.serverName, t.toolName)
}

func (t *MCPTool) Description() string {
	return fmt.Sprintf("[MCP:%s] %s", t.serverName, t.description)
}

func (t *MCPTool) Parameters() map[string]any {
	return t.inputSchema
}

func (t *MCPTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	// Auto-reconnect if server died
	if !t.client.IsAlive() {
		logger.InfoCF("mcp", fmt.Sprintf("[%s] Server dead, attempting reconnect for tool %s", t.serverName, t.toolName), nil)
		reconnCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := t.client.Reconnect(reconnCtx); err != nil {
			stderr := t.client.Stderr()
			if stderr != "" {
				logger.ErrorCF("mcp", fmt.Sprintf("[%s] stderr: %s", t.serverName, stderr), nil)
			}
			return "", fmt.Errorf("MCP server %s is down and reconnect failed: %v", t.serverName, err)
		}
	}

	result, err := t.client.CallTool(ctx, t.toolName, args, t.callTimeout)
	if err != nil {
		// If it failed because server died mid-call, try one reconnect
		if !t.client.IsAlive() {
			logger.InfoCF("mcp", fmt.Sprintf("[%s] Server died during call, retrying %s", t.serverName, t.toolName), nil)
			reconnCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			if reconnErr := t.client.Reconnect(reconnCtx); reconnErr != nil {
				return "", fmt.Errorf("MCP server %s crashed: %v (original: %v)", t.serverName, reconnErr, err)
			}
			return t.client.CallTool(ctx, t.toolName, args, t.callTimeout)
		}
		return "", err
	}

	return result, nil
}
