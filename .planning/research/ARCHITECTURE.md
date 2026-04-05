# Architecture Research

**Domain:** Go API Proxy Service -- v2.0 Modular Refactor
**Researched:** 2026-04-05
**Confidence:** HIGH

## Current State Analysis

### Existing Architecture (v1.0 Monolith)

```
main.go (assembly + globals)
  ├── proxy.go     (HTTP routing + auth + retry + model rewrite + logging)
  ├── tui.go       (838 lines: navigation + forms + model select + rendering)
  ├── config.go    (YAML load/save)
  ├── upstream.go  (LoadBalancer + SharedUpstreams)
  ├── usage.go     (SQLite + async worker)
  └── admin.go     (status + reload endpoints)
```

### Coupling Problems Identified

| Problem | Location | Impact |
|---------|----------|--------|
| **TUI holds business logic callbacks** | `tui.go` lines 145-151: 6 function callbacks (OnUpstreamAdded, OnUpstreamUpdated, etc.) | TUI knows about SharedUpstreams, LoadBalancer, persistConfig -- cannot test TUI without full system |
| **main.go wires everything inline** | `main.go` lines 88-136: 50 lines of closure callbacks | Any new feature requiring cross-component coordination must touch main.go |
| **ProxyHandler does routing** | `proxy.go` lines 77-99: admin endpoint routing inside ServeHTTP | Adding routes means editing ProxyHandler.ServeHTTP |
| **Global mutable state** | `main.go` lines 17-26: 7 package-level vars (db, lb, proxyHandler, cfg, etc.) | No encapsulation, any file can mutate any state |
| **Dual channel redundancy** | `proxy.go` lines 301-329: logToChan + logToChanWithTokens | Two separate channels for same logical event with different data |
| **Auth logic duplicated** | `proxy.go` lines 101-113 and `admin.go` lines 50-61: identical token extraction | Any auth change requires editing two places |

## Target Architecture

### System Overview

