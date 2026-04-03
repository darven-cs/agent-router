# Phase 1: Foundation - Context

**Gathered:** 2026-04-03 (auto mode)
**Status:** Ready for planning

<domain>
## Phase Boundary

Working API proxy that routes Claude SDK requests to a single upstream provider with basic TUI status display. Service exposes POST /v1/messages compatible with Claude official SDK. Users can view service status and request logs in real-time via TUI.

**Scope**: Single upstream routing, basic TUI (status + logs). Failover, usage tracking, and hot reload come in later phases.

</domain>

<decisions>
## Implementation Decisions

### Project Structure
- **D-01:** Single `main.go` file for Phase 1 — keeps it simple and fast to iterate
- **D-02:** All code in root package (no sub-packages for Phase 1)
- **D-03:** `config.yaml` alongside the binary for configuration
- **D-04:** Standard Go project layout: `main.go`, `config.go`, `proxy.go`, `tui.go`, `upstream.go`

### HTTP Handler Architecture
- **D-05:** Use `net/http` standard handler with `context.Context` for timeouts
- **D-06:** POST `/v1/messages` — exact path match, not prefix
- **D-07:** Authentication via `x-api-key` or `Authorization: Bearer` header
- **D-08:** Pass through request body to upstream as-is (don't parse/reshape)
- **D-09:** Return upstream response as-is with preserved `Content-Type`

### Config Management
- **D-10:** Config file: `config.yaml` in same directory as binary (resolved via `os.Executable()`)
- **D-11:** Environment variable expansion in config values using `os.ExpandEnv()`
- **D-12:** Config struct with clear field names matching YAML keys
- **D-13:** At least 3 upstreams pre-configured in default config (Zhipu, Aicodee, Minimax)

### TUI Architecture
- **D-14:** Bubbletea `tea.NewGoroutine` model — HTTP server runs in background goroutine
- **D-15:** TUI is the main entry point, starts HTTP server on configured port
- **D-16:** TUI displays: service name, version, port, uptime counter, upstream list, request log
- **D-17:** Request log shows: timestamp, latency (ms), upstream name, status code
- **D-18:** TUI does NOT manage upstreams in Phase 1 (TUI management comes in Phase 2)

### Upstream Routing (Load Balancing)
- **D-19:** Modulo hash algorithm: `hash(request_id) % len(enabled_upstreams)`
- **D-20:** Hash input: request ID from `x-request-id` header, or client IP if missing
- **D-21:** All enabled upstreams participate in routing even if only one is configured
- **D-22:** If no upstreams enabled, return error code 1001 immediately

### Error Response Format
- **D-23:** On auth failure: return `{"error": {"type": "authentication_error", "message": "Invalid API key"}}` with 401
- **D-24:** On upstream failure (no failover in Phase 1): return `{"error": {"type": "upstream_error", "message": "...", "code": 1001}}` with 502
- **D-25:** On timeout: return same format with 504

### Claude's Discretion
- Exact HTTP header forwarding list (which headers to pass through vs filter)
- TUI color scheme and layout details
- Log buffer size (how many requests to keep in view)
- Uptime display format (hours:minutes:seconds vs seconds counter)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

This is a greenfield project — no external specs or ADRs exist yet. All requirements are captured in the decisions above and in `.planning/REQUIREMENTS.md`.

### Project Requirements
- `.planning/REQUIREMENTS.md` — CORE-01, CORE-02, CORE-03, CORE-04, UPST-01 through UPST-04, LB-01, LB-02, LB-03, TUI-01 through TUI-04
- `.planning/ROADMAP.md` — Phase 1 success criteria

### Tech Stack
- `.planning/CLAUDE.md` §Technology Stack — Go native net/http, GORM, bubbletea, lipgloss (library versions and patterns)
- `.claude/get-shit-done/templates/context.md` — CONTEXT.md format reference

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

None — this is a greenfield project. No existing Go code to reuse.

### Established Patterns

None yet — patterns will emerge from Phase 1 implementation and carry forward to Phases 2 and 3.

### Integration Points

- **TUI ↔ HTTP Server**: Communication via shared state (channel or mutex-protected struct)
- **Config → Proxy**: Config struct passed to proxy handler at startup
- **Upstream → Proxy**: Upstream URLs and credentials from config

</code_context>

<specifics>
## Specific Ideas

No specific references yet — open to standard approaches for Phase 1.

</specifics>

<deferred>
## Deferred Ideas

None — Phase 1 scope is well-defined.

### Reviewed Todos (not folded)

No pending todos to review.

</deferred>

---

*Phase: 01-foundation*
*Context gathered: 2026-04-03*
