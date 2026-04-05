---
status: testing
phase: 02-resilience
source:
  - 02-01-SUMMARY.md
  - 02-02-SUMMARY.md
started: 2026-04-04T20:30:00Z
updated: 2026-04-04T20:30:00Z
---

## Current Test

number: 8
name: Retry Config Display
expected: Navigation shows current retry configuration (max retries, backoff times).
awaiting: user response

## Tests

### 1. Cold Start Smoke Test
expected: Kill any running server/service. Clear ephemeral state (temp DBs, caches, lock files). Start the application from scratch. Server boots without errors, any seed/migration completes, and a primary query (health check, homepage load, or basic API call) returns live data.
result: pass

### 2. TUI Navigation Menu
expected: TUI displays Navigation menu with options: List Upstreams, Add Upstream, Edit Upstream, Delete Upstream, Retry Config, Shutdown. Arrow keys navigate between options.
result: issue
reported: "UI布局不对，用户体验不行，根本操作不了"
severity: blocker

### 3. List Upstreams Display
expected: Shows list of configured upstreams with name, endpoint, status (active/disabled), and health.
result: blocked
blocked_by: ui-layout
reason: "UI布局问题导致无法进行后续操作"

### 4. Add Upstream Form
expected: Press 'a' to open Add Upstream form. Fields: Name, Endpoint URL, Priority. Tab/Arrow keys navigate fields. Enter submits. New upstream appears in list.
result: blocked
blocked_by: ui-layout
reason: "UI布局问题导致无法进行后续操作"

### 5. Edit Upstream Form
expected: Select upstream, press 'e' to edit. Same form as Add, pre-filled. Save updates the upstream.
result: blocked
blocked_by: ui-layout
reason: "UI布局问题导致无法进行后续操作"

### 6. Delete Upstream with Confirmation
expected: Select upstream, press 'd'. Shows confirmation dialog. Confirm removes upstream from list.
result: blocked
blocked_by: ui-layout
reason: "UI布局问题导致无法进行后续操作"

### 7. Graceful Shutdown
expected: Press 'q' or Ctrl+C. Server waits for in-flight requests (up to 10s), then exits cleanly.
result: blocked
blocked_by: ui-layout
reason: "UI布局问题导致无法进行后续操作"

### 8. Retry Config Display
expected: Navigation shows current retry configuration (max retries, backoff times).
result: blocked
blocked_by: ui-layout
reason: "UI布局问题导致无法进行后续操作"

## Summary

total: 8
passed: 1
issues: 1
pending: 0
skipped: 0
blocked: 6

## Gaps

- truth: "TUI界面布局合理，键盘导航流畅，用户可以正常操作所有功能"
  status: failed
  reason: "User reported: UI布局不对，用户体验不行，根本操作不了"
  severity: blocker
  test: 2
  root_cause: ""
  artifacts: []
  missing: []
  debug_session: ""
