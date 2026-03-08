# PicoClaw Fly.io Deployment Guide

## Prerequisites

- [flyctl](https://fly.io/docs/flyctl/install/) CLI installed
- Fly.io account authenticated (`fly auth login`)
- PicoClaw app created (`fly apps create picoclaw`)

## Architecture

```
┌─────────────────────────────────────┐
│  Fly.io Machine (shared-cpu-1x)     │
│                                     │
│  /usr/local/bin/picoclaw            │
│                                     │
│  /root/.picoclaw/  ← Persistent Vol │
│    ├── config.json                  │
│    ├── auth.json   ← OAuth tokens   │
│    ├── workspace/                   │
│    │   ├── skills/                  │
│    │   ├── memory/                  │
│    │   ├── sessions/                │
│    │   └── sandbox/                 │
│    └── sessions.db                  │
└─────────────────────────────────────┘
```

The persistent volume at `/root/.picoclaw` survives deploys and restarts.

## Step 1: Initial Deploy

```bash
fly deploy
```

This builds the Docker image and deploys it. On first run, `entrypoint.sh` seeds the workspace with builtin skills.

## Step 2: Set Configuration

### Option A: Config via environment variable (recommended)

```bash
fly secrets set PICOCLAW_CONFIG_JSON="$(cat config.json)" --app picoclaw
```

This takes precedence over any `config.json` file on the volume.

### Option B: Copy config.json to the volume

```bash
cat config.json | fly ssh console --app picoclaw -C "cat > /root/.picoclaw/config.json"
```

## Step 3: Set Auth Credentials (for OAuth providers like Antigravity)

Antigravity (Google Cloud Code Assist) uses OAuth tokens stored in `auth.json`. Since you can't run a browser-based OAuth flow on the server, authenticate locally first, then copy the credentials.

### 3.1 Authenticate locally

```bash
picoclaw auth login --provider antigravity
```

This opens your browser, completes Google OAuth, and saves tokens to `~/.picoclaw/auth.json`.

### 3.2 Copy auth.json to Fly

```bash
cat ~/.picoclaw/auth.json | fly ssh console --app picoclaw -C "cat > /root/.picoclaw/auth.json"
```

### 3.3 Verify on the server

```bash
fly ssh console --app picoclaw -C "cat /root/.picoclaw/auth.json"
```

> **Note:** OAuth tokens expire. The refresh token handles auto-renewal, but if it ever becomes invalid, repeat steps 3.1 and 3.2.

## Step 4: Set Other Secrets

For API-key-based providers, you can set them as Fly secrets:

```bash
# Individual provider keys
fly secrets set PICOCLAW_PROVIDERS_DEEPSEEK_API_KEY=sk-xxx --app picoclaw

# Telegram bot token
fly secrets set PICOCLAW_CHANNELS_TELEGRAM_TOKEN=your-bot-token --app picoclaw
```

Or include them in `config.json` / `PICOCLAW_CONFIG_JSON` directly.

## Step 5: Restart

```bash
fly apps restart picoclaw
```

## Updating Config After Deploy

When you change `config.json` or `auth.json` locally:

```bash
# Update config
fly secrets set PICOCLAW_CONFIG_JSON="$(cat config.json)" --app picoclaw

# Update auth (if re-authenticated)
cat ~/.picoclaw/auth.json | fly ssh console --app picoclaw -C "cat > /root/.picoclaw/auth.json"

# Restart to apply
fly apps restart picoclaw
```

## Monitoring

```bash
# View logs
fly logs --app picoclaw

# SSH into the machine
fly ssh console --app picoclaw

# Check health
curl https://picoclaw.fly.dev/health
```

## Troubleshooting

### Telegram 409 Conflict
```
terminated by other getUpdates request; make sure that only one bot instance is running
```
Only one instance can poll Telegram per bot token. Stop any local picoclaw process before deploying, or disable Telegram locally.

### Antigravity auth expired
```
antigravity auth: token refresh failed
```
Re-run `picoclaw auth login --provider antigravity` locally, then re-copy `auth.json` to Fly.

### Machine keeps stopping
Check `fly.toml` — `auto_stop_machines = 'stop'` means Fly stops the machine when idle. Set `min_machines_running = 1` to keep it alive (already configured).
