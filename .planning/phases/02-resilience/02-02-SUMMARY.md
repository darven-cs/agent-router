---
phase: 02-resilience
plan: "02"
subsystem: ui
tags: [bubbletea, lipgloss, tui, graceful-shutdown, mutex]

# Dependency graph
requires:
  - phase: 02-resilience-01
    provides: failover with exponential backoff
provides:
  - TUI navigation state (selectedIndex, formMode, confirmMode)
  - Add/edit/delete upstream forms with field navigation
  - SharedUpstreams mutex-protected thread-safe state
  - Graceful shutdown with 10s timeout
affects:
  - Phase 3 (hot config reload via SIGHUP)
  - Phase 3 (usage monitoring via SQLite)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Callback-based TUI-to-main communication (OnUpstreamAdded/Updated/Deleted)
    - Mutex-protected shared state (sync.RWMutex)
    - Graceful shutdown with context.WithTimeout

key-files:
  created: []
  modified:
    - tui.go - TUI model with navigation, forms, and confirmation dialogs
    - main.go - Callback wiring and graceful shutdown
    - upstream.go - SharedUpstreams and LoadBalancer CRUD methods

key-decisions:
  - "Used callback pattern instead of tea.Msg for TUI-to-main communication (tea.NewGoroutine not available in v1.3.10)"
  - "SharedUpstreams uses RWMutex for thread-safe access from both TUI and ProxyHandler"

patterns-established:
  - "TUI model uses callbacks (OnUpstream*) for reporting changes to main"
  - "Graceful shutdown uses context.WithTimeout(10s) + server.Shutdown()"

requirements-completed: [TUI-05, TUI-06, TUI-07, TUI-08, TUI-09]

# Metrics
duration: ~7 min
completed: 2026-04-04
---

# Phase 02 Plan 02: TUI Upstream Management Summary

**TUI-based upstream CRUD with keyboard navigation, confirmation dialogs, and graceful shutdown with 10s timeout**

## Performance

- **Duration:** ~7 min
- **Started:** 2026-04-03T17:02:28Z
- **Completed:** 2026-04-04
- **Tasks:** 3
- **Files modified:** 3 (tui.go, main.go, upstream.go)

## Accomplishments

- Implemented TUI navigation state with keyboard shortcuts (a/e/d for add/edit/delete, arrow keys for navigation, q/ctrl+c for shutdown)
- Added add/edit upstream forms with field-by-field navigation and text input
- Created mutex-protected SharedUpstreams for thread-safe state sharing between TUI and ProxyHandler
- Implemented graceful shutdown with 10s timeout waiting for in-flight requests

## Task Commits

Each task was committed atomically:

1. **Task 1: Add TUI navigation state and keyboard handling** - `d927e00` (feat)
2. **Task 2: Implement add/edit/delete upstream forms with proper message emission** - `c062062` (feat)
3. **Task 3: Wire Upstream messages in main.go and implement graceful shutdown** - `b30a658` (feat)

**Plan metadata:** `b30a658` (docs: complete plan)

## Files Created/Modified

- `tui.go` - TUI model with navigation state, form handling, confirmation dialogs, and View rendering for all modes
- `main.go` - Callback wiring to update SharedUpstreams and LoadBalancer, graceful shutdown implementation
- `upstream.go` - SharedUpstreams mutex-protected state, LoadBalancer Add/Update/Delete methods

## Decisions Made

- Used callback pattern (OnUpstreamAdded/Updated/Deleted) instead of tea.Msg for TUI-to-main communication because tea.NewGoroutine is not available in bubbletea v1.3.10
- SharedUpstreams uses RWMutex allowing concurrent reads while write operations are exclusive

## Deviations from Plan

**1. [Rule 3 - Blocking] tea.NewGoroutine not available in v1.3.10**
- **Found during:** Task 3 (wiring upstream messages)
- **Issue:** tea.NewGoroutine function does not exist in the installed bubbletea version (v1.3.10), causing build failure
- **Fix:** Replaced channel-based approach with callback functions passed to the TUI model
- **Files modified:** main.go, tui.go
- **Verification:** Build succeeds, callbacks correctly update SharedUpstreams and LoadBalancer
- **Committed in:** b30a658 (Task 3 commit)

**2. [Rule 1 - Bug] msg.Runes() was incorrectly called as method**
- **Found during:** Task 2 (form input handling)
- **Issue:** msg.Runes() was called as a method but Runes is a field on tea.KeyMsg
- **Fix:** Changed to msg.Runes (direct field access)
- **Files modified:** tui.go
- **Verification:** Build succeeds
- **Committed in:** c062062 (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 bug)
**Impact on plan:** Both deviations necessary for build success and correctness. No scope creep.

## Issues Encountered

- tea.NewGoroutine API not available - switched to callback-based architecture
- msg.Runes() was incorrectly called as method - fixed to field access

## Next Phase Readiness

- TUI foundation complete with full upstream CRUD
- SharedUpstreams ready for Phase 3 hot reload integration
- Graceful shutdown pattern established for Phase 3

---
*Phase: 02-resilience*
*Completed: 2026-04-04*
