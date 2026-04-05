# Requirements: Agent Router v2.0

**Defined:** 2026-04-05
**Core Value:** Claude Code 请求永不中断 — 多上游自动切换保障可用性，负载均衡优化成本

## v2.0 Requirements

Requirements for Architecture Refactor milestone. Each maps to roadmap phases.

### Architecture

- [ ] **ARCH-01**: Developer can find code organized by domain in cmd/ and internal/ directories (Standard Go Project Layout)
- [ ] **ARCH-02**: App struct replaces all 7 global variables, each package receives dependencies through constructors
- [ ] **ARCH-03**: TUI publishes events to EventBus instead of calling business logic directly (6 callbacks replaced)
- [ ] **ARCH-04**: New subscribers (admin API, metrics) can plug into events without touching existing code
- [ ] **ARCH-05**: Proxy request processing is composed from independent middleware layers (auth, logging, recovery, request-id, transform)
- [ ] **ARCH-06**: Middleware chain produces byte-identical HTTP behavior to current monolithic ServeHTTP()

### TUI

- [ ] **TUI-01**: User can select upstream model via [m] key and proxy immediately uses that model for routing
- [ ] **TUI-02**: Developer can modify one TUI component (nav, list, form, model-select, confirm, status) without touching others
- [ ] **TUI-03**: Each TUI child model encapsulates its own state, update logic, and rendering independently
- [ ] **TUI-04**: Parent app.go routes messages to correct child model based on current visual mode

### Config

- [ ] **CONF-01**: User can reload config via SIGHUP signal and all subscribers react to config.changed event
- [ ] **CONF-02**: User can reload config via TUI 'r' key and all subscribers react to config.changed event
- [ ] **CONF-03**: User can reload config via POST /admin/reload and all subscribers react to config.changed event

### Admin

- [ ] **ADMIN-01**: GET /admin/status returns comprehensive service status using shared auth middleware (no auth duplication)
- [ ] **ADMIN-02**: POST /admin/reload triggers config reload using shared auth middleware (no auth duplication)

## Future Requirements (v2.1+)

Deferred to future milestone. Not in current roadmap.

### Streaming & Observability

- **STREAM-01**: Middleware chain supports SSE/chunked streaming responses without touching core proxy logic
- **STREAM-02**: Circuit breaker per upstream via event bus failure events
- **OBS-01**: Prometheus metrics middleware recording request duration, status codes, upstream distribution
- **OBS-02**: Structured logging (slog) replaces fmt.Fprintf(os.Stderr, ...) throughout codebase

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Full Clean Architecture (entities/use-cases/interfaces) | Over-engineering for 1890 LOC tool — cmd/internal split gives 80% benefit at 20% cost |
| Plugin system / dynamic module loading | Go plugins are Linux-only, fragile, unnecessary for 7-file tool |
| External message broker (NATS, Redis Pub/Sub) | Local single-user tool — in-process channel pub/sub is sufficient |
| gRPC internal communication | Single binary, not a distributed system — direct function calls suffice |
| DI container (wire, dig) | 7 globals → App struct is simpler, no framework needed |
| Multi-tenant support | Local single-user tool — no tenant isolation needed |
| OAuth / SSO | Internal tool, API key auth sufficient |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| ARCH-01 | — | Pending |
| ARCH-02 | — | Pending |
| ARCH-03 | — | Pending |
| ARCH-04 | — | Pending |
| ARCH-05 | — | Pending |
| ARCH-06 | — | Pending |
| TUI-01 | — | Pending |
| TUI-02 | — | Pending |
| TUI-03 | — | Pending |
| TUI-04 | — | Pending |
| CONF-01 | — | Pending |
| CONF-02 | — | Pending |
| CONF-03 | — | Pending |
| ADMIN-01 | — | Pending |
| ADMIN-02 | — | Pending |

**Coverage:**
- v2.0 requirements: 15 total
- Mapped to phases: 0
- Unmapped: 15 ⚠️

---
*Requirements defined: 2026-04-05*
*Last updated: 2026-04-05 after initial definition*
