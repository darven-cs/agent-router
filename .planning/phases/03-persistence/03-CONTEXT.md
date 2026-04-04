# Phase 3: Persistence - Context

**Gathered:** 2026-04-04
**Status:** Ready for planning

<domain>
## Phase Boundary

Usage data persisted to SQLite, config hot reload via SIGHUP/TUI/API, admin API for operations. Phase 2 established failover and TUI upstream management — Phase 3 adds persistence layer and runtime config changes.

**Scope**: SQLite usage tracking (async writes), config hot reload (SIGHUP + TUI button + API), admin endpoints (/admin/status, /admin/reload), dynamic upstream changes (add/edit/delete/enable/disable at runtime). TUI changes persist only in-memory (file persistence in future phase).

</domain>

<decisions>
## Implementation Decisions

### SQLite Schema & ORM (USAGE-04)
- **D-01:** Use GORM v1.25.x with gorm.io/driver/sqlite for SQLite operations
- **D-02:** Per-request log table: timestamp, request_id, upstream_name, input_tokens, output_tokens, latency_ms, status_code
- **D-03:** Index on timestamp and upstream_name for efficient queries

### Usage Tracking Model (USAGE-01, USAGE-02, USAGE-03)
- **D-04:** Full per-request logging: input_tokens, output_tokens, latency_ms, upstream_name, status_code
- **D-05:** Asynchronous writes via goroutine channel (per STATE.md prior decision: "SQLite writes will be async via goroutine channel")
- **D-06:** Usage data survives service restart (persisted to usage.db)

### Async Write Implementation (USAGE-05)
- **D-07:** ProxyHandler logs to a goroutine channel, background worker drains and writes to SQLite
- **D-08:** SQLite writes do NOT block HTTP response (async, fire-and-forget with error logging)

### Config Hot Reload (CONF-01, CONF-02, CONF-03)
- **D-09:** SIGHUP signal triggers config reload: re-read config.yaml, reinitialize LoadBalancer
- **D-10:** TUI button click triggers same reload function
- **D-11:** POST /admin/reload triggers same reload function
- **D-12:** All three triggers (SIGHUP, TUI, API) invoke identical reload logic

### Dynamic Upstream Changes (CONF-04, CONF-05, CONF-06)
- **D-13:** TUI add/edit/delete/enable/disable changes take effect immediately in LoadBalancer
- **D-14:** TUI changes are persisted to config.yaml via SaveConfig() so they survive SIGHUP reload (overridden 2026-04-04 — previously runtime-only)
- **D-15:** New upstreams from TUI are added to SharedUpstreams and LoadBalancer immediately
- **D-16:** Deleted upstreams are removed from SharedUpstreams and LoadBalancer immediately

### Admin API (ADMIN-01, ADMIN-02)
- **D-17:** GET /admin/status returns: service_name, version, uptime, total_requests, total_tokens_in, total_tokens_out, per_upstream_counts, enabled_channels list
- **D-18:** POST /admin/reload triggers hot config reload (same as SIGHUP)
- **D-19:** Admin endpoints use same authentication as /v1/messages (x-api-key or Bearer token)

### Claude's Discretion
- Exact GORM model struct field names and tags
- Channel buffer size for async writes (default unbounded, log errors if full)
- TUI reload button placement and styling
- /admin/status JSON response structure细节
- Error handling when config.yaml is missing/corrupt on reload

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase Context
- `.planning/phases/01-foundation/01-CONTEXT.md` — Phase 1 decisions, error format, config structure
- `.planning/phases/02-resilience/02-CONTEXT.md` — Phase 2 decisions, SharedUpstreams, retry state
- `.planning/REQUIREMENTS.md` — USAGE-01 through USAGE-05, CONF-01 through CONF-06, ADMIN-01, ADMIN-02
- `.planning/ROADMAP.md` — Phase 3 success criteria

### Tech Stack
- `.planning/CLAUDE.md` §Technology Stack — GORM v1.25.x, sqlite driver, fsnotify patterns
- `.claude/get-shit-done/templates/context.md` — CONTEXT.md format reference

</canonical_refs>

<codebase>
## Existing Code Insights

### Reusable Assets
- `proxy.go:ProxyHandler` — Already has logChan for RequestLog, needs async usage logging
- `proxy.go:RequestLog` — Has Timestamp, LatencyMs, UpstreamName, StatusCode, RequestID, RetryAttempt, RetryCount — needs input/output tokens
- `upstream.go:LoadBalancer` — Has SelectNext, GetEnabled, AddUpstream, UpdateUpstream, DeleteUpstream
- `upstream.go:SharedUpstreams` — Thread-safe upstream state for TUI ↔ Proxy communication
- `config.go:LoadConfig` — Existing config loading with env expansion

### Established Patterns
- TUI uses `tea.NewGoroutine` for concurrent HTTP server
- Lipgloss styles defined at package level
- Error responses: `{"error": {"type": "...", "message": "...", "code": N}}`
- Config file: `config.yaml` via `os.ExpandEnv`
- Graceful shutdown: max 10s wait for in-flight requests

### Integration Points
- **Proxy → SQLite**: ProxyHandler logs to channel, background goroutine writes to GORM
- **SIGHUP → Config**: signal.Notify for SIGHUP, re-invoke LoadConfig, reinitialize LoadBalancer
- **TUI → Proxy**: SharedUpstreams already exists for TUI edits, needs reload notification
- **Admin → Proxy**: New HTTP handler mux for /admin/* endpoints

</codebase>

<specifics>
## Specific Ideas

No specific references yet — open to standard approaches for Phase 3 persistence features.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

### Reviewed Todos (not folded)

No pending todos to review.

</deferred>

---

*Phase: 03-persistence*
*Context gathered: 2026-04-04*
