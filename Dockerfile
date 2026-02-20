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

RUN apk add --no-cache ca-certificates tzdata git nodejs npm python3 py3-pip

# Install native build tools for better-sqlite3, install packages, then remove build tools
RUN apk add --no-cache --virtual .build-deps make g++ && \
    npm install -g @anthropic-ai/claude-code mcp-kb-server && \
    apk del .build-deps

COPY --from=builder /picoclaw /usr/local/bin/picoclaw

# Copy builtin skills
COPY skills/ /opt/picoclaw/skills/

# Copy Claude Code config (.claude directory) and CLAUDE.md
COPY .claude/ /opt/picoclaw/claude/
COPY CLAUDE.md /opt/picoclaw/CLAUDE.md

# Entrypoint seeds the volume on first run
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Default port for gateway
EXPOSE 18790

ENTRYPOINT ["/entrypoint.sh"]
CMD ["gateway"]
