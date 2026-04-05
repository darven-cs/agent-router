# Roadmap: Agent Router

## Milestones

- ✅ **v1.0 MVP** — Phases 1-3 (shipped 2026-04-05)
- 🚧 **v2.0 Architecture Refactor** — Phases 4-6 (in progress)

## Phases

<details>
<summary>✅ v1.0 MVP (Phases 1-3) — SHIPPED 2026-04-05</summary>

- [x] Phase 1: Foundation (1/1 plans) — completed 2026-04-04
- [x] Phase 2: Resilience (2/2 plans) — completed 2026-04-04
- [x] Phase 3: Persistence (4/4 plans) — completed 2026-04-04

</details>

- [ ] **Phase 4: Foundation Restructure** — Go project layout, App struct, model select bug fix
- [ ] **Phase 5: Event-Driven Decoupling** — Event bus, TUI componentization, config hot reload
- [ ] **Phase 6: Request Pipeline** — Middleware chain, admin API with shared auth

## Phase Details

### Phase 4: Foundation Restructure
**Goal**: Developer works with a well-organized modular codebase where every dependency is explicit and the model select bug is fixed
**Depends on**: v1.0 (shipped)
**Requirements**: ARCH-01, ARCH-02, TUI-01
**Success Criteria** (what must be TRUE):
  1. Developer can run `go build ./cmd/agent-router` and get a working binary that starts and serves requests identically to v1.0
  2. Developer can find any feature by domain in `cmd/` and `internal/{config,proxy,tui,upstream,storage}/` directories without guessing which file holds it
  3. All 7 global variables are eliminated — every package receives dependencies through constructor functions, verified by `grep -r "^var " internal/` returning zero mutable globals
  4. User can press [m] in TUI to select an upstream model and the proxy immediately routes requests using that model
**Plans**: 3 plans

Plans:
- [x] 04-01: Leaf package migration (config, upstream, storage) + primary upstream extension
- [x] 04-02: Proxy/admin migration + App struct replacing all globals
- [ ] 04-03: TUI split + primary upstream feature + final cmd/ layout + human verify

### Phase 5: Event-Driven Decoupling
**Goal**: Subsystems communicate through typed events instead of hardcoded callbacks, TUI is decomposed into independent components, and config reload works from all three triggers
**Depends on**: Phase 4
**Requirements**: ARCH-03, ARCH-04, TUI-02, TUI-03, TUI-04, CONF-01, CONF-02, CONF-03
**Success Criteria** (what must be TRUE):
  1. TUI publishes events (upstream added/edited/deleted, config reload) to EventBus instead of calling business logic directly — all 6 callbacks replaced
  2. A new subscriber (e.g., admin API or metrics collector) can plug into events by calling `bus.Subscribe()` without modifying any existing package
  3. Developer can modify one TUI child component (nav, list, form, model-select, confirm, status) without reading or touching any other component file
  4. User can reload config via SIGHUP signal, TUI [r] key, or POST /admin/reload — all three triggers produce identical config.changed events and all subscribers react correctly
  5. Goroutine count stays stable after repeated config reloads (no leaks), verified by `runtime.NumGoroutine()` before and after 10 reload cycles
**Plans**: TBD

Plans:
- [ ] 05-01: TBD
- [ ] 05-02: TBD
- [ ] 05-03: TBD

### Phase 6: Request Pipeline
**Goal**: HTTP request processing is composed from independent middleware layers that produce byte-identical behavior to the monolithic proxy, and admin endpoints use shared auth
**Depends on**: Phase 4
**Requirements**: ARCH-05, ARCH-06, ADMIN-01, ADMIN-02
**Success Criteria** (what must be TRUE):
  1. Proxy request processing is composed from independent middleware layers (auth, logging, recovery, request-id, transform) that can be individually tested
  2. Middleware chain produces byte-identical HTTP request/response behavior to the current monolithic ServeHTTP() — verified by golden-file comparison
  3. GET /admin/status returns comprehensive service status without duplicating auth logic (uses shared auth middleware)
  4. POST /admin/reload triggers config reload without duplicating auth logic (uses shared auth middleware)
**Plans**: TBD

Plans:
- [ ] 06-01: TBD
- [ ] 06-02: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 4 → 5 → 6

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Foundation | v1.0 | 1/1 | Complete | 2026-04-04 |
| 2. Resilience | v1.0 | 2/2 | Complete | 2026-04-04 |
| 3. Persistence | v1.0 | 4/4 | Complete | 2026-04-04 |
| 4. Foundation Restructure | v2.0 | 0/3 | Not started | - |
| 5. Event-Driven Decoupling | v2.0 | 0/? | Not started | - |
| 6. Request Pipeline | v2.0 | 0/? | Not started | - |
