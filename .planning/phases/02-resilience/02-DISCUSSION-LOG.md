# Phase 2: Resilience - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the analysis.

**Date:** 2026-04-04
**Phase:** 02-resilience
**Mode:** auto
**Areas analyzed:** Failover Behavior, Retry State Tracking, TUI Management Flow, Keyboard Navigation, Graceful Shutdown

## Auto-Resolved Decisions

[auto] All gray areas resolved with recommended defaults based on requirements and existing code patterns.

### Failover Behavior
- **D-01:** Retry trigger: 5xx + timeout only (not 4xx except 429)
- **D-02:** Exponential backoff: 1s → 2s → 4s
- **D-03:** Maximum 3 retries (4 total attempts)
- **D-04:** All fail → error code 1001
- **D-05:** No upstream enabled → error 1001 immediately

### Retry State Tracking
- **D-06:** Add RetryAttempt/RetryCount to RequestLog
- **D-07:** Log each retry attempt
- **D-08:** Final failure shows all retries

### TUI Management
- **D-09:** `a` key → add upstream inline form
- **D-10:** `e` key → edit selected upstream
- **D-11:** `d` key → delete with confirmation
- **D-12:** Form fields: Name, URL, API Key, Auth Type, Timeout, Enabled
- **D-13:** Validation: required fields, URL format, min timeout
- **D-14:** In-memory changes immediately effective

### Keyboard Navigation
- **D-15:** ↑/↓ arrows navigate upstream list
- **D-16:** a/e/d trigger actions on selected upstream
- **D-17:** Esc cancels form / returns to navigation
- **D-18:** q/ctrl+c triggers shutdown confirmation

### Graceful Shutdown
- **D-19:** Confirmation dialog: "Shutdown? [y/n]"
- **D-20:** y/Enter confirms, other key cancels
- **D-21:** On confirm: stop accepting, wait in-flight (max 10s), then exit

### Integration
- **D-22:** LoadBalancer.SelectNext(after) method
- **D-23:** ProxyHandler retry loop with backoff
- **D-24:** TUI handles new message types
- **D-25:** Config state protected by mutex

## External Research

No external research needed — requirements and existing code patterns provided sufficient guidance.

---

*Generated: 2026-04-04 (auto mode)*
