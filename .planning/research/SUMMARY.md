# Project Research Summary

**Project:** Agent Router v2.0 Architecture Refactor
**Domain:** Go local API proxy -- internal architecture modernization
**Researched:** 2026-04-05
**Confidence:** HIGH

## Executive Summary

Agent Router is a local Go API proxy that forwards Claude Code requests to multiple upstream providers (Zhipu, Aicodee, Minimax) with load balancing and failover. The v1.0 MVP is delivered and working: a 7-file flat `package main` codebase at ~1890 LOC. The v2.0 milestone is an architecture refactor that modernizes this flat monolith into a standard Go `cmd/internal` layout with an event bus, middleware chain, and decomposed TUI -- without adding any third-party dependencies.

Research confirms a clear 4-phase migration path, each phase producing a working build. The critical insight is that phase ordering is constrained by hard dependencies: project layout restructuring must come first because every other feature requires `internal/` directory boundaries to exist. The event bus and middleware chain are independent of each other but both require the new layout. TUI componentization should come last because it benefits from the event bus replacing callbacks first. The recommended stack adds zero new dependencies -- all new capabilities (event bus, middleware chain, TUI decomposition) are implemented with Go stdlib primitives and the existing bubbletea library.

The primary risk is import cycles during the initial restructuring. The codebase has 7 global variables and freely shared types across all files. Creating `internal/` packages without careful dependency planning will produce `import cycle not allowed` errors. Prevention requires creating a dependency-free shared types package first, then splitting files one at a time with compilation verification after each move.

## Key Findings

### Recommended Stack

The v2.0 refactor adds zero new third-party dependencies. All new capabilities use Go stdlib primitives (`sync.RWMutex`, `chan Event`, `context.Context`, `func(http.Handler) http.Handler`) and the existing bubbletea v1.3.10 library. This is a deliberate choice aligned with the project constraint of "Go native + minimal third-party libs."

**Core technologies (unchanged from v1.0):**
- Go 1.24.0 + net/http stdlib: HTTP server and client -- production-proven, zero dependencies
- bubbletea v1.3.10 + lipgloss v1.1.0: TUI framework -- Elm architecture with nested model composition
- GORM v1.31.1 + sqlite driver v1.6.0: Usage tracking storage -- already validated in v1.0
- gopkg.in/yaml.v3: Config file parsing -- stable, pure Go

**New capabilities (stdlib only):**
- Event Bus: `sync.RWMutex` + `map[reflect.Type][]chan Event` -- replaces 6 hardcoded callbacks in main.go
- Middleware Chain: `func(http.Handler) http.Handler` composition -- extracts 5 concerns from monolithic ProxyHandler.ServeHTTP
- TUI Decomposition: bubbletea nested model pattern -- splits 837-line tui.go into 6 child components

**Explicitly excluded:** justinas/alice (saves 10 lines, contradicts minimal deps), asaskevich/EventBus (reflection-based, overkill), bubblon (model stack pattern, wrong fit for single-screen layout), charmbracelet/bubbles (opinionated styling conflicts with Catppuccin palette).

### Expected Features

**Must have (table stakes):**
- ARCH-01: Standard Go Project Layout -- flat 7-file `package main` with 7 global variables is unmaintainable beyond ~2000 LOC. Creates `cmd/agent-router/` + `internal/{config,proxy,tui,eventbus,upstream,storage}/` structure
- Global State -> App Struct -- consolidate 9 global variables into explicit dependency injection via constructors. Required by ARCH-01
- TUI-01: handleModelSelect bug fix -- model selection via `[m]` key does not propagate to proxy handler's model routing. Must fix before componentization

**Should have (architectural quality):**
- ARCH-02: Go Channel Event Bus -- replaces 6 hardcoded TUI callbacks with typed pub/sub. New subscribers plug in without touching existing code
- TUI-02: TUI Componentization -- splits 837-line tui.go into 6 child models using bubbletea's nested model pattern
- CONF-01/02/03: Config Hot Reload -- trivial with event bus; SIGHUP, TUI button, and POST /admin/reload all publish the same event

**Defer (v2.1+):**
- ARCH-03: Onion Middleware Chain -- high complexity, decomposes ServeHTTP into composable middleware layers. Highest risk, should come last
- ADMIN-01/02: Admin API Routes -- already functional, middleware just removes auth duplication
- Streaming proxy support, circuit breaker per upstream, Prometheus metrics -- architecture enables these but they are not in scope

### Architecture Approach

The target architecture follows a standard Go modular monolith pattern. A thin `cmd/agent-router/main.go` wires an `App` struct with explicit dependency injection. The `internal/eventbus/` package provides typed channel pub/sub that decouples all subsystems: TUI never imports proxy, proxy never imports TUI. The `internal/proxy/middleware/` package extracts cross-cutting HTTP concerns (auth, logging, request ID, model rewrite) into composable `func(http.Handler) http.Handler` decorators. The `internal/tui/` package decomposes the monolithic TUI into a root model that delegates `Update()` and `View()` to child components following bubbletea's Elm architecture.

