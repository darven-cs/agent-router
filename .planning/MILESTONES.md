# Milestones

## v1.0 MVP (Shipped: 2026-04-05)

**Phases completed:** 3 phases, 7 plans, 21 tasks

**Key accomplishments:**

- Working Claude API proxy with modulo-hash load balancing across 3 upstreams and real-time bubbletea TUI displaying service status and request logs
- Automatic retry with exponential backoff (1s/2s/4s), SelectNext failover routing, and retry tracking in RequestLog
- TUI-based upstream CRUD with keyboard navigation, confirmation dialogs, and graceful shutdown with 10s timeout
- SQLite usage tracking via goroutine-channel async worker with UsageLog model storing per-request tokens, latency, and upstream data
- Config hot reload via SIGHUP signal, TUI 'r' key, and POST /admin/reload API - all three triggers invoke identical doReload() function
- GET /admin/status returns comprehensive service status with SQLite aggregation and in-memory stats
- Config write-back via SaveConfig() - TUI add/edit/delete/enable/disable changes persist to config.yaml and survive SIGHUP reload

### Known Gaps

- CONF-01: Reload config on SIGHUP signal — partially implemented
- CONF-02: Reload config via TUI button — partially implemented
- CONF-03: Reload config via POST /admin/reload API — partially implemented
- ADMIN-01: GET /admin/status — implemented but not fully verified
- ADMIN-02: POST /admin/reload — handler exists but routing incomplete

---
