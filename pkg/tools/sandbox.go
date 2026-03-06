package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// SandboxCreateTool creates an isolated sandbox workspace for development.
type SandboxCreateTool struct {
	baseDir string // e.g. ~/.picoclaw/workspace/sandbox
}

func NewSandboxCreateTool(workspace string) *SandboxCreateTool {
	return &SandboxCreateTool{
		baseDir: filepath.Join(workspace, "sandbox"),
	}
}

func (t *SandboxCreateTool) Name() string { return "sandbox_create" }

func (t *SandboxCreateTool) Description() string {
	return "Create an isolated sandbox workspace for coding/development. Supports templates: go, python, node, html, rust, or empty."
}

func (t *SandboxCreateTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the sandbox (alphanumeric, dashes, underscores)",
			},
			"template": map[string]interface{}{
				"type":        "string",
				"description": "Project template: go, python, node, html, rust, or empty (default: empty)",
				"enum":        []string{"empty", "go", "python", "node", "html", "rust"},
			},
		},
		"required": []string{"name"},
	}
}
func (t *SandboxCreateTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return "", fmt.Errorf("name is required")
	}

	// Validate name
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return "", fmt.Errorf("invalid sandbox name: only alphanumeric, dashes, underscores allowed")
		}
	}

	sandboxPath := filepath.Join(t.baseDir, name)

	if _, err := os.Stat(sandboxPath); err == nil {
		return "", fmt.Errorf("sandbox '%s' already exists at %s", name, sandboxPath)
	}

	if err := os.MkdirAll(sandboxPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create sandbox: %w", err)
	}

	template := "empty"
	if tmpl, ok := args["template"].(string); ok && tmpl != "" {
		template = tmpl
	}

	if err := scaffoldTemplate(sandboxPath, name, template); err != nil {
		os.RemoveAll(sandboxPath)
		return "", fmt.Errorf("failed to scaffold template: %w", err)
	}

	return fmt.Sprintf("Sandbox '%s' created at %s (template: %s)\n\nUse sandbox_exec to run commands inside it, or read/write files under this path.", name, sandboxPath, template), nil
}

func scaffoldTemplate(dir, name, template string) error {
	switch template {
	case "go":
		return scaffoldGo(dir, name)
	case "python":
		return scaffoldPython(dir, name)
	case "node":
		return scaffoldNode(dir, name)
	case "html":
		return scaffoldHTML(dir, name)
	case "rust":
		return scaffoldRust(dir, name)
	case "empty":
		// Just create a README
		return os.WriteFile(filepath.Join(dir, "README.md"), []byte(fmt.Sprintf("# %s\n\nSandbox workspace.\n", name)), 0644)
	default:
		return fmt.Errorf("unknown template: %s", template)
	}
}

func scaffoldGo(dir, name string) error {
	mod := fmt.Sprintf("module %s\n\ngo 1.24.0\n", name)
	main := `package main

import "fmt"

func main() {
	fmt.Println("Hello from sandbox!")
}
`
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "main.go"), []byte(main), 0644)
}

func scaffoldPython(dir, name string) error {
	main := `#!/usr/bin/env python3
"""${name} sandbox."""


def main():
    print("Hello from sandbox!")


if __name__ == "__main__":
    main()
`
	main = strings.ReplaceAll(main, "${name}", name)
	req := "# Add dependencies here\n"
	if err := os.WriteFile(filepath.Join(dir, "main.py"), []byte(main), 0644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte(req), 0644)
}

func scaffoldNode(dir, name string) error {
	pkg := fmt.Sprintf(`{
  "name": "%s",
  "version": "1.0.0",
  "type": "module",
  "scripts": {
    "start": "node index.js"
  }
}
`, name)
	index := `console.log("Hello from sandbox!");
`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "index.js"), []byte(index), 0644)
}

