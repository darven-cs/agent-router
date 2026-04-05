# Technology Stack: v2.0 Architecture Refactor

**Project:** Agent Router
**Researched:** 2026-04-05
**Scope:** Stack additions/changes for v2.0 refactor ONLY (Event Bus, Middleware Chain, Project Layout, TUI Decomposition)
**Overall confidence:** HIGH

## Current Stack (Unchanged)

These are already validated in v1.0. No changes needed.

| Technology | Actual Version (go.mod) | Purpose |
|------------|------------------------|---------|
| Go | 1.24.0 | Language runtime |
| net/http (stdlib) | Go 1.24.0 | HTTP server and client |
| charmbracelet/bubbletea | v1.3.10 | TUI framework (Elm architecture) |
| charmbracelet/lipgloss | v1.1.0 | TUI styling (NOT v2.x -- go.mod says v1.1.0) |
| gopkg.in/yaml.v3 | v3.0.1 | Config file parsing |
| gorm.io/gorm | v1.31.1 | ORM for usage tracking |
| gorm.io/driver/sqlite | v1.6.0 | SQLite driver for GORM |
| mattn/go-sqlite3 | v1.14.40 | CGO SQLite driver (transitive) |

**Previous v1.0 STACK.md errors corrected:** lipgloss is v1.1.0 not v2.x; bubbletea is v1.3.10 not "v1.x"; fsnotify was listed but is NOT in go.mod and NOT used (SIGHUP via os/signal suffices).

---

## v2.0 NEW Capabilities: Stack Additions

### 1. Event Bus -- NO new dependency

**Approach:** Custom Go implementation using typed channels + sync primitives.

**Why no library:** The project already uses channel-based pub/sub patterns (usageChan, logChan in main.go). A formal EventBus struct is a natural extension of existing patterns, not a new paradigm. External Go event bus libraries (e.g., `asaskevich/EventBus`) use reflection-based dispatch which adds complexity without benefit for a project with ~10 event types.

**Implementation pattern:**

```go
// internal/eventbus/bus.go
type EventBus struct {
    mu       sync.RWMutex
    handlers map[string][]chan Event
}

type Event interface{ EventTopic() string }

func New() *EventBus
func (eb *EventBus) Subscribe(topic string) <-chan Event
func (eb *EventBus) Publish(topic string, evt Event)
func (eb *EventBus) Close()
```

**Go stdlib primitives used:**
- `sync.RWMutex` for thread-safe handler registration
- `chan Event` (buffered) for async event delivery
- `context.Context` for graceful shutdown of subscriber goroutines

**Topics to implement (replaces hardcoded callbacks in main.go):**

| Topic | Replaces Callback | Payload |
|-------|-------------------|---------|
| `upstream.added` | `OnUpstreamAdded` | `*Upstream` |
| `upstream.updated` | `OnUpstreamUpdated` | `*Upstream, oldName string` |
| `upstream.deleted` | `OnUpstreamDeleted` | `name string` |
| `upstream.toggled` | `OnUpstreamToggled` | `*Upstream` |
| `model.changed` | `OnDefaultModelChanged` | `model string` |
| `upstream.model_selected` | `OnUpstreamModelSelected` | `*Upstream` |
| `config.reload` | `OnReload` | (none) |

### 2. Middleware Chain -- Custom Chain() function (optional: justinas/alice v1.2.0)

**Approach A (Recommended): Custom `Chain()` function**

The standard Go middleware pattern `func(http.Handler) http.Handler` composed via a simple `Chain()` helper. This is ~10 lines of code and zero dependencies.

```go
// internal/proxy/middleware.go
type Middleware func(http.Handler) http.Handler

func Chain(mw ...Middleware) Middleware {
    return func(next http.Handler) http.Handler {
        for i := len(mw) - 1; i >= 0; i-- {
            next = mw[i](next)
        }
        return next
    }
}
```

**Approach B (Optional): justinas/alice v1.2.0**

If the team prefers a battle-tested library over 10 lines of custom code. Alice is the de facto standard for Go middleware chaining -- minimal, no reflection, ~50 lines total.

| Attribute | Value |
|-----------|-------|
| Package | `github.com/justinas/alice` |
| Version | v1.2.0 (latest, verified via proxy.golang.org) |
| License | MIT |
| Dependencies | None (stdlib only) |
| Size | ~50 lines of code |
| Last updated | Stable, no active changes needed |

