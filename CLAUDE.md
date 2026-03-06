# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 🚀 Quick Start Commands

### Building and Running
```bash
# Build for current platform
make build

# Build for all platforms (Linux: amd64, arm, arm64, loong64, riscv64; Windows: amd64; Darwin: arm64)
make build-all

# Build with WhatsApp native support (larger binary)
make build-whatsapp-native

# Build for Raspberry Pi Zero 2 W (both 32-bit and 64-bit)
make build-pi-zero

# Build Lambda deployment package
make build-lambda

# Build and install to ~/.local/bin
make install

# Run tests
make test

# Format code
make fmt

# Run linters
make lint

# Run vet, fmt, and verify dependencies
make check

# Clean build artifacts
make clean
```

### Development Workflow
```bash
# Run specific test file
go test ./pkg/agent -v

# Run test with coverage
go test -cover ./pkg/providers

# Run with verbose logging
picoclaw -v gateway

# Run agent with message
picoclaw agent -m "Hello, what's the weather like?"

# Interactive mode
picoclaw agent

# Run with specific args
make run ARGS="-v gateway"

# Test MCP tools in Docker
make docker-test

# Run in Docker (Alpine-based)
make docker-run

# Run in Docker with full features (Node.js)
make docker-run-full
```

## 🏗️ Architecture Overview

### Core Components

**Agent System (`pkg/agent/`)**
- Message processing loop with tool execution and safety controls
- Session management and context building with memory persistence
- Tool iteration limits to prevent infinite loops
- Subagent spawning for complex task decomposition
- Contextual tool execution with message context passing

**Provider System (`pkg/providers/`)**
- HTTP-based LLM providers with generic API support
- Fallback provider system for multi-provider resilience with automatic failover
- OAuth and token-based authentication support
- Auto-detection of providers based on model names
- Error classification for intelligent fallback decisions

**Tool System (`pkg/tools/`)**
- Tool registry and execution framework with deterministic iteration
- Built-in tools: file operations, web search, shell execution, cron jobs, MCP tools
- Contextual tools that can receive message context
- Async tool support with callback mechanisms
- Subagent spawning for complex tool workflows

**Channel System (`pkg/channels/`)**
- Multi-platform messaging support: Telegram, Discord, Feishu, WhatsApp, Slack, OneBot, QQ, WeChat Work, Webchat, and more
- Event-driven communication via message bus
- Message formatting and media handling with automatic transcription
- Thinking indicators and response streaming
- Permission-based access control with allowlists
- Group trigger configuration (mentions, prefixes)

**Configuration Management (`pkg/config/`)**
- JSON-based configuration with environment variable override support
- Multi-provider support with fallback configurations
- Channel-specific configurations with access controls
- Workspace and tool configurations
- Migration support from OpenClaw

### Data Flow

1. **Message Reception**: Channels receive messages from various platforms via event-driven bus
2. **Agent Processing**: Messages are processed through the agent loop with context building
3. **Tool Execution**: Agent invokes tools as needed for task completion with safety limits
4. **LLM Provider**: Primary provider is used with intelligent fallbacks on failure
5. **Response Generation**: Final response is sent back through the originating channel

### Key Interfaces

- `LLMProvider`: Core interface for all LLM providers with stateful connection support
- `ToolDefinition`: Interface for tools that can be executed by the agent
- `MessageBus`: Event-driven communication between components
- `ToolRegistry`: Manages registration and discovery of tools with deterministic iteration
- `Channel`: Interface for multi-channel messaging support with message length providers

## 📁 File Organization

```
cmd/
├── picoclaw/      # Main CLI application with Cobra subcommands
├── picoclaw-launcher/        # Web-based launcher
├── picoclaw-launcher-tui/    # Terminal-based launcher with TUI
└── lambda/                   # AWS Lambda serverless handler

pkg/
├── agent/         # Core AI agent logic with loop processing
├── providers/     # LLM provider implementations with fallback system
├── tools/         # Tool registry and execution framework
├── channels/      # Multi-channel messaging support
├── config/        # Configuration management
├── bus/           # Message bus for inter-component communication
├── auth/          # OAuth authentication and token management
└── skills/        # Skill system for extensibility

docker/            # Container deployment configuration
skills/            # Builtin skills directory
docs/              # Documentation and design files
```

## 🔧 Configuration Structure

### Primary Config File
`~/.picoclaw/config.json` contains:
- **Agents**: Model selection, workspace paths, context window settings, memory configurations
- **Channels**: Platform-specific configurations with access controls and permission lists
- **Providers**: Multiple LLM provider support with API keys, endpoints, and fallback configurations
- **Tools**: Web search and other tool configurations with rate limiting
- **Gateway**: HTTP server settings for the messaging gateway with CORS and security settings

