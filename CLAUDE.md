# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 🚀 Quick Start Commands

### Building and Running
```bash
# Build for current platform
make build

# Build for all platforms
make build-all

# Build and install
make install

# Run tests
make test

# Format code
make fmt

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
```

## 🏗️ Architecture Overview

### Core Components

**Agent System (`pkg/agent/`)**
- Message processing loop with tool execution
- Session management and context building
- Memory management for long-term context
- Tool iteration limits and safety controls

**Provider System (`pkg/providers/`)**
- HTTP-based LLM providers with generic API support
- Fallback provider system for multi-provider resilience
- OAuth and token-based authentication support
- Auto-detection of providers based on model names

**Tool System (`pkg/tools/`)**
- Tool registry and execution framework
- Built-in tools: file operations, web search, shell execution, cron jobs
- Contextual tools that can receive message context
- Subagent spawning for complex tasks

**Channel System (`pkg/channels/`)**
- Multi-platform messaging support (Telegram, Discord, Feishu, WhatsApp, etc.)
- Message formatting and media handling
- Thinking indicators and response streaming
- Media download and transcription support

**Configuration (`pkg/config/`)**
- JSON-based configuration with environment variable support
- Support for multiple LLM providers with fallback configurations
- Channel-specific configurations with access controls
- Workspace and tool configurations

### Data Flow

1. **Message Reception**: Channels receive messages from various platforms
2. **Agent Processing**: Messages are processed through the agent loop
3. **Tool Execution**: Agent invokes tools as needed for task completion
4. **LLM Provider**: Primary provider is used with fallbacks on failure
5. **Response Generation**: Final response is sent back through the channel

### Key Interfaces

- `LLMProvider`: Core interface for all LLM providers
- `ToolDefinition`: Interface for tools that can be executed by the agent
- `MessageBus`: Event-driven communication between components
- `ToolRegistry`: Manages registration and discovery of tools

## 📁 File Organization

```
cmd/
├── picoclaw/      # Main CLI application
└── lambda/        # AWS Lambda serverless handler

pkg/
├── agent/         # Core AI agent logic
├── providers/     # LLM provider implementations
├── tools/         # Tool registry and execution
├── channels/      # Multi-channel messaging support
├── config/        # Configuration management
├── bus/           # Message bus for inter-component communication
├── auth/          # OAuth authentication and token management
└── skills/        # Skill system for extensibility

skills/            # Builtin skills directory
docker/            # Container deployment configuration
```

## 🔧 Configuration Structure

### Primary Config File
`~/.picoclaw/config.json` contains:
- **Agents**: Model selection, workspace paths, context window settings
- **Channels**: Platform-specific configurations with access controls
- **Providers**: Multiple LLM provider support with API keys and endpoints
- **Tools**: Web search and other tool configurations
- **Gateway**: HTTP server settings for the messaging gateway

### Environment Variables
- `PICOCLAW_CONFIG_JSON`: Full config as JSON string (for containers)
- Platform-specific variables for build targets
- API keys and credentials for various providers

## 🎯 Development Patterns

### Provider Implementation
New LLM providers should implement the `LLMProvider` interface and register themselves in the provider factory.

### Tool Development
Tools extend the `ToolDefinition` interface and are registered in the tool registry. Consider using contextual tools for access to message context.

### Channel Integration
New channels implement the channel interface and register message handlers for different platform events.

### Skill System
Skills are self-contained directories with a `SKILL.md` file describing functionality. They can add new tools, modify agent behavior, or extend the system.

## 🧪 Testing Strategy

### Unit Tests
- Focus on individual components in `pkg/` directories
- Use table-driven tests for provider and tool functionality
- Mock external dependencies (HTTP clients, file systems)

### Integration Tests
- Test end-to-end agent workflows
- Verify multi-provider fallback behavior
- Test channel message processing

### Testing Commands
```bash
# Test specific package
go test ./pkg/providers -v

# Test with race detector
go test -race ./pkg/...

# Benchmark performance
go test -bench=. ./pkg/agent
```

## 🚨 Common Gotchas

1. **Provider Fallbacks**: Ensure fallback providers are properly configured in config.json
2. **API Keys**: Never commit API keys - use environment variables or config files
3. **Memory Management**: Monitor memory usage, especially with long conversations
4. **Channel Permissions**: Configure `allow_from` lists to restrict access
5. **Cross-Platform Builds**: Use proper GOOS/GOARCH settings for different targets

## 🔍 Debugging Tips

### Logging
- Use structured logging with context
- Enable verbose mode with `-v` flag
- Check logs in workspace directory

### Performance
- Profile memory usage during long conversations
- Monitor provider response times
- Check tool execution durations

### Configuration Issues
- Validate JSON configuration syntax
- Check API key validity for providers
- Verify channel token permissions

## 📚 Key Files to Understand

- `pkg/agent/loop.go`: Core message processing logic
- `pkg/providers/http_provider.go`: Generic HTTP provider implementation
- `pkg/providers/fallback_provider.go`: Multi-provider fallback system
- `pkg/tools/registry.go`: Tool registration and discovery
- `cmd/picoclaw/main.go`: CLI command handling
- `config.example.json`: Complete configuration template

This repository represents an ultra-lightweight AI assistant designed for minimal resource usage while providing robust multi-provider support and extensibility through skills and tools.