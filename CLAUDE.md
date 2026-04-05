## 全局要求
**使用中文回答和写文档**：为了可读性，请你全程使用中文回答和写文档.

<!-- GSD:project-start source:PROJECT.md -->
## Project

**Agent Router**

本地轻量 API 中转服务，为 Claude Code 提供统一代理出口。对外暴露标准 Claude API（`/v1/messages`），无缝对接智谱、Aicodee、Minimax 三家上游，支持负载均衡、故障切换、配置热更新、用量监控。

**Core Value:** Claude Code 请求永不中断 — 多上游自动切换保障可用性，负载均衡优化成本。

### Constraints

- **Tech stack**: Go 原生 + 极简第三方库，单文件可运行
- **Storage**: 本地 SQLite，无外部服务依赖
- **Compatibility**: 完全兼容 Claude 官方 SDK
<!-- GSD:project-end -->

<!-- GSD:stack-start source:research/STACK.md -->
## Technology Stack

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
# Core dependencies
# SQLite driver (requires CGO - acceptable for local tool)
# Optional: Markdown rendering for TUI logs
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
- Use `go:embed` to embed config.yaml
- Use `github.com/knz/gozip` or similar for binary size
- bubbletea works on Windows via ansi emulation
- fsnotify has Windows support via ReadDirectoryChangesW
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
## Environment Variable Expansion
## HTTP Client for Upstream
## Sources
- Training data (MEDIUM confidence) — versions should be verified via `go list -m -versions` before implementation
- Go standard library documentation for net/http
- GORM documentation (gorm.io)
- Charmbracelet GitHub repositories for bubbletea/lipgloss
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

Conventions not yet established. Will populate as patterns emerge during development.
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

Architecture not yet mapped. Follow existing patterns found in the codebase.
<!-- GSD:architecture-end -->

<!-- GSD:workflow-start source:GSD defaults -->
## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd:quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd:debug` for investigation and bug fixing
- `/gsd:execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->



<!-- GSD:profile-start -->
## Developer Profile

> Profile not yet configured. Run `/gsd:profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
