# Phase 4: Foundation Restructure - Context

**Gathered:** 2026-04-05
**Status:** Ready for planning

<domain>
## Phase Boundary

Reorganize 7-file Go monolith (1890 LOC) into Standard Go Project Layout (cmd/ + internal/). Replace all 7+ global variables with App struct using constructor injection. Fix TUI model-select bug by redesigning as "Primary Upstream" selection — user pins a preferred upstream, proxy routes to it with automatic fallback to other upstreams on failure.

**Requirements covered:** ARCH-01 (directory layout), ARCH-02 (App struct replaces globals), TUI-01 (model select fix → primary upstream selection)

**Scope**: Directory restructuring, App struct, primary upstream feature. Event bus, middleware chain, and config hot reload improvements come in Phases 5-6.

</domain>

<decisions>
## Implementation Decisions

### Directory Layout (ARCH-01)
- **D-01:** Target structure: `cmd/agent-router/main.go` + `internal/{config,proxy,tui,upstream,storage,admin}/`
- **D-02:** Split tui.go (837 lines) by responsibility into 5 files: `app.go` (Model struct, NewModel), `update.go` (Update + handlers), `view.go` (View + render methods), `form.go` (form state, validation), `styles.go` (lipgloss colors, style vars)
- **D-03:** admin.go moves to `internal/admin/admin.go` — separate package from proxy, cleaner separation
- **D-04:** usage.go moves to `internal/storage/usage.go` — groups SQLite/GORM concerns
- **D-05:** Remaining files map 1:1 to packages: config.go → internal/config/, proxy.go → internal/proxy/, upstream.go → internal/upstream/

### Primary Upstream / Model Select Fix (TUI-01)
- **D-06:** `[m]` key redesign: shows upstream list + "Auto (hash)" option at top. Selecting an upstream pins it as Primary. Selecting "Auto" returns to hash-based distribution.
- **D-07:** Default behavior: FNV hash load balancing (unchanged from v1.0). Pinning is optional.
- **D-08:** When primary upstream is set: all requests route to primary first. On failure (5xx/timeout/429), auto-fallback to other enabled upstreams using existing exponential backoff retry (1s/2s/4s, max 3 retries).
- **D-09:** Model name transformation: silently replace outgoing model with the upstream's configured model. Claude Code always sees standard model names, upstream receives its configured model.
- **D-10:** TUI status bar shows "Active Upstream: {name}" when pinned, "Auto (hash)" when in distribution mode
- **D-11:** Fallback events appear in TUI log: "[Fallback] {name} failed, trying {next}..."

### App Struct Design (ARCH-02)
- **D-12:** App struct holds all top-level dependencies (cfg, db, lb, proxy, storage). Each package receives dependencies through constructor functions.
- **D-13:** App manages full lifecycle: `NewApp(cfg) *App`, `Run() error`, `Shutdown()`. Signal handling (SIGINT/SIGTERM/SIGHUP) inside App.
- **D-14:** TUI callbacks become App methods. TUI model receives a `Callbacks` struct with function fields, wired by App. This eliminates the 6+ closures in main.go that reference globals.
- **D-15:** No interfaces for now — 1890 LOC tool doesn't need the abstraction. Direct struct dependencies are sufficient.
- **D-16:** All 7 mutable globals eliminated: `db`, `usageChan`, `execPath`, `sharedUpstreams`, `lb`, `proxyHandler`, `cfg` (+ `startTime`, `stats`) — all become App struct fields.

### Migration Strategy
- **D-17:** Bottom-up, one file at a time. Order: config → upstream → storage → admin → proxy → tui → main.go → cmd/
- **D-18:** Each step must compile: `go build ./...` and `go vet ./...` pass after every file move
- **D-19:** Verification: build + vet each step, smoke test at end (start server, send request, check TUI displays correctly)

### Claude's Discretion
- Exact App struct field names and constructor signatures
- Primary upstream state storage (in LoadBalancer or separate field)
- TUI "Auto (hash)" option styling in model-select view
- Fallback log message formatting
- go.mod module path naming

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase Context
- `.planning/phases/01-foundation/01-CONTEXT.md` — Phase 1 decisions, error format, config structure, LoadBalancer API
- `.planning/phases/02-resilience/02-CONTEXT.md` — Phase 2 decisions, retry logic, SharedUpstreams, upstream CRUD callbacks
- `.planning/phases/03-persistence/03-CONTEXT.md` — Phase 3 decisions, async SQLite, SIGHUP reload, admin API, config write-back

### Requirements
- `.planning/REQUIREMENTS.md` — ARCH-01, ARCH-02, TUI-01 requirements for this phase
- `.planning/ROADMAP.md` — Phase 4 success criteria and plan slots

### Tech Stack
- `CLAUDE.md` — Technology stack, constraints, conventions

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `config.go` (67 lines) — LoadConfig, SaveConfig, env expansion. Clean, no cross-file deps except upstream types. Moves directly to internal/config/.
- `upstream.go` (158 lines) — Upstream struct, SharedUpstreams (thread-safe), LoadBalancer (FNV hash, SelectNext). Depends on config types for construction. Moves to internal/upstream/.
- `usage.go` (78 lines) — initDB, GORM model, StartUsageWorker. Self-contained. Moves to internal/storage/.
- `admin.go` (144 lines) — handleAdminStatus, handleAdminReload. References globals (cfg, proxyHandler, startTime). Needs App struct references after migration.
- `proxy.go` (334 lines) — ProxyHandler, ServeHTTP (with admin routing), retry loop, transformModelName, RequestLog struct. Largest logic file. Moves to internal/proxy/.
- `tui.go` (837 lines) — Complete bubbletea model, 6 modes (nav, form, model-select, confirm), 6 callbacks, all lipgloss styles. Splits into 5 files in internal/tui/.
- `main.go` (272 lines) — Entry point, 7 globals, 6 callback closures, persistConfig, doReload, signal handling. Becomes cmd/agent-router/main.go with App struct.

### Established Patterns
- Error responses: `{"error": {"type": "...", "message": "...", "code": N}}`
- TUI uses `tea.NewGoroutine` for concurrent HTTP server
- Config write-back: `persistConfig()` builds config from SharedUpstreams state
- SIGHUP reload: `doReload()` re-reads config.yaml, reinitializes LoadBalancer
- Async writes: goroutine drains channel → GORM writes
- Fallback already exists in proxy.go retry loop: SelectNext after failure

### Integration Points
- **Proxy ↔ Upstream**: LoadBalancer interface — Select/SelectNext/AddUpstream/UpdateUpstream/DeleteUpstream
- **TUI ↔ Proxy**: Callbacks struct (6 functions) + logChan (RequestLog events)
- **TUI ↔ Config**: persistConfig() on upstream changes, doReload() on SIGHUP/r key
- **Storage ← Proxy**: usageChan for async SQLite writes
- **Admin ← Proxy**: handleAdminStatus reads globals — needs App reference after refactor

</code_context>

<specifics>
## Specific Ideas

- User wants "Primary Upstream" concept: manually pin preferred upstream as main, auto-fallback to others if it fails
- [m] should show upstream list with their real models (e.g., "Zhipu (GLM-4)", "Aicodee (MiniMax)")
- TUI status bar should display current active upstream prominently
- Fallback events should be visible in TUI log stream
- Model name replacement should be transparent — Claude Code always sees standard names, upstream gets its configured model

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 04-foundation-restructure*
*Context gathered: 2026-04-05*
