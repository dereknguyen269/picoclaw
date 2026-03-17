package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/sipeed/picoclaw/pkg/logger"
)

type ClaudeCodeTool struct {
	workingDir    string
	timeout       time.Duration
	anthropicKey  string
	anthropicBase string
}

func NewClaudeCodeTool(workingDir string) *ClaudeCodeTool {
	return &ClaudeCodeTool{
		workingDir: workingDir,
		timeout:    10 * time.Minute, // Coding tasks can take longer
	}
}

// SetAnthropicKey sets the API key for Claude Code authentication
func (t *ClaudeCodeTool) SetAnthropicKey(key string) {
	t.anthropicKey = key
}

// SetAnthropicBase sets the custom API base URL for Claude Code
func (t *ClaudeCodeTool) SetAnthropicBase(base string) {
	t.anthropicBase = base
}

func (t *ClaudeCodeTool) Name() string {
	return "claude_code"
}

func (t *ClaudeCodeTool) Description() string {
	return "Execute coding tasks using Claude Code CLI. Use this for complex coding tasks like implementing features, fixing bugs, refactoring code, or analyzing codebases. Provide a clear description of what needs to be coded."
}

func (t *ClaudeCodeTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"task": map[string]interface{}{
				"type":        "string",
				"description": "The coding task or prompt to send to Claude Code. Be specific about what needs to be implemented, fixed, or analyzed.",
			},
			"working_dir": map[string]interface{}{
				"type":        "string",
				"description": "Optional working directory where Claude Code should operate. Defaults to the agent's workspace.",
			},
		},
		"required": []string{"task"},
	}
}

func (t *ClaudeCodeTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	task, ok := args["task"].(string)
	if !ok || task == "" {
		return "", fmt.Errorf("task is required and must be a non-empty string")
	}

	logger.InfoCF("claude_code", "Starting Claude Code execution",
		map[string]interface{}{
			"task_preview": truncateString(task, 100),
		})

	// Determine working directory
	cwd := t.workingDir
	if wd, ok := args["working_dir"].(string); ok && wd != "" {
		// Resolve to absolute path
		absWd, err := filepath.Abs(wd)
		if err != nil {
			return "", fmt.Errorf("invalid working directory: %w", err)
		}
		cwd = absWd
	}

	// Ensure working directory exists
	if cwd != "" {
		if _, err := os.Stat(cwd); os.IsNotExist(err) {
			return "", fmt.Errorf("working directory does not exist: %s", cwd)
		}
	}

	// Check if claude CLI is available
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		logger.ErrorCF("claude_code", "Claude CLI not found", nil)
		return "", fmt.Errorf("claude CLI not found. Please install Claude Code from https://claude.com/code")
	}

	logger.InfoCF("claude_code", "Executing Claude Code",
		map[string]interface{}{
			"claude_path":  claudePath,
			"working_dir":  cwd,
			"has_api_key":  t.anthropicKey != "",
			"has_base_url": t.anthropicBase != "",
		})

	// Create context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	// Execute claude command with the task
	// Use -p (print mode) for non-interactive execution
	// Claude Code will automatically read .claude/ config from the working directory
	cmd := exec.CommandContext(cmdCtx, claudePath, "-p", task)
	cmd.Dir = cwd

	// Set environment with Anthropic API key and base URL if configured
	cmd.Env = os.Environ()
	if t.anthropicKey != "" {
		cmd.Env = append(cmd.Env, "ANTHROPIC_API_KEY="+t.anthropicKey)
	}
	if t.anthropicBase != "" {
		cmd.Env = append(cmd.Env, "ANTHROPIC_BASE_URL="+t.anthropicBase)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute the command
	err = cmd.Run()

	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\n\nSTDERR:\n" + stderr.String()
	}

	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			logger.ErrorCF("claude_code", "Execution timed out", map[string]interface{}{
				"timeout": t.timeout.String(),
			})
			return "", fmt.Errorf("claude code execution timed out after %v", t.timeout)
		}
		logger.ErrorCF("claude_code", "Execution failed", map[string]interface{}{
			"error":  err.Error(),
			"stderr": stderr.String(),
		})
		return "", fmt.Errorf("claude code execution failed: %w\n\nOutput:\n%s", err, output)
	}

	logger.InfoCF("claude_code", "Execution completed",
		map[string]interface{}{
			"output_length": len(output),
		})

	if output == "" {
		output = "Claude Code completed the task successfully (no output)"
	}

	// Truncate if too long
	maxLen := 50000
	if len(output) > maxLen {
		output = output[:maxLen] + fmt.Sprintf("\n\n... (truncated, %d more chars)", len(output)-maxLen)
	}

	return output, nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func (t *ClaudeCodeTool) SetTimeout(timeout time.Duration) {
	t.timeout = timeout
}
