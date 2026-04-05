---
phase: quick
plan: 260404-q4m
subsystem: tui
tags: [go, tui, model-selection, persistence]
provides:
  - handleModelSelect now saves selected model to upstream and persists to config.yaml
  - New OnUpstreamModelSelected callback updates sharedUpstreams and persists
affects: [tui, config-persistence]
tech-stack:
  added: []
  patterns: [callback-based TUI events]
key-files:
  created: []
  modified:
    - tui.go
    - main.go
key-decisions: []
duration: 5min
completed: 2026-04-04
---

# Quick Task 260404-q4m: TUI Model Selection Fix Summary

**Model selection now saves to upstream and persists to config.yaml**

## Performance
- **Duration:** 5min
- **Tasks:** 1
- **Files modified:** 2

## Problem
In `handleModelSelect`, when user selected a model in model-select mode and pressed Enter or 0-9:
- Code read `us.Model` from the upstream but never wrote it back
- `OnDefaultModelChanged` only updated global default, not per-upstream model
- Model selection was lost on SIGHUP reload

## Solution
1. Added `OnUpstreamModelSelected func(*Upstream)` callback to TUI model
2. In `handleModelSelect` enter/0-9 cases: set `us.Model = m.defaultModel` before calling callbacks
3. New callback in `main.go` calls `sharedUpstreams.Update(u.Name, u)` then `persistConfig()`

## Task Commits
1. **Task 1: Fix handleModelSelect to save model to upstream** - `cd0f316`

## Files Modified
- `tui.go` - Added OnUpstreamModelSelected callback field, fixed handleModelSelect to save model
- `main.go` - Implemented OnUpstreamModelSelected to update sharedUpstreams and persist config

## Verification
- Build: `go build -o /tmp/agent-router-test .` - SUCCESS
- Tests: `go test ./...` - No test files (project has none)
