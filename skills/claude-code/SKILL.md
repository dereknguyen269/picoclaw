---
name: claude-code
description: "Use Claude Code CLI as a coding agent. Includes setup, configuration, CLAUDE.md, hooks, permissions, and task delegation."
metadata: {"nanobot":{"emoji":"🤖","requires":{"bins":["claude"]}}}
---

# Claude Code Skill

Use the `claude` CLI to delegate complex coding tasks. Claude Code can read files, write code, run commands, and handle multi-step programming tasks autonomously.

## Installation

```bash
# Install via npm
npm install -g @anthropic-ai/claude-code

# Verify
claude --version

# First-time auth (opens browser for Anthropic login)
claude auth login

# Or use API key directly
export ANTHROPIC_API_KEY=sk-ant-xxxxx
```

## Quick Usage

Run a one-shot task (non-interactive, prints result):
```bash
claude -p "your task description" --no-input
```

Run with a specific model:
```bash
claude -p "fix the bug in main.go" --model claude-sonnet-4-20250514 --no-input
```

## Project Setup: CLAUDE.md

CLAUDE.md is the instruction file that Claude Code reads at startup. Place it in the project root to give Claude Code context about the project.

Create `CLAUDE.md` in the project root:
```bash
cat > /path/to/project/CLAUDE.md << 'EOF'
# Project: MyApp

## Overview
Go backend service with Telegram bot integration.

## Tech Stack
- Go 1.24
- PostgreSQL
- Redis

## Build & Test
- Build: `go build ./cmd/myapp`
- Test: `go test ./...`
- Lint: `golangci-lint run`

## Code Style
- Follow standard Go conventions
- Use meaningful variable names
- Add comments for exported functions
- Error messages should be lowercase, no punctuation

## Project Structure
- `cmd/` - Entry points
- `pkg/` - Library packages
- `internal/` - Private packages

## Important Rules
- Never commit secrets or API keys
- Always run tests before committing
- Use conventional commits (feat:, fix:, docs:, etc.)
EOF
```

CLAUDE.md can also exist in subdirectories for folder-specific instructions. Claude Code merges them:
- `/CLAUDE.md` — project-wide rules
- `/pkg/api/CLAUDE.md` — API-specific rules

## Settings & Permissions

Claude Code settings control which tools it can use without asking. Located at `.claude/settings.json` in the project:

```bash
mkdir -p /path/to/project/.claude
cat > /path/to/project/.claude/settings.json << 'EOF'
{
  "permissions": {
    "allow": [
      "Bash(go build*)",
      "Bash(go test*)",
      "Bash(go fmt*)",
      "Bash(git status)",
      "Bash(git diff*)",
      "Bash(git log*)",
      "Bash(git add*)",
      "Bash(git commit*)",
      "Bash(ls*)",
      "Bash(cat*)",
      "Bash(head*)",
      "Bash(tail*)",
      "Bash(grep*)",
      "Bash(find*)",
      "Read",
      "Write"
    ],
    "deny": [
      "Bash(rm -rf*)",
      "Bash(sudo*)",
      "Bash(curl*|*api_key*)",
      "Bash(git push*)"
    ]
  }
}
EOF
```

Global settings (apply to all projects) at `~/.claude/settings.json`:
```bash
mkdir -p ~/.claude
cat > ~/.claude/settings.json << 'EOF'
{
  "permissions": {
    "deny": [
      "Bash(rm -rf /)*",
      "Bash(sudo*)",
      "Bash(shutdown*)"
    ]
  }
}
EOF
```

## Hooks

Claude Code supports hooks that run automatically on events. Configure in `.claude/settings.json`:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "echo 'About to run a bash command'"
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Write",
        "hooks": [
          {
            "type": "command",
            "command": "go fmt $CLAUDE_FILE_PATH"
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "go test ./... 2>&1 | tail -5"
          }
        ]
      }
    ]
  }
}
```

Hook events:
- `PreToolUse` — before a tool runs (can block with non-zero exit + reason on stderr)
- `PostToolUse` — after a tool runs
- `Stop` — when Claude Code finishes a task
- `Notification` — when Claude Code wants to notify the user

Hook environment variables available:
- `$CLAUDE_FILE_PATH` — file being edited (for Write/Edit tools)
- `$CLAUDE_TOOL_NAME` — name of the tool being used
- `$CLAUDE_TOOL_INPUT` — JSON input to the tool

## Full Auto Mode

For tasks that need file writes and command execution without confirmation:
```bash
claude -p "your task" --no-input --allowedTools "Edit,Write,Bash"
```

## Piping Context

```bash
cat error.log | claude -p "Analyze this error log and suggest fixes" --no-input
```

```bash
git diff | claude -p "Review this diff and check for bugs" --no-input
```

## Picoclaw Integration Patterns

### Setup a project for Claude Code (run via exec tool):
```bash
# Create CLAUDE.md and settings in one go
mkdir -p /path/to/project/.claude
cat > /path/to/project/CLAUDE.md << 'EOF'
# Project instructions here
EOF
cat > /path/to/project/.claude/settings.json << 'EOF'
{"permissions":{"allow":["Read","Write","Bash(go *)","Bash(git status)","Bash(git diff*)"]}}
EOF
```

### Delegate a coding task:
```bash
claude -p "Read the codebase and add error handling to all HTTP endpoints in pkg/api/" --no-input --allowedTools "Read,Write,Edit,Bash"
```

### Code review with git diff:
```bash
git diff HEAD~1 | claude -p "Review this commit for bugs, security issues, and code quality" --no-input
```

### Fix failing tests:
```bash
claude -p "Run 'go test ./...' and fix all failing tests" --no-input --allowedTools "Read,Write,Edit,Bash"
```

### Multi-agent with tmux (parallel tasks):
```bash
SOCKET="${TMPDIR:-/tmp}/claude-agents.sock"
tmux -S "$SOCKET" new-session -d -s agent-1
tmux -S "$SOCKET" new-session -d -s agent-2

tmux -S "$SOCKET" send-keys -t agent-1 "cd /project && claude -p 'Add unit tests for pkg/auth/' --no-input --allowedTools 'Read,Write,Bash'" Enter
tmux -S "$SOCKET" send-keys -t agent-2 "cd /project && claude -p 'Add unit tests for pkg/config/' --no-input --allowedTools 'Read,Write,Bash'" Enter
```

## When to Use Claude Code vs Direct Tools

Use Claude Code when:
- The task requires multi-step reasoning about code
- You need to generate or refactor large amounts of code
- The task involves understanding complex codebases
- Bug fixing requires reading multiple files and understanding context

Use picoclaw's built-in tools directly when:
- Simple file reads/writes
- Running a single command
- The task is straightforward and doesn't need deep code reasoning

## Tips

- Always use `--no-input` for non-interactive execution from picoclaw
- Use `-p` flag to pass the prompt as a string
- For long tasks, use the `exec` tool with a longer timeout or tmux
- Output is returned as text — parse it or forward to user via `message` tool
- Claude Code has its own tool use (file read/write, bash) — it works independently
- Set `ANTHROPIC_API_KEY` env var for headless/server environments
