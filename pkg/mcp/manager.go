package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/tools"
)

// ServerConfig defines an MCP server to connect to.
type ServerConfig struct {
	Command     string            `json:"command"`
	Args        []string          `json:"args"`
	Env         map[string]string `json:"env,omitempty"`
	Disabled    bool              `json:"disabled,omitempty"`
	CallTimeout int               `json:"call_timeout,omitempty"` // per-tool call timeout in seconds
}

// Manager manages multiple MCP server connections and their tools.
type Manager struct {
	mu       sync.RWMutex
	clients  map[string]*Client
	configs  map[string]ServerConfig
	registry *tools.ToolRegistry // reference for dynamic tool refresh
}

// NewManager creates a new MCP manager.
func NewManager() *Manager {
	return &Manager{
		clients: make(map[string]*Client),
		configs: make(map[string]ServerConfig),
	}
}

// SetRegistry sets the tool registry for dynamic tool updates on notifications.
func (m *Manager) SetRegistry(r *tools.ToolRegistry) {
	m.registry = r
}

// ConnectAll starts all configured MCP servers and discovers their tools.
// Returns the tools to register, skipping servers that fail to connect.
func (m *Manager) ConnectAll(servers map[string]ServerConfig) []tools.Tool {
	var allTools []tools.Tool

	for name, cfg := range servers {
		if cfg.Disabled {
			logger.InfoCF("mcp", fmt.Sprintf("Skipping disabled MCP server: %s", name), nil)
			continue
		}

		m.configs[name] = cfg

		mcpTools, err := m.connectServer(name, cfg)
		if err != nil {
			logger.ErrorCF("mcp", fmt.Sprintf("Failed to connect MCP server %s: %v", name, err), nil)
			continue
		}

		allTools = append(allTools, mcpTools...)
		logger.InfoCF("mcp", fmt.Sprintf("Connected MCP server %s: %d tools", name, len(mcpTools)), nil)
	}

	return allTools
}

func (m *Manager) connectServer(name string, cfg ServerConfig) ([]tools.Tool, error) {
	// Build env slice
	var env []string
	for k, v := range cfg.Env {
		expanded := os.ExpandEnv(v)
		env = append(env, fmt.Sprintf("%s=%s", k, expanded))
	}

	client, err := NewClient(name, cfg.Command, cfg.Args, env)
	if err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}

	// Set up notification handler for tools/list_changed
	safeName := sanitizeName(name)
	client.SetNotifyHandler(func(method string, params json.RawMessage) {
		logger.InfoCF("mcp", fmt.Sprintf("[%s] Notification: %s", name, method), nil)
		if method == "notifications/tools/list_changed" {
			m.refreshTools(name, safeName, client, cfg)
		}
	})

	// Initialize with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Initialize(ctx); err != nil {
		client.Close()
		return nil, fmt.Errorf("initialize: %w", err)
	}

	// Discover tools (with pagination)
	toolInfos, err := client.ListTools(ctx)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("list tools: %w", err)
	}

	m.mu.Lock()
	m.clients[name] = client
	m.mu.Unlock()

	callTimeout := time.Duration(cfg.CallTimeout) * time.Second
	if callTimeout <= 0 {
		callTimeout = 60 * time.Second
	}

	var mcpTools []tools.Tool
	for _, info := range toolInfos {
		tool := NewMCPTool(client, safeName, info, callTimeout)
		mcpTools = append(mcpTools, tool)
	}

	return mcpTools, nil
}

// refreshTools re-discovers tools from a server after a tools/list_changed notification.
func (m *Manager) refreshTools(name, safeName string, client *Client, cfg ServerConfig) {
	if m.registry == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	toolInfos, err := client.ListTools(ctx)
	if err != nil {
		logger.ErrorCF("mcp", fmt.Sprintf("[%s] Failed to refresh tools: %v", name, err), nil)
		return
	}

	callTimeout := time.Duration(cfg.CallTimeout) * time.Second
	if callTimeout <= 0 {
		callTimeout = 60 * time.Second
	}

	// Register new/updated tools (overwrites existing by name)
	for _, info := range toolInfos {
		tool := NewMCPTool(client, safeName, info, callTimeout)
		m.registry.Register(tool)
	}

	logger.InfoCF("mcp", fmt.Sprintf("[%s] Refreshed tools: %d available", name, len(toolInfos)), nil)
}

// ServerStatus returns status info for all managed servers.
func (m *Manager) ServerStatus() map[string]map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make(map[string]map[string]any)
	for name, client := range m.clients {
		s := map[string]any{
			"alive": client.IsAlive(),
		}
		if !client.IsAlive() {
			stderr := client.Stderr()
			if stderr != "" {
				// Truncate stderr for status display
				if len(stderr) > 500 {
					stderr = stderr[len(stderr)-500:]
				}
				s["last_stderr"] = stderr
			}
		}
		status[name] = s
	}
	return status
}

// Close shuts down all MCP server connections gracefully.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, client := range m.clients {
		logger.InfoCF("mcp", fmt.Sprintf("Closing MCP server: %s", name), nil)
		client.GracefulClose(5 * time.Second)
	}
}

// sanitizeName makes a server name safe for use in tool names.
func sanitizeName(name string) string {
	replacer := strings.NewReplacer("-", "_", ".", "_", " ", "_")
	return replacer.Replace(name)
}
