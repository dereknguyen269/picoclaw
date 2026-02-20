#!/bin/sh
# Seed workspace on first run (volume is empty)
WORKSPACE="/root/.picoclaw/workspace"
mkdir -p "$WORKSPACE/skills" "$WORKSPACE/memory" "$WORKSPACE/sandbox" "$WORKSPACE/sessions"

# Copy builtin skills if not present
if [ -d /opt/picoclaw/skills ] && [ ! -f "$WORKSPACE/skills/.seeded" ]; then
  cp -rn /opt/picoclaw/skills/* "$WORKSPACE/skills/" 2>/dev/null || true
  touch "$WORKSPACE/skills/.seeded"
fi

# Copy .claude config if not present
if [ -d /opt/picoclaw/claude ] && [ ! -d "$WORKSPACE/.claude" ]; then
  cp -r /opt/picoclaw/claude "$WORKSPACE/.claude"
fi

if [ -f /opt/picoclaw/CLAUDE.md ] && [ ! -f "$WORKSPACE/CLAUDE.md" ]; then
  cp /opt/picoclaw/CLAUDE.md "$WORKSPACE/CLAUDE.md"
fi

exec picoclaw "$@"