**Major components:**
1. `internal/eventbus/` -- Typed pub/sub via Go channels. Replaces direct callbacks. Events are typed Go structs (UpstreamAddedEvent, ConfigReloadedEvent, RequestLogEvent). Subscribers receive events through buffered channels with non-blocking sends
2. `internal/proxy/middleware/` -- Composable HTTP middleware chain. Auth, logging, request ID, and model rewrite each become standalone testable handlers. Chain built by reverse iteration of `func(http.Handler) http.Handler`
3. `internal/tui/components/` -- Six child bubbletea models (nav, upstream_list, form, model_select, confirm, status). Root `app.go` routes messages to active component. All models use value receivers
4. `internal/config/`, `internal/storage/`, `internal/upstream/` -- Pure data and utility packages extracted from existing code. No cross-imports between them

**Key architectural constraint:** `internal/tui` and `internal/proxy` NEVER import each other. They communicate exclusively through the EventBus. This is the primary architectural win of the refactor.

### Critical Pitfalls

1. **Import cycle during restructuring (Pitfall 5)** -- All types currently share `package main` with no boundaries. Creating `internal/` packages without a dependency-free shared types package will immediately produce `import cycle not allowed`. Prevention: create `internal/upstream/` with zero imports first, then split files one at a time with `go build ./...` after each move.

2. **Global state survives the move (Pitfall 6)** -- Moving `var db *gorm.DB` from main.go to `internal/usage/db.go` is not restructuring, it is relocating global state. Prevention: every package receives dependencies through constructor functions. `main()` is the ONLY place that wires dependencies. Verify with `grep -r "^var " internal/` returning zero mutable globals.

3. **Event bus goroutine leaks (Pitfall 1)** -- Every subscriber goroutine leaks if channels are never closed or stop signals are missing. The current codebase already has a latent version of this with `logChan`. Prevention: `context.WithCancel` for every subscriber, `sync.WaitGroup` for tracking, `Close()` method with `closed bool` flag.

4. **Send on closed channel panic during shutdown (Pitfall 2)** -- If Publish() runs concurrently with bus shutdown, the process panics. Go provides no `isClosed()` for channels. Prevention: `sync.RWMutex` + `closed bool` guard in `Publish()`. Never close subscriber channels from publisher side.

5. **Middleware chain breaks on missing next.ServeHTTP() (Pitfall 3)** -- Converting ServeHTTP early returns into middleware without calling `next` silently drops requests. Prevention: strict contract -- each middleware EITHER calls next OR writes complete response, never both. Chain execution test before individual middleware implementation.

## Implications for Roadmap

Based on research, suggested phase structure:

### Phase 1: Foundation (ARCH-01 + Global State + TUI-01)

**Rationale:** Every subsequent feature depends on `internal/` directory boundaries and explicit dependency injection. Must come first.
**Delivers:** Standard Go project layout with `cmd/agent-router/main.go` entry point and `internal/{config,proxy,tui,upstream,storage}/` packages. All 9 global variables consolidated into constructor-injected dependencies. Model select bug fixed.
**Addresses:** ARCH-01, Global State -> App Struct, TUI-01
**Avoids:** Import cycles (Pitfall 5), lingering global state (Pitfall 6), build path breakage (Pitfall 11)
**Key verification:** `go build -o agent-router ./cmd/agent-router && ./agent-router` starts correctly and finds config.yaml

### Phase 2: Decoupling (ARCH-02: Event Bus + Config Hot Reload)

**Rationale:** Event bus is the highest-value architectural improvement -- it decouples TUI from backend and enables all future features to plug in independently. Can be done in parallel with Phase 3 since it touches different files.
**Delivers:** `internal/eventbus/` package with typed channel pub/sub. 6 TUI callbacks replaced with event subscriptions. Config hot reload via events. Main.go shrinks from ~270 lines to ~80 lines.
**Addresses:** ARCH-02, CONF-01/02/03
**Avoids:** Goroutine leaks (Pitfall 1), send on closed channel (Pitfall 2), event ordering bugs (Pitfall 9), duplicate callback+event (Pitfall 10)
**Key verification:** `go test -race` passes, `runtime.NumGoroutine()` stable after config reload

### Phase 3: Request Pipeline (ARCH-03: Middleware Chain + Admin Routes)

**Rationale:** Middleware chain is the highest-complexity feature and must not break proxy behavior. It is independent of the event bus (different coupling axis -- horizontal request processing vs vertical component communication). Should come after event bus to avoid compounding risk.
**Delivers:** `internal/proxy/middleware/` package with composable HTTP handlers. Auth duplication eliminated. Request processing becomes independently testable per concern.
**Addresses:** ARCH-03, ADMIN-01/02
**Avoids:** Missing next.ServeHTTP() (Pitfall 3), wrong middleware order (Pitfall 4), retry re-executing auth (integration gotcha)
**Key verification:** Golden-file test capturing current proxy request/response pairs, verify identical behavior

