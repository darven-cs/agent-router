---
phase: quick
plan: 260404-qrg
type: execute
wave: 1
depends_on: []
files_modified:
  - /home/darven/桌面/dev_app/agent-router/main.go
autonomous: true
requirements: []
must_haves:
  truths:
    - "TUI model selection updates LoadBalancer immediately, requests use new model without restart"
  artifacts:
    - path: main.go
      provides: OnUpstreamModelSelected callback fix
  key_links:
    - from: main.go OnUpstreamModelSelected
      to: lb (LoadBalancer)
      via: lb.UpdateUpstream(u)
---

<objective>
Fix: TUI selects other upstream model but requests still use old model (Zhipu).

Purpose: OnUpstreamModelSelected callback only updates sharedUpstreams but NOT lb (LoadBalancer). Proxy uses lb.SelectNext() which reads stale data.
Output: Fixed OnUpstreamModelSelected in main.go
</objective>

<context>
@/home/darven/桌面/dev_app/agent-router/main.go (lines 127-132 OnUpstreamModelSelected)
@/home/darven/桌面/dev_app/agent-router/main.go (lines 113-119 OnUpstreamToggled - the working pattern)

**Bug:** When user selects an upstream model via TUI (m key or 0-9), `OnUpstreamModelSelected` updates `sharedUpstreams` and persists to config, but does NOT update the `lb` (LoadBalancer).

**Root cause:** Compare with `OnUpstreamToggled` (line 113-119):
```go
tuiModel.OnUpstreamToggled = func(u *Upstream) {
    sharedUpstreams.Update(u.Name, u)
    lb.UpdateUpstream(u)  // <-- This line EXISTS in OnUpstreamToggled
    if err := persistConfig(); err != nil {
        fmt.Fprintf(os.Stderr, "Failed to persist config: %v\n", err)
    }
}
```

But `OnUpstreamModelSelected` (line 127-132) is missing `lb.UpdateUpstream(u)`:
```go
tuiModel.OnUpstreamModelSelected = func(u *Upstream) {
    sharedUpstreams.Update(u.Name, u)
    // lb.UpdateUpstream(u)  <-- MISSING!
    if err := persistConfig(); err != nil {
        fmt.Fprintf(os.Stderr, "Failed to persist upstream model: %v\n", err)
    }
}
```

**Impact:** The proxy uses `h.lb.SelectNext()` and `h.lb.GetEnabled()` which read from `lb.upstreams`. When model is changed in TUI, requests still use the old model because lb is stale.
</context>

<tasks>

<task type="auto">
  <name>Task 1: Add lb.UpdateUpstream to OnUpstreamModelSelected</name>
  <files>/home/darven/桌面/dev_app/agent-router/main.go</files>
  <action>
In main.go, find `OnUpstreamModelSelected` callback (around line 127-132) and add `lb.UpdateUpstream(u)` after `sharedUpstreams.Update(u.Name, u)`:

```go
tuiModel.OnUpstreamModelSelected = func(u *Upstream) {
    sharedUpstreams.Update(u.Name, u)
    lb.UpdateUpstream(u)  // ADD THIS LINE - sync LoadBalancer with SharedUpstreams
    if err := persistConfig(); err != nil {
        fmt.Fprintf(os.Stderr, "Failed to persist upstream model: %v\n", err)
    }
}
```

This mirrors the pattern used in `OnUpstreamToggled` (line 113-119) which correctly updates both `sharedUpstreams` AND `lb`.
  </action>
  <verify>go build -o /tmp/agent-router-test . && echo "BUILD SUCCESS"</verify>
  <done>When user selects a model for an upstream in TUI, requests use the new model immediately (not just after restart)</done>
</task>

</tasks>

<verification>
1. Press 'm' to enter model-select mode
2. Use up/down to select an upstream
3. Press enter or 0-9 to select that upstream's model
4. Send a test request - the request should use the newly selected model
</verification>

<success_criteria>
- Build succeeds
- OnUpstreamModelSelected updates both sharedUpstreams and lb (LoadBalancer)
- Requests immediately use the new model after TUI selection
</success_criteria>

<output>
After completion, create `.planning/quick/260404-qrg-tui-zhipu/260404-qrg-SUMMARY.md`
</output>
