# Phase 2: Resilience - Context

**Gathered:** 2026-04-04 (auto mode)
**Status:** Ready for planning

<domain>
## Phase Boundary

Claude Code requests never fail due to upstream issues — automatic failover with exponential backoff retry, plus full TUI upstream management (add/edit/delete). TUI displays real-time failover events. Graceful shutdown with confirmation.

**Scope**: Failover logic, retry state in TUI, upstream CRUD via keyboard. Usage tracking and config hot reload come in Phase 3.

</domain>

<decisions>
## Implementation Decisions

### Failover Behavior (FAIL-01, FAIL-02, FAIL-03, FAIL-04)
- **D-01:** Retry trigger: 5xx responses OR timeout (context deadline exceeded) — NOT 4xx except 429
- **D-02:** Exponential backoff: 1s → 2s → 4s delays between retries
- **D-03:** Maximum 3 retries per request (4 total attempts including initial)
- **D-04:** After all retries exhausted, return error code 1001 with `{"error": {"type": "upstream_error", "message": "All upstreams failed", "code": 1001}}`
- **D-05:** If no upstreams enabled at all, return error code 1001 immediately without retry

### Failover State Tracking
- **D-06:** Add `RetryAttempt int` and `RetryCount int` fields to `RequestLog` struct
- **D-07:** Log entry written for each retry attempt with upstream name and retry number
- **D-08:** Final failure log entry shows all retries that were attempted

### TUI Upstream Management (TUI-05, TUI-06, TUI-07)
- **D-09:** Press `a` to enter "add upstream" inline form mode
- **D-10:** Press `e` to edit selected upstream (or Enter on selected)
- **D-11:** Press `d` to delete selected upstream with "Press Enter to confirm" prompt
- **D-12:** Upstream form fields: Name, URL, API Key, Auth Type (bearer/x-api-key), Timeout (seconds), Enabled (toggle)
- **D-13:** Form validation: Name and URL required, URL must be valid http/https, Timeout minimum 5s
- **D-14:** TUI maintains mutable `upstreams` slice — changes take effect immediately in LoadBalancer

### TUI Keyboard Navigation (TUI-08)
- **D-15:** Arrow keys (↑/↓) navigate upstream list when in navigation mode
- **D-16:** `a`/`e`/`d` keys trigger actions on selected upstream (when not in form mode)
- **D-17:** `Esc` cancels current form or returns to navigation mode
- **D-18:** `q` or `ctrl+c` initiates graceful shutdown confirmation (not immediate quit)

### Graceful Shutdown (TUI-09)
- **D-19:** Press `q` or `ctrl+c` → show confirmation dialog: "Shutdown? [y/n]"
- **D-20:** `y` or `Enter` confirms shutdown, any other key cancels
- **D-21:** On confirm: stop accepting new requests, wait for in-flight requests (max 10s timeout), then exit

### Integration with Existing Code
- **D-22:** `ProxyHandler` gains `SelectNext()` method on LoadBalancer to get next upstream after failure
- **D-23:** `ProxyHandler` gains retry loop with exponential backoff using `time.Sleep`
- **D-24:** TUI `Update()` handles new message types: `UpstreamAdded`, `UpstreamDeleted`, `UpstreamUpdated`
- **D-25:** Shared config state protected by mutex for concurrent reads during TUI edits

### Claude's Discretion
- Exact lipgloss colors for retry state (e.g., warning color for mid-retry)
- Form input field ordering and default values
- Whether to show retry attempts as separate log lines or aggregated
- Confirmation dialog styling (border, placement)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase Context
- `.planning/phases/01-foundation/01-CONTEXT.md` — Phase 1 decisions, error format, LoadBalancer API
- `.planning/REQUIREMENTS.md` — FAIL-01 through FAIL-04, TUI-05 through TUI-09
- `.planning/ROADMAP.md` — Phase 2 success criteria

### Tech Stack
- `.planning/CLAUDE.md` §Technology Stack — Go native net/http, bubbletea, lipgloss patterns
- `.claude/get-shit-done/templates/context.md` — CONTEXT.md format reference

</canonical_refs>

<codebase>
## Existing Code Insights

### Reusable Assets
- `proxy.go:ProxyHandler` — Already has `ServeHTTP`, `proxyRequest`, error format. Needs retry loop.
- `proxy.go:LoadBalancer` — Has `Select(hashInput)` and `GetEnabled()`. Needs `SelectNext(after *Upstream)`.
- `proxy.go:RequestLog` — Already has Timestamp, LatencyMs, UpstreamName, StatusCode, RequestID. Needs RetryAttempt.
- `upstream.go:Upstream` struct — Already has all fields needed for management CRUD.
- `tui.go:model` — Has `upstreams []*Upstream` and `logs []RequestLog`. Needs selection index and form state.

### Established Patterns
- TUI uses `tea.NewGoroutine` for concurrent HTTP server
- Lipgloss styles defined at package level (stylePrimary, styleError, etc.)
- Error responses: `{"error": {"type": "...", "message": "...", "code": N}}`
- Log buffer: capped at 50 entries, FIFO

### Integration Points
- **Proxy ↔ Config**: ProxyHandler receives LoadBalancer at construction — needs refresh after upstream edits
- **TUI ↔ Proxy**: Communication via `logChan chan RequestLog` — needs bidirectional channel for upstream changes
- **Config hot reload** (Phase 3): Config changes from TUI should eventually persist, but Phase 2 only keeps in-memory

</codebase>

<specifics>
## Specific Ideas

No specific references yet — open to standard approaches for Phase 2 resilience features.

</specifics>

<deferred>
## Deferred Ideas

None — Phase 2 scope is well-defined.

### Reviewed Todos (not folded)

No pending todos to review.

</deferred>

---

*Phase: 02-resilience*
*Context gathered: 2026-04-04*