### Environment Variables
- `PICOCLAW_CONFIG_JSON`: Full config as JSON string (for containers)
- `INSTALL_PREFIX`: Installation prefix (default: ~/.local)
- `WORKSPACE_DIR`: Workspace directory (default: ~/.picoclaw/workspace)
- Platform-specific variables for build targets (GOOS, GOARCH)
- API keys and credentials for various providers

## 🎯 Development Patterns

### Provider Implementation
New LLM providers should implement the `LLMProvider` interface and register themselves in the provider factory with proper error handling, fallback classification, and connection management.

### Tool Development
Tools extend the `ToolDefinition` interface and are registered in the tool registry. Consider using contextual tools for access to message context, async tools for long-running operations, and subagent spawning for complex workflows.

### Channel Integration
New channels implement the channel interface and register message handlers for different platform events with proper permission checking, message formatting, and media handling support.

### Skill System
Skills are self-contained directories with a `SKILL.md` file describing functionality. They can add new tools, modify agent behavior, or extend the system with custom logic.

## 🧪 Testing Strategy

### Unit Tests
- Focus on individual components in `pkg/` directories with table-driven tests
- Mock external dependencies (HTTP clients, file systems, LLM responses)
- Test error cases and edge conditions for robustness

### Integration Tests
- Test end-to-end agent workflows with real LLM providers when possible
- Verify multi-provider fallback behavior and error recovery
- Test channel message processing and response formatting

### Testing Commands
```bash
# Test specific package
go test ./pkg/providers -v

# Test with race detector
go test -race ./pkg/...

# Benchmark performance
go test -bench=. ./pkg/agent

# Run all tests
make test

# Run with coverage
go test -cover ./pkg/...
```

## 🚨 Common Gotchas

1. **Provider Fallbacks**: Ensure fallback providers are properly configured in config.json with correct API keys and endpoints
2. **API Keys**: Never commit API keys - use environment variables or config files. Use the `PICOCLAW_CONFIG_JSON` env var for containers
3. **Memory Management**: Monitor memory usage, especially with long conversations and multiple providers
4. **Channel Permissions**: Configure `allow_from` lists to restrict access and prevent unauthorized usage
5. **Cross-Platform Builds**: Use proper GOOS/GOARCH settings for different targets, especially for ARM variants
6. **Tool Safety**: Implement proper iteration limits and timeouts to prevent infinite loops and resource exhaustion
7. **Configuration Validation**: Validate JSON configuration syntax and ensure all required fields are present

## 🔍 Debugging Tips

### Logging
- Use structured logging with context via `pkg/logger`
- Enable verbose mode with `-v` flag for detailed output
- Check logs in workspace directory for runtime issues
- Use `PICOCLAW_LOG_LEVEL` environment variable to control log verbosity

### Performance
- Profile memory usage during long conversations with `pprof`
- Monitor provider response times and identify bottlenecks
- Check tool execution durations and optimize slow tools
- Use `go test -bench=. ./pkg/agent` for performance regression testing

### Configuration Issues
- Validate JSON configuration syntax with online validators
- Check API key validity for providers before deployment
- Verify channel token permissions and webhook URLs
- Test configuration changes in development environment first

## 📚 Key Files to Understand

- `pkg/agent/loop.go`: Core message processing logic with tool execution
- `pkg/providers/http_provider.go`: Generic HTTP provider implementation
- `pkg/providers/fallback_provider.go`: Multi-provider fallback system with error classification
- `pkg/tools/registry.go`: Tool registration and discovery with deterministic iteration
- `cmd/picoclaw/main.go`: CLI command handling with Cobra
- `config.example.json`: Complete configuration template with all options
- `Makefile`: Comprehensive build system with multiple targets and platforms
- `pkg/channels/base.go`: Base channel implementation with common functionality
- `pkg/bus/bus.go`: Message bus for inter-component communication

## 💡 Additional Development Commands

### Advanced Build Options
```bash
# Generate code (run before building)
make generate

# Build specific platforms
make build-linux-arm
make build-linux-arm64

# Install with skills
make install-skills

# Clean Docker images and volumes
make docker-clean
```

### Docker Development
```bash
# Build minimal Docker image
make docker-build

# Build full-featured Docker image with Node.js
make docker-build-full

# Run tests in Docker
make docker-test

# Run gateway in Docker
make docker-run
make docker-run-full
```

### Code Quality
```bash
# Run all linting tools
make check

# Fix linting issues automatically
make fix

# Update dependencies
make update-deps

# Verify dependencies
make deps
```

This repository represents an ultra-lightweight AI assistant designed for minimal resource usage while providing robust multi-provider support and extensibility through skills and tools. The system is optimized for $10 hardware with <10MB RAM footprint and sub-second startup times.