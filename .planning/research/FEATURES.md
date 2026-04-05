# Feature Research: v2.0 Architecture Refactor

**Domain:** Go local API proxy -- internal architecture modernization
**Researched:** 2026-04-05
**Confidence:** HIGH (codebase-verified, pattern research confirmed)

*Note: This document covers ONLY new features for the v2.0 architecture refactor milestone. v1.0 MVP features (proxy pass-through, load balancing, TUI, config persistence, usage tracking) are already delivered and documented in PROJECT.md as Validated.*

## Feature Landscape

### Table Stakes (Maintainability Requires These)

Features expected in any well-structured Go project at this scale. Missing them means the codebase becomes painful to modify.

| Feature | Why Expected | Complexity | Codebase Impact | Notes |
|---------|--------------|------------|-----------------|-------|
| ARCH-01: Standard Go Project Layout | Flat 7-file `package main` with 7 global variables is unmaintainable beyond ~2000 LOC; Go community expects `cmd/internal` split | MEDIUM | Restructures all 7 files into `cmd/agent-router/` + `internal/{config,proxy,upstream,tui,admin,usage}/` | Must be done FIRST -- every other feature depends on new directory boundaries. `internal/` enforces privacy via Go compiler. No `pkg/` needed (no public libraries). |
| TUI-02: TUI Componentization | 837-line `tui.go` with 5+ distinct UI concerns is the largest file; any UI change risks breaking unrelated rendering | MEDIUM | Splits `tui.go` into 6 child models under `internal/tui/`: `app.go` (root router), `nav.go`, `list.go`, `form.go`, `model_select.go`, `status.go` | Uses bubbletea model tree pattern: parent delegates `Update()` and `View()` to children. Communication via typed `tea.Msg`. |
| TUI-01: handleModelSelect Bug Fix | Model selection via `[m]` key only updates upstream model, not the actual proxy handler's model selection logic | LOW | Fix in `tui.go:handleModelSelect()` and/or `main.go:OnUpstreamModelSelected` callback | Should be fixed before componentization to avoid propagating the bug into split components. |

### Differentiators (Architectural Quality)

Features that elevate the codebase from "working MVP" to "extensible platform." Not strictly required, but make future feature additions 10x easier.

| Feature | Value Proposition | Complexity | Codebase Impact | Notes |
|---------|-------------------|------------|-----------------|-------|
| ARCH-02: Go Channel Event Bus | Replaces 6+ hard-coded TUI callbacks in `main.go` (lines 88-136) with decoupled pub/sub. New subscribers (admin API, metrics, future features) plug in without touching existing code. | MEDIUM | New `internal/event/` package. `main.go` callbacks become event handlers. Proxy/TUI/Admin publish events instead of calling shared state directly. | Design: `sync.RWMutex` + `map[string][]chan Event`. Topics: `upstream.added`, `upstream.removed`, `config.changed`, `request.completed`. `Publish()` copies subscriber list before async send to avoid lock contention. |
| ARCH-03: Onion Middleware Chain | `ProxyHandler.ServeHTTP()` (proxy.go:76-123) mixes routing, auth, transform, retry, logging in one method. Middleware chain makes each concern independent and testable. | HIGH | New `internal/middleware/` package. Decomposes `ServeHTTP` into: recovery -> logging -> auth -> request-id -> route -> transform -> retry -> core handler. | Uses `func(http.Handler) http.Handler` composition. Chain built by iterating middlewares in reverse. `context.Context` carries cross-cutting data (request ID, upstream name). No framework needed. |

### Anti-Features (Explicitly Do NOT Build)