**Recommendation:** Use Approach A (custom `Chain()`). The project constraint is "Go native + minimal third-party libs." Adding a dependency for 10 lines of code contradicts that principle. Alice is the correct fallback only if the middleware chain grows complex (10+ middleware functions).

**Middleware decomposition plan for proxy.go ServeHTTP:**

```
Request → AuthMiddleware → RoutingMiddleware → ModelTransformMiddleware → RetryMiddleware → CoreHandler
```

| Middleware | Current Location | Extracts From |
|------------|-----------------|---------------|
| AuthMiddleware | proxy.go lines 101-113 | API key validation from headers |
| RoutingMiddleware | proxy.go lines 77-99, 96-99 | Path/method routing |
| ModelTransformMiddleware | proxy.go lines 128-137 | Model name in request body |
| RetryMiddleware | proxy.go lines 215-257 | Retry loop with exponential backoff |
| CoreHandler | proxy.go lines 125-213 | Actual upstream proxy request |

### 3. Standard Go Project Layout -- No dependency, structural change only

**Approach:** Follow official Go module layout from go.dev documentation.

**Target directory structure:**

```
agent-router/
  cmd/
    agent-router/
      main.go              # Entry point: wire dependencies, start server + TUI
  internal/
    config/
      config.go            # LoadConfig, SaveConfig, Config struct
    eventbus/
      bus.go               # EventBus, Event interface, Subscribe/Publish
    proxy/
      handler.go           # ProxyHandler struct, CoreHandler
      middleware.go         # Middleware type, Chain(), individual middleware funcs
      transform.go          # transformModelName (extracted from proxy.go)
      retry.go             # isRetryable, retry logic (extracted)
    upstream/
      upstream.go           # Upstream, UpstreamConfig, SharedUpstreams
      loadbalancer.go       # LoadBalancer interface, ModuloHash implementation
    usage/
      usage.go              # RequestLog, initDB, StartUsageWorker
    tui/
      app.go                # Root model (replaces tui.go model struct)
      styles.go             # Catppuccin palette, style definitions
      messages.go           # Message types (UpstreamAdded, ReloadRequest, etc.)
      update.go             # Update() handler, delegates to sub-components
      view.go               # View() renderer, assembles nav + content + status
      views/
        nav.go              # renderNavigation()
        upstream_list.go    # renderUpstreamList()
        model_select.go     # renderModelSelect(), handleModelSelect()
        confirmation.go     # renderConfirmation(), handleConfirm()
        status.go           # renderStatus()
      forms/
        upstream_form.go    # renderForm(), handleFormInput(), submitForm()
  go.mod
  go.sum
  config.yaml
```

**Why no `/pkg/` directory:** The project is a single binary, not a library. `/pkg/` is for exported packages consumed by external projects. `/internal/` enforces privacy via the Go compiler -- no other module can import these packages.

**Why `/internal/tui/views/` and `/internal/tui/forms/` subdirectories:** The TUI is the largest component (837 lines in tui.go). Decomposing into sub-packages keeps each file under 200 lines while maintaining logical cohesion. Views are read-only renderers; forms handle mutable input state.

### 4. TUI Component Decomposition -- No new dependency (bubbletea nested model pattern)

**Approach:** Top-down nested model composition using bubbletea's built-in Elm Architecture.

Each TUI component is a bubbletea model (implements `Init()`, `Update()`, `View()`). The root model delegates to child components.

```go
// internal/tui/app.go -- Root model
type App struct {
    nav     Nav
    content Content
    status  Status
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // Route messages to the correct sub-component
    var cmd tea.Cmd
    switch msg.(type) {
    case tea.WindowSizeMsg:
        a.nav, cmd = a.nav.Update(msg)
        a.content, _ = a.content.Update(msg)
        a.status, _ = a.status.Update(msg)
    case RequestLog:
        a.status, cmd = a.status.Update(msg)
    default:
        a.content, cmd = a.content.Update(msg)
    }
    return a, cmd
}

func (a App) View() string {
    return lipgloss.JoinVertical(lipgloss.Top,
        a.nav.View(),
        a.content.View(),
        a.status.View(),
    )
}
```

**Why NOT bubblon (`github.com/donderom/bubblon` v1.2.1):**

Bubblon implements a "Model Stack" pattern for screen navigation (push/pop screens like a navigation controller). This project uses a single-screen layout with three fixed regions (nav bar, content area, status bar). There are no screens to push or pop. The model stack pattern would add indirection without benefit. The top-down nested model pattern is the correct fit.

