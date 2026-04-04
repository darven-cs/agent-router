---
phase: 03-persistence
plan: "02"
subsystem: infra
tags: [sighup, config-reload, hot-reload, admin-api]

# Dependency graph
requires:
  - phase: 03-01
    provides: SQLite usage tracking, async writes
provides:
  - SIGHUP signal handler for config hot reload
  - TUI 'r' key trigger for config hot reload
  - POST /admin/reload API endpoint
  - doReload() function shared by all three triggers
affects:
  - Phase 03-03 (admin API completion)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Single doReload() function invoked by SIGHUP, TUI, and API"
    - "Thread-safe reload via mutex-protected SharedUpstreams"
    - "LoadBalancer replaced atomically on reload"

key-files:
  created: []
  modified:
    - main.go - doReload function, SIGHUP handler, OnReload callback
    - tui.go - ReloadRequest/ReloadComplete messages, OnReload field, 'r' key handler
    - admin.go - handleAdminReload function (handleAdminStatus already existed)
    - proxy.go - routing for /admin/reload (already existed)

key-decisions:
  - "All three triggers (SIGHUP, TUI 'r', POST /admin/reload) invoke identical doReload() function"
  - "TUI changes persist only in-memory - they survive reload because SharedUpstreams and LoadBalancer are reinitialized from config but TUI changes update those in-memory structures directly"

patterns-established:
  - "Pattern: Channel-based async notification (ReloadComplete via tea.Cmd)"
  - "Pattern: Package-level vars for shared state (lb, proxyHandler, sharedUpstreams, cfg, execPath)"

requirements-completed: [CONF-01, CONF-02, CONF-03]

# Metrics
duration: 10min
completed: 2026-04-04
---

# Phase 03-02: Config Hot Reload Summary

**Config hot reload via SIGHUP signal, TUI 'r' key, and POST /admin/reload API - all three triggers invoke identical doReload() function**

## Performance

- **Duration:** 10 min
- **Started:** 2026-04-04T04:31:45Z
- **Completed:** 2026-04-04T04:42:00Z
- **Tasks:** 3
- **Files modified:** 4

## Accomplishments
- SIGHUP signal triggers config reload and prints "Config reloaded successfully"
- TUI 'r' key triggers reload with visual feedback via ReloadComplete message
- curl -X POST http://localhost:{port}/admin/reload returns {"status":"reloaded"}
- All three use identical doReload() function that re-reads config.yaml and reinitializes LoadBalancer

## Task Commits

Each task was committed atomically:

1. **Task 1: Create doReload() function and SIGHUP handler** - (already present in main.go from prior work)
2. **Task 2: Add TUI reload button and ReloadRequest/ReloadComplete messages** - `2260448` (feat)
3. **Task 3: Add POST /admin/reload endpoint** - `2260448` (feat) (same commit as Task 2)

**Plan metadata:** `2260448` (docs: complete plan)

## Files Created/Modified
- `main.go` - doReload function, SIGHUP handler, OnReload callback (already present)
- `tui.go` - ReloadRequest/ReloadComplete types, OnReload field, 'r' key handler, ReloadComplete handler, help text
- `admin.go` - handleAdminReload function updated to call doReload synchronously
- `proxy.go` - routing for /admin/reload (already present)

## Decisions Made
- All three triggers invoke identical doReload() function - ensures consistent behavior regardless of trigger source
- TUI changes (add/edit/delete/enable/disable) persist only in-memory and survive reload because they update SharedUpstreams and LoadBalancer directly, while reload reinitializes from config.yaml

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- admin.go already existed with a different doReload implementation - reconciled by keeping doReload in main.go and updating handleAdminReload to call the main.go version
- TUI Update function doesn't have direct access to tea.Program - resolved using tea.Cmd return pattern

## Next Phase Readiness
- Config hot reload is complete and working
- Ready for Phase 03-03: Admin API endpoints /admin/status and /admin/reload completion
- Note: /admin/status and /admin/reload routing already exist in proxy.go

---
*Phase: 03-persistence*
*Completed: 2026-04-04*