```
┌──────────────────────────────────────────────────────────────────┐
│                          cmd/agent-router/                        │
│                      main.go (thin assembly only)                 │
├──────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │                    internal/eventbus/                        │ │
│  │              EventBus (Go channel pub/sub)                   │ │
│  │   Subscribe: "upstream.added", "config.reload", etc.        │ │
│  └──────────┬────────────────────┬─────────────────────────────┘ │
│             │                    │                                │
│  ┌──────────▼──────────┐  ┌─────▼──────────────────────────────┐│
│  │ internal/proxy/      │  │ internal/tui/                      ││
│  │ ┌─────────────────┐  │  │ ┌──────────────────────────────┐  ││
│  │ │ middleware/      │  │  │ │ app.go (root model)          │  ││
│  │ │  auth.go         │  │  │ ├──────────────────────────────┤  ││
│  │ │  logging.go      │  │  │ │ components/                  │  ││
│  │ │  requestid.go    │  │  │ │  nav.go                      │  ││
│  │ │  modelrewrite.go │  │  │ │  upstream_list.go            │  ││
│  │ ├─────────────────┤  │  │ │  form.go                     │  ││
│  │ │ handler.go       │  │  │ │  model_select.go             │  ││
│  │ │ router.go        │  │  │ │  confirm.go                  │  ││
│  │ │ retry.go         │  │  │ │  status.go                   │  ││
│  │ ├─────────────────┤  │  │ ├──────────────────────────────┤  ││
│  │ │ balancer.go      │  │  │ │ styles/                      │  ││
│  │ └─────────────────┘  │  │ │  theme.go                    │  ││
│  └──────────────────────┘  │ └──────────────────────────────┘  ││
│                             └────────────────────────────────────┘│
│  ┌──────────────────────┐  ┌────────────────────────────────────┐│
│  │ internal/config/     │  │ internal/storage/                  ││
│  │  config.go           │  │  sqlite.go                         ││
│  │  (load/save/watch)   │  │  usage.go                          ││
│  └──────────────────────┘  └────────────────────────────────────┘│
│  ┌──────────────────────┐                                        │
│  │ internal/upstream/   │  Types shared across packages          │
│  │  upstream.go         │  (Upstream, LoadBalancer, Pool)       │
│  └──────────────────────┘                                        │
└──────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | New or Modified |
|-----------|----------------|-----------------|
| `cmd/agent-router/main.go` | Thin assembly: create EventBus, wire subscribers, start HTTP + TUI | **Modified** (drastically simplified) |
| `internal/eventbus/` | Typed pub/sub via Go channels; replaces direct callbacks | **NEW** |
| `internal/proxy/middleware/` | Composable http.Handler decorators (auth, logging, model rewrite) | **NEW** (extracted from proxy.go) |
| `internal/proxy/handler.go` | Route to middleware chain, dispatch to upstream | **Modified** (simplified) |
| `internal/proxy/retry.go` | Exponential backoff retry with isRetryable check | **Modified** (extracted) |
| `internal/proxy/balancer.go` | LoadBalancer + SharedUpstreams | **Modified** (moved from upstream.go) |
| `internal/tui/app.go` | Root bubbletea model, message routing to sub-models | **Modified** (simplified) |
| `internal/tui/components/` | Individual UI components (nav, list, form, confirm, status) | **NEW** (extracted from tui.go) |
| `internal/tui/styles/` | Catppuccin theme, shared style definitions | **NEW** (extracted from tui.go) |
| `internal/config/` | Config load/save/watch | **Modified** (moved from config.go) |
| `internal/storage/` | SQLite init, usage worker, stats | **Modified** (moved from usage.go) |
| `internal/upstream/` | Upstream type, pool management | **Modified** (consolidated) |

## Recommended Project Structure

```
agent-router/
├── cmd/
│   └── agent-router/
│       └── main.go              # Thin entry: wire event bus, start server + TUI
│
├── internal/
│   ├── eventbus/
│   │   └── bus.go               # Event definitions + pub/sub via channels
│   │
│   ├── proxy/
│   │   ├── handler.go           # HTTP routing, middleware chain assembly
│   │   ├── retry.go             # Retry with exponential backoff
│   │   ├── balancer.go          # LoadBalancer + SharedUpstreams
│   │   └── middleware/
│   │       ├── auth.go          # API key extraction + validation
│   │       ├── logging.go       # Request logging to event bus
│   │       ├── requestid.go     # Request ID extraction
│   │       └── modelrewrite.go  # Model name transformation
│   │
│   ├── tui/
│   │   ├── app.go               # Root tea.Model, Update/View delegation
│   │   ├── components/
│   │   │   ├── nav.go           # Top navigation bar
│   │   │   ├── upstream_list.go # Main upstream list view
│   │   │   ├── form.go          # Add/Edit form
│   │   │   ├── model_select.go  # Model selection view
│   │   │   ├── confirm.go       # Confirmation dialog
│   │   │   └── status.go        # Bottom status bar
│   │   └── styles/
│   │       └── theme.go         # Catppuccin palette, shared styles
│   │
│   ├── config/
│   │   └── config.go            # Config types, LoadConfig, SaveConfig
│   │
│   ├── storage/
│   │   ├── sqlite.go            # DB init, WAL mode, schema migration
│   │   └── usage.go             # UsageLog model, worker, stats
│   │
│   └── upstream/
│       └── upstream.go          # Upstream type, UpstreamConfig
│
├── go.mod
├── go.sum
├── config.yaml
└── Makefile
```

### Structure Rationale

- **`cmd/agent-router/`**: Single binary entry point per [official Go module layout](https://go.dev/doc/modules/layout). Keeps root directory clean.
- **`internal/`**: All packages here cannot be imported externally, giving freedom to refactor APIs without breakage.
- **`internal/eventbus/`**: Single file sufficient for this scale. Events are typed Go structs, not stringly-typed.
- **`internal/proxy/middleware/`**: Separate sub-package because each middleware is self-contained and testable independently.
- **`internal/tui/components/`**: Each component owns its own Update/View methods following the nested model pattern.
- **`internal/tui/styles/`**: Theme extraction prevents 100+ lines of style declarations polluting component files.

## Architectural Patterns

### Pattern 1: Event Bus (Go Channel Pub/Sub)

**What:** A central event bus using typed Go channels replaces direct function callbacks between TUI and business logic.

**When to use:** When multiple components need to react to the same event (TUI update + config persist + LB update on upstream change). When you want to decouple event producers from consumers.

**Why this over callbacks:** The current `main.go` wires 6 closures that each call SharedUpstreams, LoadBalancer, and persistConfig. Every new feature that needs cross-component coordination requires editing main.go. An event bus lets each component subscribe independently.

**Trade-offs:** Slight runtime overhead from channel operations (negligible at this scale). Adds one abstraction layer. Not needed for 1:1 direct calls.

**Implementation:**
```go
// internal/eventbus/bus.go