**Why NOT charmbracelet/bubbles:** Bubbles provides pre-built components (text inputs, lists, etc.). The project uses custom rendering with Catppuccin Mocha palette and a unique layout. Bubbles' opinionated styling would conflict with the existing design system. Custom components are the right choice here.

---

## What NOT to Add

| Do NOT Add | Why | What to Do Instead |
|------------|-----|-------------------|
| `asaskevich/EventBus` or any Go event bus library | Reflection-based dispatch, adds complexity for ~10 event types | Custom EventBus with typed channels (matches existing usageChan pattern) |
| `justinas/alice` v1.2.0 | Only saves ~10 lines of trivial code; contradicts "minimal deps" constraint | Custom `Chain()` function using `func(http.Handler) http.Handler` |
| `donderom/bubblon` v1.2.1 | Model Stack for screen navigation; this project has single-screen layout | Top-down nested model pattern (bubbletea built-in) |
| `charmbracelet/bubbles` | Pre-built components with opinionated styling; conflicts with Catppuccin palette | Custom rendering via lipgloss styles (already in use) |
| `/pkg/` directory | Project is a single binary, not a library | `/internal/` for all packages |
| `gorilla/mux` or any router | Deprecated (gorilla); overkill (chi/echo) for 2 endpoints | net/http ServeMux or direct path matching (already in use) |
| `hashicorp/go-retryablehttp` | Current retry logic works fine, would be a rewrite for no gain | Keep existing retry with exponential backoff |
| `fsnotify` | Not needed; SIGHUP via os/signal already works (not in go.mod) | Keep existing SIGHUP handler |

---

## Installation (v2.0 additions)

```bash
# v2.0 adds NO new dependencies.
# All new capabilities are implemented with Go stdlib:
#   - Event Bus: sync.RWMutex + chan Event
#   - Middleware Chain: func(http.Handler) http.Handler
#   - Project Layout: directory restructuring only
#   - TUI Decomposition: bubbletea nested model pattern

# Optional (only if team decides against custom Chain()):
# go get github.com/justinas/alice@v1.2.0
```

---

## Version Compatibility (v2.0 additions)

| New Code | Depends On | Compatible | Notes |
|----------|-----------|------------|-------|
| Custom EventBus | Go sync package | Go 1.24.0 | stdlib, no version concern |
| Custom Chain() | net/http | Go 1.24.0 | stdlib, HandlerFunc stable since Go 1.0 |
| TUI nested models | bubbletea v1.3.10 | Yes | Elm Architecture is core to bubbletea, stable API |
| /internal/ layout | Go compiler | Go 1.24.0 | /internal/ enforced since Go 1.4 |

---

## Migration Path (v1.0 -> v2.0)

### Phase order for dependency introduction:

1. **Project Layout** (ARCH-01) -- Zero code change risk. Just move files into /internal/ and fix import paths. Do this first because all other phases depend on the new directory structure.

2. **Event Bus** (ARCH-02) -- Introduces EventBus struct. Replace 7 callbacks in main.go with Subscribe/Publish calls. Depends on layout being done so bus.go has a home.

3. **Middleware Chain** (ARCH-03) -- Decompose ProxyHandler.ServeHTTP into middleware functions. Depends on layout so middleware.go has a home. Can be done in parallel with Event Bus since they touch different files.

4. **TUI Decomposition** (TUI-02) -- Split 837-line tui.go into sub-modules. Depends on layout so tui/ package has a home. Should be done AFTER Event Bus since TUI callbacks will be replaced by event subscriptions.

---

## Sources

- Go module proxy (proxy.golang.org) for alice v1.2.0 and bubblon v1.2.1 version verification -- HIGH confidence
- go.dev official documentation for /internal/ and /cmd/ layout conventions -- HIGH confidence
- charmbracelet/bubbletea GitHub for nested model composition patterns -- HIGH confidence
- donderom/bubblon GitHub README for Model Stack architecture evaluation -- HIGH confidence
- justinas/alice GitHub for middleware chaining API -- HIGH confidence
- Alex Edwards "Making and Using HTTP Middleware" tutorial for Chain() pattern -- MEDIUM confidence
- Current go.mod and source code analysis for actual dependency versions -- HIGH confidence
- Previous v1.0 research (2026-04-03) for validated stack decisions -- HIGH confidence

---
*Stack research for: Agent Router v2.0 Architecture Refactor*
*Researched: 2026-04-05*