Features that seem appealing for a "proper architecture" but would be over-engineering for a local single-user tool.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Full Clean Architecture (entities/use-cases/interfaces) | "Industry standard for maintainable Go" | Adds 3-4 layers of indirection for a 1890 LOC tool. Interface explosion with single implementations. Makes debugging harder, not easier. | Standard Go Project Layout (`cmd/internal` split) gives 80% of the benefit at 20% of the cost. Keep domain logic in `internal/` packages, no abstract interfaces unless there are 2+ implementations. |
| Plugin System / Dynamic Module Loading | "Extensible middleware via plugins" | Go plugins are Linux-only (`plugin` package), fragile across versions, and unnecessary for a tool with 7 files. | Compile-time composition. Middleware chain is just function composition -- add new middleware by importing a package. Zero reflection, zero dynamic loading. |
| External Message Broker (NATS, Redis Pub/Sub) | "Production-grade event bus" | Adds external dependencies, network failure modes, and operational complexity for a LOCAL tool. | Go channel event bus is sufficient. In-process pub/sub with typed channels. No network, no serialization, no external process. If scale ever demands it, swap the `EventBus` interface implementation. |
| gRPC Internal Communication | "Service mesh ready" | This is a single binary, not a distributed system. gRPC adds protobuf definitions, code generation, and complexity for zero benefit. | Direct function calls within the process. Event bus for loose coupling. HTTP for the single external interface (`/v1/messages`). |
| Global State via Dependency Injection Container | "Replace global variables with DI" | Go DI containers (wire, dig) add complexity. For 7 global variables, a simple `App` struct with explicit initialization in `main()` is cleaner. | Refactor globals into an `App` struct during project layout restructure. Pass dependencies explicitly through constructors. No DI framework needed. |

## Feature Dependencies

```
[ARCH-01: Project Layout]
    |
    +--requires--> [Global state -> App struct refactor]
    |                  (7 globals in main.go become App struct fields)
    |
    +--enables--> [TUI-02: TUI Componentization]
    |                 (split tui.go requires internal/tui/ package to exist)
    |
    +--enables--> [ARCH-02: Event Bus]
    |                 (event package needs internal/ directory structure)
    |
    +--enables--> [ARCH-03: Middleware Chain]
    |                 (middleware package needs internal/ directory structure)
    |
    +--enables--> [CONF-01/02/03: Config Hot Reload]
    |                 (clean config package boundaries enable hot reload)
    |
    +--enables--> [ADMIN-01/02: Admin API Routes]
                     (admin package needs internal/ directory structure)

[TUI-01: Model Select Bug Fix]
    +--prerequisite--> [TUI-02: TUI Componentization]
                          (fix bug first, then split file to avoid
                           propagating broken logic into child models)

[ARCH-02: Event Bus]
    +--replaces--> [6 TUI callbacks in main.go:88-136]
    +--replaces--> [direct global state mutation in persistConfig/doReload]

[ARCH-03: Middleware Chain]
    +--extracts--> [Auth logic from proxy.go:101-113]
    +--extracts--> [Admin routing from proxy.go:78-93]
    +--extracts--> [Request ID from proxy.go:116-119]
    +--extracts--> [Logging from proxy.go:179/187/206-207]
    +--extracts--> [Model transform from proxy.go:134-137]
    +--replaces--> [Auth duplication in admin.go:51-61 and admin.go:81-91]

[TUI-02: TUI Componentization]
    +--extracts--> [renderNavigation() -> internal/tui/nav.go]
    +--extracts--> [renderUpstreamList() -> internal/tui/list.go]
    +--extracts--> [renderForm() + handleFormInput() -> internal/tui/form.go]
    +--extracts--> [renderModelSelect() + handleModelSelect() -> internal/tui/model_select.go]
    +--extracts--> [renderConfirmation() + handleConfirm() -> internal/tui/confirm.go]
    +--extracts--> [renderStatus() -> internal/tui/status.go]
    +--keeps--> [app.go as root model (message router, child coordination)]
```

### Dependency Notes

- **ARCH-01 requires Global State Refactor:** The 7 global variables (`db`, `usageChan`, `execPath`, `sharedUpstreams`, `lb`, `proxyHandler`, `cfg` plus the `startTime` and `stats` globals) must be consolidated into an `App` struct before packages can be split. Each `internal/` package receives only the dependencies it needs through constructor injection. This is the single most impactful change -- it determines the API surface of every package.