// Event types -- each is a distinct Go type for type-safe dispatch
type Event interface{ eventMarker() }

type UpstreamAddedEvent struct{ Upstream *upstream.Upstream }
func (UpstreamAddedEvent) eventMarker() {}

type UpstreamUpdatedEvent struct{ Upstream *upstream.Upstream; OldName string }
func (UpstreamUpdatedEvent) eventMarker() {}

type UpstreamDeletedEvent struct{ Name string }
func (UpstreamDeletedEvent) eventMarker() {}

type ConfigReloadRequestEvent struct{}
func (ConfigReloadRequestEvent) eventMarker() {}

type ConfigReloadedEvent struct{ Config *config.Config }
func (ConfigReloadedEvent) eventMarker() {}

type ModelSelectedEvent struct{ Upstream *upstream.Upstream }
func (ModelSelectedEvent) eventMarker() {}

type RequestLogEvent struct{ Log RequestLog }
func (RequestLogEvent) eventMarker() {}

// EventBus provides type-safe pub/sub via Go channels
type EventBus struct {
    subscribers map[reflect.Type][]chan Event
    mu          sync.RWMutex
}

func New() *EventBus {
    return &EventBus{
        subscribers: make(map[reflect.Type][]chan Event),
    }
}

// Subscribe returns a read-only channel for a specific event type.
// The caller drains the channel in its own goroutine.
func Subscribe[T Event](bus *EventBus) <-chan T {
    ch := make(chan T, 64)
    bus.mu.Lock()
    typ := reflect.TypeOf((*T)(nil)).Elem()
    // Wrap typed channel into the generic subscriber list
    bus.subscribers[typ] = append(bus.subscribers[typ], wrapChan(ch))
    bus.mu.Unlock()
    return ch
}

// Publish sends an event to all subscribers of that type.
// Non-blocking: drops if channel full (acceptable for TUI updates).
func Publish(bus *EventBus, evt Event) {
    typ := reflect.TypeOf(evt)
    bus.mu.RLock()
    for _, ch := range bus.subscribers[typ] {
        select {
        case ch <- evt:
        default: // drop if consumer is slow
        }
    }
    bus.mu.RUnlock()
}
```

**Wiring in main.go becomes:**
```go
bus := eventbus.New()

// Config subscriber: persists on any upstream change
configSub := eventbus.Subscribe[UpstreamAddedEvent](bus)
go func() {
    for evt := range configSub {
        persistConfig()
    }
}()

// LB subscriber: updates load balancer
lbSub := eventbus.Subscribe[UpstreamAddedEvent](bus)
go func() {
    for evt := range lbSub {
        lb.AddUpstream(evt.Upstream)
    }
}()

// TUI publishes events instead of calling callbacks directly
```

### Pattern 2: Middleware Chain (Onion/Decorator)

**What:** Each cross-cutting concern becomes a standalone `func(http.Handler) http.Handler` that wraps the next handler. Handlers are chained left-to-right, executing outer-to-inner then inner-to-outer on return.

**When to use:** For HTTP request processing concerns that apply to multiple routes (auth, logging, request ID, model rewriting).

**Why this over current approach:** Currently `proxy.go` mixes routing, auth, model rewriting, retry, and logging in a single 335-line file. Middleware extraction lets each concern be tested independently and reordered without touching others.

**Trade-offs:** Slightly more files. Each middleware is trivially testable. No third-party library needed -- the pattern is pure stdlib.

**Implementation:**
```go
// internal/proxy/middleware/middleware.go

type Middleware func(http.Handler) http.Handler