### Phase 4: TUI Decomposition (TUI-02)

**Rationale:** Should come last because it benefits from event bus replacing callbacks first. Without the event bus, decomposed TUI components would still need callback references to the backend. With the event bus, components publish events and are fully decoupled.
**Delivers:** 6 child bubbletea models replacing 837-line tui.go. Root `app.go` routes messages to active component. Each component owns its own state.
**Addresses:** TUI-02
**Avoids:** Value receiver violations (Pitfall 7), WindowSizeMsg not forwarded (Pitfall 8)
**Key verification:** Terminal resize to 40x10 produces correct layout, all keyboard interactions work identically

### Phase Ordering Rationale

- Phase 1 is a hard prerequisite: every `internal/` package needs the directory structure to exist. Without it, no other package can be cleanly separated.
- Phases 2 and 3 are independent of each other (event bus addresses vertical decoupling, middleware addresses horizontal request processing) but both depend on Phase 1.
- Phase 4 depends on Phase 2: TUI components should publish events rather than holding callback references. Componentization without the event bus would propagate the callback coupling into child models.
- Phase 3 is placed before Phase 4 because middleware chain risk should be isolated and verified before the large TUI refactor.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 3 (Middleware Chain):** Complex integration -- must verify that retry logic does not re-execute auth, and that admin routes integrate correctly with the middleware chain. Golden-file testing approach needs definition.
- **Phase 4 (TUI Decomposition):** bubbletea nested model message routing has subtle gotchas (value receivers, WindowSizeMsg forwarding). While patterns are documented, the specific decomposition of this 837-line model needs careful planning.

Phases with standard patterns (skip research-phase):
- **Phase 1 (Project Layout):** Well-documented Go convention. Mechanical file moves with compilation verification.
- **Phase 2 (Event Bus):** Go channel pub/sub is a well-established pattern. Implementation details (lifecycle, ordering) are covered in pitfalls research.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All versions verified from go.mod. Zero new dependencies confirmed. All "what not to use" decisions have clear rationale. |
| Features | HIGH | Feature list is codebase-derived, not aspirational. Every feature maps to specific lines in existing files. Dependencies between features are explicit. |
| Architecture | HIGH | Target architecture follows standard Go patterns (cmd/internal layout, middleware chain, event bus). Component boundaries and dependency directions are explicit and compiler-enforced. |
| Pitfalls | HIGH | 11 pitfalls identified from codebase analysis and Go community patterns. Each has specific warning signs and verification steps. Pitfall-to-phase mapping is complete. |

**Overall confidence:** HIGH

### Gaps to Address

- **Event ordering semantics:** Research recommends synchronous dispatch for CRUD operations but does not fully specify which events must be ordered. During Phase 2 planning, define a complete ordered vs unordered event taxonomy.
- **Migration granularity for callbacks:** Research recommends migrating one callback at a time but the specific migration order matters (e.g., OnReload should migrate first since it is simplest). Define during Phase 2 planning.
- **Test infrastructure:** Current codebase has no automated tests. Each phase should establish testing patterns for its domain. Phase 1 should include a basic test framework setup.
- **Bubbletea version compatibility for nested models:** Research assumes bubbletea v1.3.10 supports the nested model pattern correctly. Verify with a small spike during Phase 4 planning if any API uncertainty arises.

## Sources

### Primary (HIGH confidence)
- go.dev official documentation -- Go module layout (`cmd/internal` conventions), net/http middleware patterns
- charmbracelet/bubbletea GitHub -- nested model composition patterns, Elm architecture
- Existing codebase analysis -- all 7 Go source files read and analyzed (main.go 272 LOC, proxy.go 334 LOC, tui.go 837 LOC, upstream.go 158 LOC, config.go 67 LOC, usage.go 78 LOC, admin.go 144 LOC)
- go.mod dependency verification -- actual versions confirmed (Go 1.24.0, bubbletea v1.3.10, lipgloss v1.1.0, GORM v1.31.1)
- justinas/alice GitHub -- reference implementation for middleware chaining

### Secondary (MEDIUM confidence)
- Alex Edwards "Making and Using HTTP Middleware" -- Chain() pattern
- Roman Parykin "Managing Nested Models with Bubble Tea" -- TUI componentization pattern
- Go community sources (r/golang, go101.org) -- channel closing principles, goroutine leak prevention
- golang-standards/project-layout -- community reference for directory conventions

### Tertiary (LOW confidence)
- Blog posts on middleware gotchas -- missing next.ServeHTTP() patterns
- Community event bus patterns for Go -- typed channel pub/sub variations

---
*Research completed: 2026-04-05*
*Ready for roadmap: yes*
