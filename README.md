# Agent Router

Local API proxy that routes Claude Code requests to multiple upstream providers (Zhipu, Aicodee, Minimax) with automatic failover and load balancing.

## Features

- **Claude API Compatible**: Exposes POST /v1/messages endpoint fully compatible with Claude official SDK
- **Multi-Upstream Support**: Configure multiple upstream providers with enable/disable toggle
- **Load Balancing**: Modulo-hash distribution across enabled upstreams
- **Real-time TUI**: Monitor service status, upstream health, and request logs
- **Authentication**: API key validation on every request

## Quick Start

### Prerequisites

- Go 1.21+
- API keys for at least one upstream provider

### Configuration

1. Copy `config.yaml` and set your API keys:
   ```bash
   export AGENT_ROUTER_API_KEY="your-router-api-key"
   export ZHIPU_API_KEY="your-zhipu-key"
   export AICODEE_API_KEY="your-aicodee-key"
   export MINIMAX_API_KEY="your-minimax-key"
   ```

2. Edit `config.yaml` to enable/disable upstreams and adjust timeouts

### Build

```bash
make deps
make build
```

### Run

```bash
./agent-router
```

### Usage

Send requests to the proxy:
```bash
curl -X POST http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: your-router-api-key" \
  -d '{"model": "claude-3-5-sonnet", "messages": [{"role": "user", "content": "Hello"}]}'
```

### TUI Controls

- **q** or **Ctrl+C**: Quit

## Configuration

| Field | Description |
|-------|-------------|
| service.port | HTTP server port |
| service.api_key | API key for authentication |
| upstreams[].name | Display name |
| upstreams[].url | Upstream API endpoint |
| upstreams[].api_key | Upstream API key (supports env vars) |
| upstreams[].auth_type | "bearer" or "x-api-key" |
| upstreams[].enabled | Enable/disable upstream |
| upstreams[].timeout | Request timeout in seconds |

## Architecture

- `main.go`: Entry point, TUI orchestration
- `config.go`: Configuration loading with env expansion
- `proxy.go`: HTTP handler with authentication and proxying
- `upstream.go`: Load balancer with modulo-hash distribution
- `tui.go`: Bubbletea TUI for monitoring