// Chain builds a middleware chain. Middlewares are applied left-to-right:
// Chain(h, A, B, C) means A wraps B wraps C wraps h.
func Chain(h http.Handler, mws ...Middleware) http.Handler {
    for i := len(mws) - 1; i >= 0; i-- {
        h = mws[i](h)
    }
    return h
}

// internal/proxy/middleware/auth.go
func Auth(validKey string) Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            token := extractToken(r) // shared helper, eliminates duplication
            if token != validKey {
                writeError(w, http.StatusUnauthorized, "authentication_error", "Invalid API key")
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}

// internal/proxy/middleware/modelrewrite.go
func ModelRewrite(defaultModel string, getUpstreamModel func() string) Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Read, transform, replace body
            // ...extracted from proxy.go lines 129-137
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

**Assembly in handler.go:**
```go
// internal/proxy/handler.go

func NewHandler(cfg *config.Config, lb *balancer.LoadBalancer, bus *eventbus.EventBus) http.Handler {
    mux := http.NewServeMux()
    mux.HandleFunc("/v1/messages", handleMessages)
    mux.HandleFunc("/admin/status", handleAdminStatus)
    mux.HandleFunc("/admin/reload", handleAdminReload)

    return Chain(mux,
        middleware.RequestID(),
        middleware.Logging(bus),
        middleware.Auth(cfg.Service.APIKey),
    )
}
```

### Pattern 3: TUI Componentization (Nested Models)

**What:** The monolithic 838-line `tui.go` is decomposed into a root `app.go` model that delegates `Update()` and `View()` to child component models. Each component manages its own state.

**When to use:** When a single bubbletea model exceeds ~300 lines with multiple distinct visual modes (navigation, form, confirmation, model selection).

**Why this over current approach:** The current `model` struct has 15+ fields for different modes (formMode, formData, formField, modelSelectMode, confirmMode, confirmType). Each mode's Update/View logic is interleaved in a single file. Componentization isolates each mode.

**Trade-offs:** More files. Requires a message-routing pattern in the root model. But each component becomes independently understandable and testable.

**Implementation (nested model pattern):**
```go
// internal/tui/app.go -- root model

type AppModel struct {
    width, height int

    // Child components
    nav           components.NavComponent
    upstreamList  components.UpstreamListComponent
    form          components.FormComponent
    modelSelect   components.ModelSelectComponent
    confirm       components.ConfirmComponent
    statusBar     components.StatusBarComponent

    // Which component is active in the content area
    activeView    ViewType  // "list", "form-add", "form-edit", "model-select", "confirm-delete", "confirm-shutdown"

    // Shared data
    upstreams     []*upstream.Upstream
    defaultModel  string
    eventBus      *eventbus.EventBus
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width, m.height = msg.Width, msg.Height
        // Propagate size to all children
        m.nav.SetSize(m.width)
        m.statusBar.SetSize(m.width)
    case RequestLog:
        m.statusBar.UpdateLog(msg)
    }

    // Route to active content component
    var cmd tea.Cmd
    switch m.activeView {
    case "list":
        m.upstreamList, cmd = m.upstreamList.Update(msg)
        m.handleListActions(&m)  // check if list emitted a mode change
    case "form-add", "form-edit":
        m.form, cmd = m.form.Update(msg)
        m.handleFormActions(&m)  // check if form submitted or cancelled
    case "model-select":
        m.modelSelect, cmd = m.modelSelect.Update(msg)
    case "confirm-delete", "confirm-shutdown":
        m.confirm, cmd = m.confirm.Update(msg)
    }
    return m, cmd
}

func (m AppModel) View() string {
    nav := m.nav.View()

    var content string
    switch m.activeView {
    case "list":
        content = m.upstreamList.View()
    case "form-add", "form-edit":
        content = m.form.View()
    case "model-select":
        content = m.modelSelect.View()
    case "confirm-delete", "confirm-shutdown":
        content = m.confirm.View()
    }

    status := m.statusBar.View()
    return lipgloss.JoinVertical(lipgloss.Top, nav, content, status)
}
```