- **TUI-01 is prerequisite for TUI-02:** The `handleModelSelect` bug (model selection does not propagate to the proxy handler's actual model routing) must be fixed in the monolithic `tui.go` first. If componentization happens first, the bug gets distributed across the `model_select.go` child model AND the parent `app.go`, making it harder to isolate and fix.

- **ARCH-02 replaces main.go callback wiring:** Lines 88-136 of `main.go` contain 6 callback functions (`OnUpstreamAdded`, `OnUpstreamUpdated`, `OnUpstreamDeleted`, `OnUpstreamToggled`, `OnDefaultModelChanged`, `OnUpstreamModelSelected`, `OnReload`). Each callback directly mutates `sharedUpstreams`, `lb`, `cfg`, and `proxyHandler`. The event bus replaces all of these: TUI publishes an `UpstreamAdded` event, and subscribers (upstream manager, config persister, load balancer) react independently.

- **ARCH-03 extracts auth duplication:** `admin.go` duplicates the exact same auth logic at lines 51-61 and 81-91 as `proxy.go` lines 101-113. Middleware chain consolidates this into a single `AuthMiddleware` that runs once per request.

- **ARCH-02 and ARCH-03 are independent:** Event bus addresses TUI-to-backend communication (vertical decoupling). Middleware chain addresses request processing pipeline (horizontal decoupling). They can be implemented in either order, but both benefit from having ARCH-01's directory structure in place first.

## MVP Definition

### Phase 1: Foundation (v2.0-alpha)

Must come first because everything else depends on directory boundaries and explicit dependency passing.

- [ ] **ARCH-01: Standard Go Project Layout** -- Without this, no other package can be cleanly separated. Creates the `internal/` boundary that makes all subsequent refactors safe.
- [ ] **Global State -> App Struct** -- Consolidate 7 global variables into an `App` struct. Each `internal/` package receives dependencies through constructors, not global access.
- [ ] **TUI-01: handleModelSelect Bug Fix** -- Fix the model selection bug while `tui.go` is still a single file. Lowest risk, immediate user-visible improvement.

### Phase 2: Decomposition (v2.0-beta)

With clean package boundaries established, split the two largest files.

- [ ] **TUI-02: TUI Componentization** -- Split 837-line `tui.go` into 6 child models. Each child encapsulates its own state, update logic, and rendering. Parent `app.go` routes messages to children.
- [ ] **ARCH-02: Go Channel Event Bus** -- Replace 6+ hard-coded callbacks with typed event channels. New `internal/event/` package with `Publish()`/`Subscribe()` API. Existing callbacks become event subscribers.
- [ ] **CONF-01/02/03: Config Hot Reload** -- With event bus in place, config reload becomes "publish config.changed event" and subscribers react. SIGHUP, TUI button, and POST /admin/reload all trigger the same path.

### Phase 3: Pipeline (v2.0-rc)

Final architectural improvement to the request processing path.

- [ ] **ARCH-03: Onion Middleware Chain** -- Decompose `ProxyHandler.ServeHTTP()` into composable middleware layers. Extract auth, routing, logging, transform into independent `internal/middleware/` handlers.
- [ ] **ADMIN-01/02: Admin API Routes** -- With middleware chain, admin endpoints get auth middleware for free instead of duplicating auth logic.

### Future Consideration (v2.1+)

Features that the architecture enables but are not part of this milestone.

- [ ] **Streaming proxy support** -- Middleware chain makes it easy to add a streaming middleware that handles SSE/chunked responses without touching the core proxy logic.
- [ ] **Circuit breaker per upstream** -- Event bus enables upstream failure events that a circuit breaker subscriber can consume independently.
- [ ] **Prometheus metrics** -- Middleware chain enables a metrics middleware that records request duration, status codes, upstream distribution. Event bus enables counter gauges from request events.
- [ ] **Structured logging (slog)** -- With middleware chain, add a structured logging middleware that replaces `fmt.Fprintf(os.Stderr, ...)` throughout the codebase.

## Feature Prioritization Matrix

| Feature | Developer Value | Implementation Cost | Priority | Rationale |
|---------|----------------|---------------------|----------|-----------|
| ARCH-01: Project Layout | HIGH | MEDIUM | P1 | Enables everything else. 7 files to restructure, ~1890 LOC to move. |
| Global State -> App Struct | HIGH | MEDIUM | P1 | Required by ARCH-01. 9 globals to consolidate, constructor injection for 6 packages. |
| TUI-01: Model Select Bug Fix | MEDIUM | LOW | P1 | Quick win, immediate user benefit, must happen before TUI-02. |
| TUI-02: TUI Componentization | HIGH | MEDIUM | P2 | 837-line file is biggest maintenance burden. Model tree pattern is well-documented. |
| ARCH-02: Event Bus | HIGH | MEDIUM | P2 | Decouples TUI from backend. 6 callbacks -> event subscribers. |
| CONF-01/02/03: Config Hot Reload | MEDIUM | LOW | P2 | Trivial with event bus. SIGHUP + TUI + Admin all publish same event. |
| ARCH-03: Middleware Chain | MEDIUM | HIGH | P3 | Highest complexity. Must not break proxy behavior. Extensive testing needed. |
| ADMIN-01/02: Admin API Routes | LOW | LOW | P3 | Already functional in proxy.go. Middleware just removes auth duplication. |

**Priority key:**
- P1: Must have first -- other features depend on them
- P2: Should have -- primary value of the v2.0 milestone
- P3: Nice to have -- completes the architecture but highest risk

## Target Directory Structure

```
agent-router/
  cmd/
    agent-router/
      main.go              # Entrypoint: init App, start server, run TUI
  internal/
    app/
      app.go               # App struct (replaces 7 globals)
    config/
      config.go            # Config struct, Load, Save
    event/
      bus.go               # EventBus: Publish, Subscribe, Unsubscribe
      events.go            # Event type definitions
    middleware/
      chain.go             # Chain builder
      auth.go              # Auth middleware
      logging.go           # Request logging middleware
      recovery.go          # Panic recovery middleware
      requestid.go         # Request ID middleware
    proxy/
      proxy.go             # Core proxy handler
      transform.go         # Model name transform
      retry.go             # Retry with backoff logic
      admin.go             # Admin endpoint handlers
    upstream/
      upstream.go          # Upstream struct, SharedUpstreams
      balancer.go          # LoadBalancer (modulo hash)
    tui/
      app.go               # Root model (message router)
      nav.go               # Navigation bar child model
      list.go              # Upstream list child model
      form.go              # Add/Edit form child model
      model_select.go      # Model selection child model
      confirm.go           # Confirmation dialog child model
      status.go            # Status bar child model
      styles.go            # Catppuccin palette + shared styles
    usage/
      usage.go             # UsageLog, UsageStats
      worker.go            # SQLite async worker
  config.yaml
  go.mod
```

## Implementation Risk Assessment

| Feature | Risk | Mitigation |
|---------|------|------------|
| ARCH-01: Project Layout | Import cycle between packages | Keep all cross-package communication through interfaces or the event bus. No `internal/proxy` imports `internal/tui`. |
| TUI-02: Componentization | Breaking message routing when splitting model | Write integration test that exercises all TUI modes before splitting. Verify each child model in isolation. |
| ARCH-02: Event Bus | Goroutine leak from unclosed channels | `Unsubscribe()` must close and drain the channel. Use `context.Context` for cancellation. Test with `go test -race`. |
| ARCH-03: Middleware Chain | Subtle behavior change in request processing | Golden-file test: capture exact request/response pairs from current proxy, verify middleware chain produces identical results. |
| Global State -> App Struct | Initialization order errors | Use a `NewApp()` constructor that validates all dependencies. Fail fast at startup, not runtime. |

## Sources

- **Codebase analysis:** All 7 Go source files read and analyzed (main.go, proxy.go, tui.go, upstream.go, admin.go, config.go, usage.go)
- **Go project layout:** `golang-standards/project-layout` GitHub repository -- community reference, NOT official Go standard
- **Event bus pattern:** Go `sync.RWMutex` + `map[string][]chan Event` pattern from Go community best practices
- **Middleware chain:** `func(http.Handler) http.Handler` composition from `net/http` standard library; Justinas/alice as reference implementation
- **Bubbletea componentization:** Model tree pattern from Charmbracelet documentation and bubbletea examples repository
- **Architecture patterns:** "Let's Go" by Alex Edwards for Go project structure guidance

---

*Feature research for: Agent Router v2.0 Architecture Refactor*
*Researched: 2026-04-05*
*Confidence: HIGH (codebase-verified)*
