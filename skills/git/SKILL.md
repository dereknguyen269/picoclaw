---
name: git
description: "Git workflow for staging, committing, branching, and pushing code to remote repositories. Covers common patterns for GitHub, GitLab, and other remotes."
metadata: {"nanobot":{"emoji":"📦","requires":{"bins":["git"]}}}
---

# Git Skill

Use `git` for version control operations. Always check the current state before making changes.

## Check Status

```bash
git status
git log --oneline -10
git branch -a
git remote -v
```

## Stage and Commit

Stage specific files:
```bash
git add path/to/file1 path/to/file2
```

Stage all changes:
```bash
git add -A
```

Commit with a message:
```bash
git commit -m "feat: add new feature"
```

Stage and commit in one step (tracked files only):
```bash
git commit -am "fix: resolve bug in handler"
```

## Commit Message Convention

Follow Conventional Commits:
- `feat:` new feature
- `fix:` bug fix
- `docs:` documentation
- `refactor:` code refactoring
- `chore:` maintenance tasks
- `test:` adding tests

## Push to Remote

Push current branch:
```bash
git push origin HEAD
```

Push and set upstream for a new branch:
```bash
git push -u origin HEAD
```

Force push (after rebase, use with caution):
```bash
git push --force-with-lease origin HEAD
```

## Branching

Create and switch to a new branch:
```bash
git checkout -b feature/my-feature
```

Switch to existing branch:
```bash
git checkout main
```

Delete a branch after merge:
```bash
git branch -d feature/my-feature
```

## Pull and Sync

Pull latest changes:
```bash
git pull origin main
```

Rebase current branch on main:
```bash
git pull --rebase origin main
```

## Diff and Review

View unstaged changes:
```bash
git diff
```

View staged changes:
```bash
git diff --cached
```

View changes in last commit:
```bash
git show --stat HEAD
```

## Stash

Save work in progress:
```bash
git stash push -m "wip: description"
```

Restore stashed work:
```bash
git stash pop
```

## Undo

Unstage a file:
```bash
git restore --staged path/to/file
```

Discard local changes to a file:
```bash
git restore path/to/file
```

Amend last commit (before push):
```bash
git commit --amend --no-edit
```

## GitHub: Create PR After Push

After pushing a branch, create a PR using `gh` (requires github skill):
```bash
git push -u origin HEAD
gh pr create --title "feat: description" --body "Details here"
```

## Common Workflow

1. Check status: `git status`
2. Stage changes: `git add -A`
3. Commit: `git commit -m "feat: description"`
4. Push: `git push origin HEAD`

## Setup: Connect to a Repository

### Initialize and connect to a remote repo:
```bash
cd /path/to/project
git init
git remote add origin https://github.com/user/repo.git
git add -A
git commit -m "init: initial commit"
git branch -M main
git push -u origin main
```

### Clone an existing repo:
```bash
git clone https://github.com/user/repo.git
```

### Authentication

For HTTPS (recommended for servers/containers), use a Personal Access Token (PAT):
```bash
# Configure git to store credentials
git config --global credential.helper store

# Set credentials (token as password)
git config --global user.name "Your Name"
git config --global user.email "your@email.com"

# Use token in URL (for automation / CI / Docker)
git remote set-url origin https://<TOKEN>@github.com/user/repo.git
```

For SSH:
```bash
# Generate key (if not exists)
ssh-keygen -t ed25519 -C "your@email.com"

# Show public key — add this to GitHub > Settings > SSH Keys
cat ~/.ssh/id_ed25519.pub

# Test connection
ssh -T git@github.com

# Use SSH remote URL
git remote set-url origin git@github.com:user/repo.git
```

### Switch remote URL (HTTPS ↔ SSH):
```bash
# Check current remote
git remote -v

# Switch to SSH
git remote set-url origin git@github.com:user/repo.git

# Switch to HTTPS
git remote set-url origin https://github.com/user/repo.git
```

### Docker / Fly.io Setup

For containers that need git push access, set the token as an env var and configure at startup:
```bash
# Set as Fly secret
fly secrets set GIT_TOKEN=ghp_xxxxxxxxxxxx GIT_REPO=https://github.com/user/repo.git

# In your entrypoint or agent, configure git:
git config --global user.name "PicoClaw Bot"
git config --global user.email "bot@example.com"
git remote set-url origin https://${GIT_TOKEN}@github.com/user/repo.git
```