**Component interface:**
```go
// internal/tui/components/component.go

// Component is a simplified interface for TUI sub-models.
// Unlike full tea.Model, components return an Action to signal
// intent to the parent rather than using tea.Cmd.
type Component interface {
    Update(msg tea.Msg) (Component, tea.Cmd)
    View() string
    SetSize(width int)
}

// Action signals from child to parent about state transitions
type Action struct {
    Type    string      // "submit", "cancel", "select", "delete", "mode-change"
    Payload interface{} // e.g., *Upstream for submit
}
```

### Pattern 4: Standard Go Project Layout

**What:** Follow the [official Go module layout](https://go.dev/doc/modules/layout) with `cmd/` for entry points and `internal/` for all non-exportable packages.

**When to use:** When a project exceeds a single flat directory of ~7 files and needs clear boundaries between subsystems.

**Why this over flat layout:** The current flat layout with `package main` everywhere means every file can access every global variable. Moving to `internal/` packages enforces compile-time boundaries -- `internal/tui` cannot import `internal/proxy` (and vice versa) unless you explicitly allow it.

**Trade-offs:** More import paths. Slightly more boilerplate for type passing. But the compiler enforces boundaries that currently only exist by convention.

## Data Flow

### Request Flow (After Refactor)

```
Client Request
    |
    v
+-------------------+
|  http.Server      |  Accept connection
+--------+----------+
         |
         v
+-------------------+
|  Middleware Chain  |  RequestID -> Logging -> Auth
+--------+----------+
         |
         v
+-------------------+
|  ServeMux         |  Route: /v1/messages, /admin/status, /admin/reload
+--------+----------+
         |
         v
+-------------------+
|  handleMessages   |  Read body -> ModelRewrite middleware already handled
+--------+----------+
         |
         v
+-------------------+
|  Retry Loop       |  SelectNext(LB) -> proxyRequest -> isRetryable?
+--------+----------+
         |
         v
+-------------------+
|  Publish Event    |  EventBus.Publish(RequestLogEvent{...})
+--------+----------+
         |
    +----+----+------------+
    |         |            |
    v         v            v
 TUI        SQLite     Stats
(subscribe) (subscribe) (subscribe)
```

### Event Flow (After Refactor)

```
TUI User Action (e.g., "add upstream")
    |
    v
+------------------------+
|  TUI form submits      |  FormComponent detects submit
+----------+-------------+
           |
           v
+------------------------+
|  EventBus.Publish(     |  UpstreamAddedEvent{Upstream: ...}
|    UpstreamAddedEvent) |
+----------+-------------+
           |
     +-----+------+--------------+
     |            |              |
     v            v              v
+---------+ +----------+ +------------+
| Config  | |    LB    | | SharedPool |
|persist  | |AddUpstream| |   .Add    |
+---------+ +----------+ +------------+
```

### Key Data Flows

1. **Request path:** Client -> Middleware Chain (RequestID, Auth, Logging) -> ServeMux -> handleMessages -> Retry Loop -> Upstream -> Response
2. **Usage tracking:** RequestLogEvent published to EventBus -> SQLite subscriber writes async -> Stats subscriber updates counters
3. **Config changes:** TUI publishes event -> Config subscriber calls SaveConfig -> LB subscriber updates LoadBalancer -> Pool subscriber updates SharedUpstreams
4. **Config reload (SIGHUP/Admin):** ConfigReloadRequestEvent -> Config subscriber loads new config -> publishes ConfigReloadedEvent -> all subscribers update from new config

## Integration Points

### New Components vs Modified

| File | Status | What Changes |
|------|--------|-------------|
| `cmd/agent-router/main.go` | **Modified** | Reduced from 273 to ~80 lines; only assembly + wiring |
| `internal/eventbus/bus.go` | **NEW** | ~100 lines; typed channel pub/sub |
| `internal/proxy/handler.go` | **Modified** | Extracted from proxy.go; routing only, no business logic |
| `internal/proxy/retry.go` | **Modified** | Extracted from proxy.go; retry + isRetryable |
| `internal/proxy/balancer.go` | **Modified** | Moved from upstream.go; LoadBalancer + SharedUpstreams |
| `internal/proxy/middleware/auth.go` | **NEW** | ~30 lines; extracted from proxy.go + admin.go |
| `internal/proxy/middleware/logging.go` | **NEW** | ~40 lines; publishes RequestLogEvent |
| `internal/proxy/middleware/requestid.go` | **NEW** | ~15 lines; extracted from proxy.go |
| `internal/proxy/middleware/modelrewrite.go` | **NEW** | ~30 lines; extracted from proxy.go |
| `internal/tui/app.go` | **Modified** | Root model; delegates to components (~150 lines) |
| `internal/tui/components/nav.go` | **NEW** | ~60 lines; extracted from tui.go renderNavigation |
| `internal/tui/components/upstream_list.go` | **NEW** | ~80 lines; extracted from tui.go renderUpstreamList |
| `internal/tui/components/form.go` | **NEW** | ~120 lines; extracted from tui.go form handling |
| `internal/tui/components/model_select.go` | **NEW** | ~60 lines; extracted from tui.go model select |
| `internal/tui/components/confirm.go` | **NEW** | ~40 lines; extracted from tui.go confirm dialog |
| `internal/tui/components/status.go` | **NEW** | ~60 lines; extracted from tui.go renderStatus |
| `internal/tui/styles/theme.go` | **NEW** | ~100 lines; extracted from tui.go style declarations |
| `internal/config/config.go` | **Modified** | Moved from config.go; same logic, new package |
| `internal/storage/sqlite.go` | **Modified** | Moved from usage.go; initDB |
| `internal/storage/usage.go` | **Modified** | Moved from usage.go; worker + stats |
| `internal/upstream/upstream.go` | **Modified** | Consolidated types from config.go + upstream.go |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| TUI <-> Business Logic | EventBus (publish events) | TUI never imports proxy, config, or storage packages |
| Proxy <-> Storage | EventBus (RequestLogEvent) | Proxy publishes, storage subscribes |
| Proxy <-> Config | EventBus (ConfigReloadedEvent) | Config publishes, proxy subscribes |
| TUI <-> Config | EventBus (various upstream events) | TUI publishes, config subscribes |
| Middleware -> Proxy | Direct function call | Middleware wraps handler, same package |

### Dependency Direction (Enforced by Go Compiler)

```
cmd/agent-router
  imports: internal/eventbus, internal/proxy, internal/tui, internal/config, internal/storage

internal/eventbus
  imports: (nothing -- standalone)

internal/proxy
  imports: internal/eventbus, internal/upstream, internal/config

internal/tui
  imports: internal/eventbus, internal/upstream
  DOES NOT import: internal/proxy, internal/config, internal/storage

internal/config
  imports: internal/upstream

internal/storage
  imports: (nothing -- receives events only)

internal/upstream
  imports: (nothing -- pure types)
```

**Key constraint:** `internal/tui` and `internal/proxy` NEVER import each other. They communicate exclusively through the EventBus. This is the primary architectural win.

## Build Order

The refactor must proceed in dependency order. Each phase produces a working build.

### Phase 1: Extract Types (Zero Behavior Change)

1. Create `internal/upstream/upstream.go` -- move `Upstream`, `UpstreamConfig` types
2. Create `internal/config/config.go` -- move `Config`, `ServiceConfig`, `LoadConfig`, `SaveConfig`
3. Create `internal/storage/sqlite.go` + `usage.go` -- move `UsageLog`, `UsageStats`, `initDB`, `StartUsageWorker`
4. Verify: `go build ./...` passes with re-exported types from main package

**Why first:** Types have zero dependencies. Moving them is mechanical and proves the `internal/` structure compiles.

### Phase 2: Extract Proxy Layer (Middleware Chain)

1. Create `internal/proxy/middleware/` -- extract auth, logging, requestid, modelrewrite
2. Create `internal/proxy/retry.go` -- extract retry loop + isRetryable
3. Create `internal/proxy/balancer.go` -- move LoadBalancer + SharedUpstreams
4. Create `internal/proxy/handler.go` -- route assembly with middleware chain
5. Verify: HTTP proxy still works identically, all tests pass

**Why second:** Proxy has no dependency on TUI. Can be refactored and tested in isolation.

### Phase 3: Introduce EventBus

1. Create `internal/eventbus/bus.go` -- typed channel pub/sub
2. Replace direct channels (logChan, usageChan) with EventBus subscriptions
3. Replace TUI callbacks (OnUpstreamAdded, etc.) with EventBus.Publish calls
4. Replace admin.go global state access with EventBus subscribers
5. Verify: All cross-component communication works through events

**Why third:** EventBus requires proxy and storage to be in their own packages first. After this phase, main.go shrinks dramatically.

### Phase 4: TUI Componentization

1. Create `internal/tui/styles/theme.go` -- extract all lipgloss styles
2. Create `internal/tui/components/` -- extract each visual component
3. Create `internal/tui/app.go` -- root model with message routing
4. Remove old `tui.go`
5. Verify: TUI renders identically, all keyboard interactions work

**Why last:** TUI componentization depends on EventBus being in place (components publish events instead of calling callbacks). Can be done incrementally -- one component at a time.

## Scaling Considerations

This is a local tool with a single user. Scaling is not a concern. The architecture is designed for maintainability and extensibility, not throughput.

| Concern | Approach |
|---------|----------|
| Concurrent requests | Already handled: async channels, WAL SQLite |
| TUI responsiveness | Buffered EventBus channels with non-blocking sends |
| Config hot reload | EventBus lets any subscriber react to ConfigReloadedEvent |
| New upstream providers | Add to config; no code changes needed |

## Anti-Patterns

### Anti-Pattern 1: Over-Abstracting the EventBus

**What people do:** Build a generic event bus with string topics, wildcard matching, and async processing pipelines.

**Why it is wrong:** This project has ~8 event types. A generic framework adds complexity with zero benefit.

**Do this instead:** Use typed Go structs as events. Use a simple map of `reflect.Type` to `[]chan Event`. Under 100 lines of code.

### Anti-Pattern 2: Deep Directory Nesting

**What people do:** Create `internal/proxy/middleware/auth/v1/`, `internal/tui/components/forms/upstream/`, etc.

**Why it is wrong:** At ~1890 LOC total, deeply nested directories signal over-engineering. Each directory adds import ceremony.

**Do this instead:** One level of nesting under `internal/`. Sub-packages only where there is a clear boundary (middleware/, components/). Flatten aggressively.

### Anti-Pattern 3: Component Communication via Shared Mutable State

**What people do:** Pass a pointer to a shared `App` struct that every component can read and mutate.

**Why it is wrong:** Recreates the global state problem. Any component can corrupt any other component's state.

**Do this instead:** Components receive data through constructor injection and communicate state changes through the EventBus. Read-only shared data (upstream list) is passed by copy.

### Anti-Pattern 4: Premature cmd/ Split for a Single Binary

**What people do:** Create `cmd/proxy/main.go`, `cmd/admin/main.go`, `cmd/tui/main.go` as separate binaries.

**Why it is wrong:** The TUI IS the primary interface -- there is no use case for running the proxy without the TUI. Multiple binaries add build complexity for no operational benefit.

**Do this instead:** Single `cmd/agent-router/main.go`. Admin API is an HTTP route, not a separate binary.

## Sources

- [Organizing a Go Module -- go.dev official](https://go.dev/doc/modules/layout) (HIGH confidence)
- [Managing Nested Models with Bubble Tea -- Roman Parykin](https://donderom.com/posts/managing-nested-models-with-bubble-tea/) (HIGH confidence -- TUI componentization)
- [Understanding Go Middleware Through net/http](https://beyondthecode.medium.com/understanding-go-middleware-through-net-http-f59c823395fe) (HIGH confidence -- onion model)
- [Monoliths That Scale: Command and Event Buses](https://dev.to/er1cak/monoliths-that-scale-architecting-with-command-and-event-buses-2mp) (MEDIUM confidence -- event bus pattern)
- [Channel vs Callbacks -- r/golang](https://www.reddit.com/r/golang/comments/1rnk5tl/channel_vs_callbacks/) (MEDIUM confidence -- Go community consensus)
- [justinas/alice -- Middleware chaining](https://github.com/justinas/alice) (HIGH confidence -- reference implementation)
- [charmbracelet/bubbles -- TUI component library](https://github.com/charmbracelet/bubbles) (HIGH confidence -- componentization reference)

---
*Architecture research for: Agent Router v2.0 Modular Refactor*
*Researched: 2026-04-05*