func scaffoldHTML(dir, name string) error {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>%s</title>
  <style>
    body { font-family: system-ui, sans-serif; max-width: 800px; margin: 2rem auto; padding: 0 1rem; }
  </style>
</head>
<body>
  <h1>%s</h1>
  <p>Hello from sandbox!</p>
  <script>
    console.log("sandbox ready");
  </script>
</body>
</html>
`, name, name)
	return os.WriteFile(filepath.Join(dir, "index.html"), []byte(html), 0644)
}

func scaffoldRust(dir, name string) error {
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		return err
	}
	cargo := fmt.Sprintf(`[package]
name = "%s"
version = "0.1.0"
edition = "2021"
`, name)
	main := `fn main() {
    println!("Hello from sandbox!");
}
`
	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(cargo), 0644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(srcDir, "main.rs"), []byte(main), 0644)
}

// SandboxListTool lists all sandbox workspaces.
type SandboxListTool struct {
	baseDir string
}

func NewSandboxListTool(workspace string) *SandboxListTool {
	return &SandboxListTool{baseDir: filepath.Join(workspace, "sandbox")}
}

func (t *SandboxListTool) Name() string { return "sandbox_list" }

func (t *SandboxListTool) Description() string {
	return "List all sandbox workspaces with their size and template info."
}

func (t *SandboxListTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *SandboxListTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	entries, err := os.ReadDir(t.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "No sandboxes found.", nil
		}
		return "", fmt.Errorf("failed to list sandboxes: %w", err)
	}

	if len(entries) == 0 {
		return "No sandboxes found.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Sandboxes (%d):\n\n", len(entries)))

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		path := filepath.Join(t.baseDir, name)
		template := detectTemplate(path)
		size := dirSize(path)

		sb.WriteString(fmt.Sprintf("  %s  [%s]  %s  (%s)\n", name, template, path, formatSize(size)))
	}

	return sb.String(), nil
}

func detectTemplate(dir string) string {
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		return "go"
	}
	if _, err := os.Stat(filepath.Join(dir, "main.py")); err == nil {
		return "python"
	}
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		return "node"
	}
	if _, err := os.Stat(filepath.Join(dir, "index.html")); err == nil {
		return "html"
	}
	if _, err := os.Stat(filepath.Join(dir, "Cargo.toml")); err == nil {
		return "rust"
	}
	return "empty"
}

func dirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

func formatSize(bytes int64) string {
	switch {
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// SandboxExecTool runs commands inside a sandbox, scoped to its directory.
// Supports background mode for long-running processes (servers, watchers).
type SandboxExecTool struct {
	baseDir   string
	timeout   time.Duration
	bgProcs   map[string]*bgProcess
	bgProcsMu sync.Mutex
}

type bgProcess struct {
	cmd     *exec.Cmd
	stdout  *syncBuffer
	stderr  *syncBuffer
	done    chan struct{}
	sandbox string
	command string
}

// syncBuffer is a thread-safe bytes.Buffer.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
	max int
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	// Ring-buffer: keep only last N bytes
	b.buf.Write(p)
	if b.buf.Len() > b.max {
		data := b.buf.Bytes()
		b.buf.Reset()
		b.buf.Write(data[len(data)-b.max:])
	}
	return len(p), nil
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func NewSandboxExecTool(workspace string) *SandboxExecTool {
	return &SandboxExecTool{
		baseDir: filepath.Join(workspace, "sandbox"),
		timeout: 30 * time.Second,
		bgProcs: make(map[string]*bgProcess),
	}
}

func (t *SandboxExecTool) Name() string { return "sandbox_exec" }

func (t *SandboxExecTool) Description() string {
	return `Execute a shell command inside a sandbox workspace.
- For short commands (build, test, ls): runs and returns output (30s timeout).
- For long-running commands (servers, watchers): set background=true to start in background, returns immediately with initial output.
- Use sandbox_exec with background_action="status" to check output, or "stop" to kill a background process.`
}

func (t *SandboxExecTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the sandbox",
			},
			"command": map[string]interface{}{
				"type":        "string",
				"description": "Shell command to execute",
			},
			"background": map[string]interface{}{
				"type":        "boolean",
				"description": "Run as background process (for servers, watchers). Returns immediately with initial output.",
			},
			"background_action": map[string]interface{}{
				"type":        "string",
				"description": "Action for background process: 'status' to get output, 'stop' to kill it. Requires name only.",
				"enum":        []string{"status", "stop"},
			},
			"timeout": map[string]interface{}{
				"type":        "integer",
				"description": "Timeout in seconds for foreground commands (default: 30, max: 300)",
			},
		},
		"required": []string{"name"},
	}
}

func (t *SandboxExecTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return "", fmt.Errorf("name is required")
	}

	sandboxPath := filepath.Join(t.baseDir, name)
	if _, err := os.Stat(sandboxPath); os.IsNotExist(err) {
		return "", fmt.Errorf("sandbox '%s' not found. Use sandbox_create first", name)
	}

	// Handle background process actions (status/stop)
	if action, ok := args["background_action"].(string); ok {
		return t.handleBgAction(name, action)
	}

	command, ok := args["command"].(string)
	if !ok || command == "" {
		return "", fmt.Errorf("command is required")
	}

	// Background mode
	if bg, ok := args["background"].(bool); ok && bg {
		return t.startBackground(name, sandboxPath, command)
	}

	// Foreground mode with timeout
	timeout := t.timeout
	if secs, ok := args["timeout"].(float64); ok && secs > 0 {
		timeout = time.Duration(secs) * time.Second
		if timeout > 300*time.Second {
			timeout = 300 * time.Second
		}
	}

	return t.runForeground(ctx, name, sandboxPath, command, timeout)
}

func (t *SandboxExecTool) runForeground(ctx context.Context, name, sandboxPath, command string, timeout time.Duration) (string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "sh", "-c", command)
	cmd.Dir = sandboxPath
	cmd.Env = append(os.Environ(), fmt.Sprintf("SANDBOX_DIR=%s", sandboxPath))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\nSTDERR:\n" + stderr.String()
	}

	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			return fmt.Sprintf("[sandbox:%s] %s\n\nCommand timed out after %v. If this is a server/watcher, use background=true.", name, command, timeout), nil
		}
		output += fmt.Sprintf("\nExit code: %v", err)
	}

	if output == "" {
		output = "(no output)"
	}

	return truncateOutput(fmt.Sprintf("[sandbox:%s] %s\n\n%s", name, command, output), 20000), nil
}

func (t *SandboxExecTool) startBackground(name, sandboxPath, command string) (string, error) {
	t.bgProcsMu.Lock()
	defer t.bgProcsMu.Unlock()

	// Stop existing bg process for this sandbox if any
	if existing, ok := t.bgProcs[name]; ok {
		select {
		case <-existing.done:
			// Already finished
		default:
			existing.cmd.Process.Kill()
			existing.cmd.Wait()
		}
		delete(t.bgProcs, name)
	}

	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = sandboxPath
	cmd.Env = append(os.Environ(), fmt.Sprintf("SANDBOX_DIR=%s", sandboxPath))

	stdoutBuf := &syncBuffer{max: 32768}
	stderrBuf := &syncBuffer{max: 32768}
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start background process: %w", err)
	}

	proc := &bgProcess{
		cmd:     cmd,
		stdout:  stdoutBuf,
		stderr:  stderrBuf,
		done:    make(chan struct{}),
		sandbox: name,
		command: command,
	}

	go func() {
		cmd.Wait()
		close(proc.done)
	}()

	t.bgProcs[name] = proc

	// Wait briefly to capture initial output (startup messages, errors)
	time.Sleep(2 * time.Second)

	// Check if it already exited (crash)
	select {
	case <-proc.done:
		output := stdoutBuf.String()
		if stderrBuf.String() != "" {
			output += "\nSTDERR:\n" + stderrBuf.String()
		}
		delete(t.bgProcs, name)
		return truncateOutput(fmt.Sprintf("[sandbox:%s] Background process exited immediately.\n\n%s", name, output), 20000), nil
	default:
		output := stdoutBuf.String()
		if stderrBuf.String() != "" {
			output += "\nSTDERR:\n" + stderrBuf.String()
		}
		if output == "" {
			output = "(no output yet)"
		}
		return truncateOutput(fmt.Sprintf("[sandbox:%s] Background process started (PID %d): %s\n\nInitial output:\n%s\n\nUse sandbox_exec with background_action='status' to check, or 'stop' to kill.", name, cmd.Process.Pid, command, output), 20000), nil
	}
}

func (t *SandboxExecTool) handleBgAction(name, action string) (string, error) {
	t.bgProcsMu.Lock()
	defer t.bgProcsMu.Unlock()

	proc, ok := t.bgProcs[name]
	if !ok {
		return fmt.Sprintf("No background process running in sandbox '%s'.", name), nil
	}

	switch action {
	case "status":
		output := proc.stdout.String()
		if proc.stderr.String() != "" {
			output += "\nSTDERR:\n" + proc.stderr.String()
		}
		if output == "" {
			output = "(no output)"
		}

		running := true
		select {
		case <-proc.done:
			running = false
		default:
		}

		status := "running"
		if !running {
			status = "exited"
			delete(t.bgProcs, name)
		}

		return truncateOutput(fmt.Sprintf("[sandbox:%s] Background process (%s): %s\nCommand: %s\n\n%s", name, status, proc.command, proc.command, output), 20000), nil

	case "stop":
		select {
		case <-proc.done:
			delete(t.bgProcs, name)
			return fmt.Sprintf("[sandbox:%s] Process already exited.", name), nil
		default:
			proc.cmd.Process.Kill()
			proc.cmd.Wait()
			output := proc.stdout.String()
			if proc.stderr.String() != "" {
				output += "\nSTDERR:\n" + proc.stderr.String()
			}
			delete(t.bgProcs, name)
			return truncateOutput(fmt.Sprintf("[sandbox:%s] Background process stopped.\n\nFinal output:\n%s", name, output), 20000), nil
		}

	default:
		return "", fmt.Errorf("unknown background_action: %s (use 'status' or 'stop')", action)
	}
}

func truncateOutput(output string, maxLen int) string {
	if len(output) > maxLen {
		return output[:maxLen] + fmt.Sprintf("\n... (truncated, %d more chars)", len(output)-maxLen)
	}
	return output
}

// SandboxDestroyTool removes a sandbox workspace.
type SandboxDestroyTool struct {
	baseDir string
}

func NewSandboxDestroyTool(workspace string) *SandboxDestroyTool {
	return &SandboxDestroyTool{baseDir: filepath.Join(workspace, "sandbox")}
}

func (t *SandboxDestroyTool) Name() string { return "sandbox_destroy" }

func (t *SandboxDestroyTool) Description() string {
	return "Permanently delete a sandbox workspace and all its files."
}

func (t *SandboxDestroyTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the sandbox to destroy",
			},
		},
		"required": []string{"name"},
	}
}

func (t *SandboxDestroyTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return "", fmt.Errorf("name is required")
	}

	// Safety: prevent path traversal
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid sandbox name")
	}

	sandboxPath := filepath.Join(t.baseDir, name)

	// Verify it's actually under baseDir
	absPath, err := filepath.Abs(sandboxPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}
	absBase, err := filepath.Abs(t.baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve base: %w", err)
	}
	if !strings.HasPrefix(absPath, absBase) {
		return "", fmt.Errorf("invalid sandbox path")
	}

	if _, err := os.Stat(sandboxPath); os.IsNotExist(err) {
		return "", fmt.Errorf("sandbox '%s' not found", name)
	}

	if err := os.RemoveAll(sandboxPath); err != nil {
		return "", fmt.Errorf("failed to destroy sandbox: %w", err)
	}

	return fmt.Sprintf("Sandbox '%s' destroyed.", name), nil
}
