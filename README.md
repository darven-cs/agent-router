# Agent Router

Local API proxy for Claude Code with multi-upstream support, load balancing, automatic failover, usage monitoring, and TUI dashboard.

**Core Value:** Claude Code requests never fail — multi-upstream automatic failover guarantees availability, load balancing optimizes cost.

[English](README.md) | [中文](README_ZH.md)

---

## Features

- **Claude API Compatible** — Exposes `POST /v1/messages` endpoint, fully compatible with the official Claude SDK
- **Multi-Upstream Support** — Configure multiple upstream providers (Zhipu, Aicodee, Minimax) with per-upstream enable/disable toggle
- **Load Balancing** — FNV-1a hash modulo distribution across enabled upstreams
- **Primary Upstream** — Set a preferred upstream; automatically falls back after 3 consecutive failures
- **Automatic Failover** — Exponential backoff retry on timeout/5xx/429; cycles through all upstreams before giving up
- **Hot Config Reload** — SIGHUP signal, TUI button, or admin API to reload `config.yaml` without restart
- **Real-time TUI** — Monitor service status, upstream health, request logs, and token usage
- **Usage Tracking** — Local SQLite (WAL mode) persists every request with input/output tokens

## Quick Start

### Prerequisites

- Go 1.21+
- API keys for at least one upstream provider

### Build

```bash
make deps
make build
```

### Run

```bash
./agent-router
```

### Send a request

```bash
curl -X POST http://localhost:6856/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: your-router-api-key" \
  -d '{
    "model": "claude-sonnet-4-6",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

### Admin API

```bash
# Reload config
curl -X POST http://localhost:6856/admin/reload \
  -H "x-api-key: your-router-api-key"

# Get status
curl http://localhost:6856/admin/status \
  -H "x-api-key: your-router-api-key"
```

## TUI Controls

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate upstream list |
| `Space` | Toggle upstream enabled/disabled |
| `a` | Add new upstream |
| `e` | Edit selected upstream |
| `d` | Delete selected upstream |
| `m` | Select primary upstream |
| `r` | Reload config.yaml |
| `q` | Quit |

## Tech Stack

| Component | Technology |
|-----------|------------|
| HTTP Server | Go native `net/http` |
| TUI Framework | `charmbracelet/bubbletea` |
| TUI Styling | `charmbracelet/lipgloss` |
| Config | `gopkg.in/yaml.v3` |
| Database | SQLite + `gorm.io` |

## License

MIT
