# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.0 — MVP

**Shipped:** 2026-04-05
**Phases:** 3 | **Plans:** 7 | **Tasks:** 21

### What Was Built
- Working Claude API proxy with modulo-hash load balancing across 3 upstreams (Zhipu, Aicodee, Minimax)
- Automatic failover with exponential backoff (1s/2s/4s) and SelectNext routing
- bubbletea TUI with real-time status, request logs, keyboard-driven upstream CRUD, and graceful shutdown
- SQLite usage tracking via async goroutine-channel worker (per-request tokens, latency, upstream data)
- Config hot reload (SIGHUP/TUI/API) and config write-back (SaveConfig) for TUI persistence

### What Worked
- Go native net/http + bubbletea stack proved ideal — zero framework overhead, fast iteration
- GSD phase-based workflow kept scope tight — each phase had clear entry/exit criteria
- Async SQLite writes via goroutine channel avoided both blocking and database-locked errors
- Quick task workflow for bug fixes (TUI model selection, config write-back) was efficient

### What Was Inefficient
- Phase 3 gap closure: CONF-01/02/03 and ADMIN-01/02 requirements were marked but not fully verified before closing
- lipgloss v2 vs v0.6.0 confusion cost time in Phase 1 — should have checked actual Go module tags upfront
- Quick tasks (260404-q4m, 260404-qrg) revealed TUI state sync issues that could have been caught in Phase 2 verification

### Patterns Established
- Config write-back pattern: TUI changes → SaveConfig() → persist to config.yaml → survive reload
- Single goroutine drain pattern for SQLite writes (avoid database locked)
- isRetryable safe default (false) with explicit whitelist for timeout/5xx/429

### Key Lessons
1. Verify requirements against actual implementation, not just plan completion — 5 of 37 v1 requirements slipped through as "partially done"
2. Check Go module version tags before starting — vanity URLs can have non-standard versioning (lipgloss v2 vs v0.6.0)
3. TUI state management needs explicit synchronization tests — keyboard shortcuts can create race conditions with shared state
4. Gap closure plans are valuable — config write-back (03-04) was the right fix at the right time

### Cost Observations
- Model mix: primarily sonnet (balanced profile)
- Sessions: ~4 sessions over 2 days
- Notable: 1890 LOC / 7 files / 54 commits — lean and focused delivery

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Sessions | Phases | Key Change |
|-----------|----------|--------|------------|
| v1.0 | ~4 | 3 | Initial GSD workflow, phase-based execution |

### Cumulative Quality

| Milestone | Tests | Coverage | Zero-Dep Additions |
|-----------|-------|----------|-------------------|
| v1.0 | 0 | N/A | GORM, bubbletea, lipgloss, fsnotify |

### Top Lessons (Verified Across Milestones)

1. Go native net/http is sufficient for API proxy — no framework needed
2. GSD phase workflow maintains tight scope and clear deliverables
