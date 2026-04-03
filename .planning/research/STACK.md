# Stack Research

**Domain:** Go Local API Proxy / Router
**Researched:** 2026-04-03
**Confidence:** MEDIUM (based on training data; verify versions at implementation)

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go native net/http | 1.21+ | HTTP server & client | Standard library, zero dependencies, production-proven |
| GORM | v1.25.x | ORM for SQLite operations | De facto standard for Go ORMs, excellent SQLite support |
| gorm.io/driver/sqlite | v1.5.x | SQLite driver for GORM | Official GORM SQLite driver, cgo-free (mattn/go-sqlite3) |
| charmbracelet/bubbletea | v1.x | TUI framework | Declarative Elm-like architecture, built on Termbox |
| charmbracelet/lipgloss | v2.x | TUI styling |Composable styles, 256-color support, works with bubbletea |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| gopkg.in/yaml.v3 | v3.x | YAML config parsing | Standard for Go YAML, used by Kubernetes |
| github.com/fsnotify/fsnotify | v1.7.x | File system watching | Hot config reload via SIGHUP |
| github.com/mattn/go-sqlite3 | v1.14.x | SQLite driver (CGO) | Required by gorm SQLite driver |
| github.com/charmbracelet/glamour | v0.6.x | Markdown rendering | Optional: for rendering logs in TUI |
| github.com/muesli/termenv | latest | Terminal capabilities | Optional: advanced terminal detection for lipgloss |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| golangci-lint | Linting | Standard Go linter aggregator |
| air | Hot reload during dev | Live reload for development |
| go-bindata | Embed config files | Optional: embed config.yaml in binary |

## Installation

```bash
# Core dependencies
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/lipgloss@latest
go get gorm.io/gorm@latest
go get gorm.io/driver/sqlite@latest
go get gopkg.in/yaml.v3@latest
go get github.com/fsnotify/fsnotify@latest

# SQLite driver (requires CGO - acceptable for local tool)
go get github.com/mattn/go-sqlite3@latest

# Optional: Markdown rendering for TUI logs
go get github.com/charmbracelet/glamour@latest
```

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| net/http (native) | gin, echo, fiber | If REST DSL needed; overkill for simple proxy |
| GORM | raw sqlx, sqlc | If maximum performance; GORM sufficient for usage tracking |
| fsnotify | polling (inotify-tools) | fsnotify is event-driven, more efficient |
| bubbletea | tview, go-ui | bubbletea has better composition model |
| gopkg.in/yaml.v3 | toml, hjson | TOML if preferred; YAML has better ecosystem |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| gorilla/mux | Deprecated, unmaintained | net/http with httputil or go-chi |
| gorp | Old, unmaintained | GORM |
| viper | Overcomplicated for local tool | gopkg.in/yaml.v3 + os.ExpandEnv |
| tview | Callback-based, harder to compose | bubbletea (Elm architecture) |

## Stack Patterns by Variant

**If single binary distribution is critical:**
- Use `go:embed` to embed config.yaml
- Use `github.com/knz/gozip` or similar for binary size

**If Windows compatibility needed:**
- bubbletea works on Windows via ansi emulation
- fsnotify has Windows support via ReadDirectoryChangesW

**If minimal binary size needed:**
- Skip glamour (adds ~5MB)
- Use raw lipgloss styling only

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|-----------------|-------|
| bubbletea@1.x | lipgloss@2.x | Major versions aligned |
| GORM@1.25.x | go-sqlite3@1.14.x | Compatible, sqlite3 v2 also available |
| fsnotify@1.7.x | Go 1.17+ | Requires Go 1.17+ for some features |
| yaml.v3 | Any Go version | Pure Go implementation |

## Hot Config Reload Implementation

For the config reload feature, combine:

1. **Signal handling**: `os/signal` for SIGHUP
2. **File watching**: `fsnotify` for config changes
3. **Thread-safe config**: Use `sync.RWMutex` for config access

```go
// Config reload pattern
type Config struct {
    mu sync.RWMutex
    // config fields
}

func (c *Config) Reload(path string) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    // re-read YAML
}

func (c *Config) Get() Config {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.config
}
```

## Environment Variable Expansion

Use `os.ExpandEnv` for config file variable substitution:

```go
// In YAML unmarshaling
func ExpandEnv(s string) string {
    return os.ExpandEnv(s)
}

// Usage: Read YAML, then iterate and expand
```

## HTTP Client for Upstream

Go native `http.Client` is sufficient:

```go
client := &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:    90 * time.Second,
    },
}
```

For connection pooling and retry logic, use `github.com/hashicorp/go-retryablehttp` as alternative.

## Sources

- Training data (MEDIUM confidence) — versions should be verified via `go list -m -versions` before implementation
- Go standard library documentation for net/http
- GORM documentation (gorm.io)
- Charmbracelet GitHub repositories for bubbletea/lipgloss

---
*Stack research for: Go Local API Proxy*
*Researched: 2026-04-03*
