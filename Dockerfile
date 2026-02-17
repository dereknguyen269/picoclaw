# Build stage
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o /picoclaw ./cmd/picoclaw

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata git nodejs npm

# Install Claude Code CLI
RUN npm install -g @anthropic-ai/claude-code

COPY --from=builder /picoclaw /usr/local/bin/picoclaw

# Copy builtin skills
COPY skills/ /opt/picoclaw/skills/

# Copy Claude Code config (.claude directory) and CLAUDE.md
COPY .claude/ /root/.picoclaw/workspace/.claude/
COPY CLAUDE.md /root/.picoclaw/workspace/CLAUDE.md

# Workspace and config directory
RUN mkdir -p /root/.picoclaw/workspace/skills /root/.picoclaw/workspace/memory

# Default port for gateway
EXPOSE 18790

ENTRYPOINT ["picoclaw"]
CMD ["gateway"]
