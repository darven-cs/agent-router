# Pitfalls Research

**Domain:** v2.0 Architecture Refactor -- Event Bus, Middleware Chain, Go Project Layout, TUI Componentization
**Project:** Agent Router (Go API Proxy with bubbletea TUI)
**Researched:** 2026-04-05
**Confidence:** HIGH (codebase-specific analysis), MEDIUM (bubbletea internals from community sources)

---

## Critical Pitfalls

### Pitfall 1: Event Bus Goroutine Leak -- Subscribers Never Unsubscribe

**What goes wrong:**
When replacing direct callbacks with a Go channel-based Event Bus, every subscriber goroutine that listens on a channel will leak if the channel is never closed or the goroutine never receives a stop signal. In the current codebase, `main.go` creates `logChan` (buffered 100) and `usageChan` (buffered 100) and drains them in goroutines. Adding an Event Bus multiplies this pattern: each subscriber (TUI, usage tracker, logger, admin API) will have its own goroutine. If a subscriber is removed or replaced during config reload, its goroutine blocks forever on the receive.

**Why it happens:**
The existing code already has this latent risk. `usageChan` is closed on shutdown (`close(usageChan)` in `main.go:186`), but `logChan` is never explicitly closed -- the goroutine at line 166 that drains `logChan` will leak if the TUI exits before all messages are consumed. When you generalize this into an Event Bus with multiple subscribers, the problem compounds. Developers forget to implement `Unsubscribe()` or forget to call it.

**How to avoid:**
- Every subscriber registration must return an unsubscribe function or `context.CancelFunc`.
- Use `context.WithCancel` for each subscriber goroutine. The Event Bus `Publish()` method must select on both the subscriber channel AND the done context:
  ```go
  func (eb *EventBus) Publish(e Event) {
      eb.mu.RLock()
      defer eb.mu.RUnlock()
      if eb.closed { return }
      for _, ch := range eb.subscribers {
          select {
          case ch <- e:
          case <-eb.done:
          }
      }
  }
  ```
- Never close subscriber channels from the publisher side. Only the subscriber should close its own channel.
- Use `sync.WaitGroup` to track subscriber goroutines and wait for them during shutdown.

**Warning signs:**
- `runtime.NumGoroutine()` increases after every config reload.
- `go test -race` shows goroutines stuck on channel receive after test completion.
- Memory profile shows growing goroutine stacks.

**Phase to address:**
ARCH-02 (Event Bus). The Event Bus implementation itself must include lifecycle management from the first commit. Do not build a "publish-only" bus and add unsubscribe later.

---

### Pitfall 2: Send on Closed Channel Panic in Event Bus

**What goes wrong:**
When the Event Bus shuts down, it closes its internal channels. If a `Publish()` call happens concurrently with the close (e.g., an in-flight HTTP request finishes and tries to log), the program panics with `send on closed channel`. This is a runtime panic, not a recoverable error -- it crashes the entire process.

**Why it happens:**
Go intentionally does not provide an `isClosed()` API for channels because checking-then-sending is inherently racy. The current codebase has a simplified version of this risk: `logToChan()` checks `if h.logChan != nil` before sending (proxy.go:302), but this nil-check is not synchronized with any channel closure. In the Event Bus, the publisher needs to guard sends with a mutex and a `closed` boolean flag.

**How to avoid:**
- Guard all `Publish()` calls with `sync.RWMutex` + a `closed bool` flag:
  ```go
  type EventBus struct {
      mu     sync.RWMutex
      closed bool
      subs   []chan Event
  }
  func (eb *EventBus) Publish(e Event) {
      eb.mu.RLock()
      defer eb.mu.RUnlock()
      if eb.closed { return } // safe: no send after close
      for _, ch := range eb.subs {
          select {
          case ch <- e:
          default: // drop if subscriber is slow (configurable)
          }
      }
  }
  func (eb *EventBus) Close() {
      eb.mu.Lock()
      defer eb.mu.Unlock()
      eb.closed = true
      // do NOT close subscriber channels -- subscribers own their channels
  }
  ```
- Never close a channel from the sending side. Only the receiving goroutine should close its own channel.
- During shutdown, call `Close()` first, then wait for subscribers to drain via `WaitGroup`.

**Warning signs:**
- Random panics during shutdown with message `send on closed channel`.
- Panics during config reload if reload triggers bus reinitialization.

**Phase to address:**
ARCH-02 (Event Bus). Shutdown safety is not optional -- build it into the initial Event Bus struct.

---

### Pitfall 3: Middleware Chain Breaks on Early Return or Missing `next.ServeHTTP()`

**What goes wrong:**
When decomposing the monolithic `ProxyHandler.ServeHTTP()` into a middleware chain (auth, logging, retry, routing), forgetting to call `next.ServeHTTP(w, r)` in any code path breaks the entire chain. Requests silently return with no response, or only partial middleware executes. This is the single most common middleware mistake in Go.

**Why it happens:**
The current `ServeHTTP()` (proxy.go:76-123) has early returns for auth failure (`writeError` + return), admin routes, and 404. When converting these into middleware, each `return` must be replaced with either calling `next` or writing a complete response. If a middleware writes a response AND calls `next`, the response is corrupted. If it writes a response and does NOT call `next`, the chain stops -- but only if intentional. The mistake is accidental omission.

**How to avoid:**
- Adopt a strict middleware contract: each middleware EITHER calls `next.ServeHTTP(w, r)` OR writes a complete response, NEVER both.
- Write a `Middleware` type and a `Chain()` helper from the start:
  ```go
  type Middleware func(http.Handler) http.Handler
  func Chain(middlewares ...Middleware) Middleware {
      return func(final http.Handler) http.Handler {
          for i := len(middlewares) - 1; i >= 0; i-- {
              final = middlewares[i](final)
          }
          return final
      }
  }
  ```
- Write a unit test that verifies the full chain executes by counting middleware visits. Every middleware must be visited in order.
- The retry middleware is special: it wraps `next` in a loop. Do NOT implement retry as a standard middleware -- implement it as a wrapper around the final upstream call.

**Warning signs:**
- HTTP requests returning empty responses (0 bytes, no headers).
- Auth middleware passes but logging middleware never fires.
- 404 returned for valid routes after middleware refactor.

**Phase to address:**
ARCH-03 (Middleware Chain). Write the `Chain()` helper and the middleware contract test BEFORE implementing individual middleware functions.

---

### Pitfall 4: Wrong Middleware Ordering Exposes Security Holes

**What goes wrong:**
Middleware executes in onion order (outermost first on request, innermost first on response). Placing logging before auth means unauthenticated requests are logged (information leak). Placing rate limiting before auth means rate limits apply to failed auth attempts (DoS amplification). Placing recovery AFTER auth means a panic in auth is uncaught.

**Why it happens:**
The current monolithic handler has implicit ordering: admin check, then auth, then proxy (proxy.go:77-122). When decomposing into middleware, developers often chain them in the order they think of them, not in the order they need to execute. The "onion" model is counterintuitive: the first middleware in the chain is the outermost layer, executing first on the request and last on the response.

**How to avoid:**
- Define the middleware ordering contract explicitly in a comment and enforce it:
  ```
  Recovery -> Logging -> Auth -> RateLimit -> Routing -> Proxy
  ```
- The `Chain()` function applies middleware in reverse order (last listed = outermost). Document this clearly:
  ```go
  // Middleware listed first executes LAST on request (outermost layer)
  handler := Chain(
      recovery,  // outermost: catches panics from everything below
      logging,   // logs all requests including auth failures
      auth,      // rejects unauthenticated before rate limiting
      ratelimit, // rate limit only authenticated requests
      routing,   // route to admin or proxy
  )(proxyHandler)
  ```
- Write integration tests that verify: unauthenticated requests ARE logged, rate-limited requests have valid auth tokens, panics are recovered with proper error responses.

**Warning signs:**
- Auth failures not appearing in logs.
- Rate limiting triggered before auth (check order of log lines).
- Unhandled panics crashing the server.

**Phase to address:**
ARCH-03 (Middleware Chain). The ordering must be defined in the plan and verified in the first implementation commit.

---

### Pitfall 5: Import Cycle When Restructuring to cmd/internal Layout

**What goes wrong:**
Moving from a single `package main` with 7 files to a `cmd/agent-router/` + `internal/` layout creates circular imports. For example, `internal/proxy` needs `Upstream` type from `internal/upstream`, but `internal/upstream` needs `RequestLog` from `internal/proxy`, creating an import cycle. The Go compiler rejects this with `import cycle not allowed`.

**Why it happens:**
The current codebase has all types in `package main` with no import boundaries. Types are freely shared:
- `RequestLog` is defined in `proxy.go` but used in `tui.go`, `usage.go`, `main.go`
- `Upstream` is defined in `upstream.go` but used everywhere
- `Config` is defined in `config.go` but used everywhere
- Global variables (`db`, `cfg`, `lb`, `proxyHandler`, `sharedUpstreams`) are accessed directly from `admin.go`, `proxy.go`, `tui.go`

When you split these into packages, every cross-reference becomes a potential import cycle.

**How to avoid:**
- Create a shared `internal/types/` or `internal/domain/` package for shared types (`Upstream`, `RequestLog`, `Config`, `UpstreamConfig`) with ZERO imports from other internal packages. This package must be dependency-free.
- Define interfaces in the consuming package, not the providing package. For example, TUI should define `UpstreamManager` interface, not import the concrete implementation.
- Use dependency injection via constructors rather than global variables:
  ```go
  // Instead of accessing global 'db' in admin.go:
  type AdminHandler struct {
      db     *gorm.DB
      config *ConfigManager
      upstreams *SharedUpstreams
  }
  ```
- Start with a `internal/types/` package containing only data structs, then split files into packages one at a time, compiling after each move.

**Warning signs:**
- `import cycle not allowed` compiler error after moving a file.
- Needing to pass `main.` prefixed types across packages.
- Interface definitions that mirror concrete types exactly (sign of forced decoupling).

**Phase to address:**
ARCH-01 (Directory Restructuring). The shared types package must be created FIRST, before any other package split. This is the foundation of the entire restructuring.

---

### Pitfall 6: Global State Remains After Restructuring

**What goes wrong:**
The current codebase has 7 global variables in `main.go` (`db`, `usageChan`, `execPath`, `sharedUpstreams`, `lb`, `proxyHandler`, `cfg`) and 1 in `usage.go` (`stats`). After restructuring to `cmd/internal`, these globals persist as package-level variables in their new packages, defeating the purpose of the restructuring. Tests become impossible to parallelize because they share mutable global state.

**Why it happens:**
It is tempting to move `var db *gorm.DB` from `main.go` to `internal/usage/db.go` and call it "restructured." The code compiles, the program works, but the architecture is still a flat global-mutable-singleton design wearing a directory-structure costume.

**How to avoid:**
- Every package must receive its dependencies through constructor functions, not global variables.
- The `main()` function should be the ONLY place that wires dependencies together:
  ```go
  func main() {
      cfg := config.Load(path)
      db := usage.InitDB(dbPath)
      bus := eventbus.New()
      lb := upstream.NewLoadBalancer(cfg.Upstreams)
      proxy := proxy.NewHandler(lb, cfg, bus)
      admin := admin.NewHandler(db, cfg, bus)
      tui := tui.NewModel(cfg, bus)
      // wire event bus subscriptions
      bus.Subscribe("request.log", tui.OnRequestLog)
      bus.Subscribe("config.reload", proxy.OnConfigReload)
      // start everything
      ...
  }
  ```
- Verify restructuring by checking: can I create two independent instances of the proxy handler in a test? If not, globals are still leaking.

**Warning signs:**
- Package-level `var` declarations outside `main`.
- Tests that fail when run in parallel (`go test -count=1` passes but `-count=2` fails).
- Needing to call `init()` functions to set up state.

**Phase to address:**
ARCH-01 (Directory Restructuring). Eliminate ALL package-level mutable state during the restructuring phase. Verify with parallel tests.

---

### Pitfall 7: TUI Model Split Breaks bubbletea's Value Receiver Contract

**What goes wrong:**
When splitting the 837-line `tui.go` into sub-components (app, update, view, form), the current `model` struct uses value receivers (`func (m model) Update(...) (tea.Model, tea.Cmd)`). If sub-components use pointer receivers or if the parent model mutates child models through pointers, bubbletea's Elm architecture breaks. State changes in child models may not propagate correctly, or the TUI may render stale state.

**Why it happens:**
bubbletea's event loop calls `model.Update(msg)` and expects the returned model to be the new complete state. With value receivers, modifications inside `Update()` are lost unless returned. The current code works because `m` is modified and returned. When splitting into sub-models, if a child model's `Update()` uses a pointer receiver and modifies `*m` but the parent returns the parent's value (not the child's updated pointer), the child's changes are silently discarded.

**How to avoid:**
- Keep value receivers for all `tea.Model` implementations (`Init`, `Update`, `View`).
- Use pointer receivers only for helper methods that are called within `Update()`:
  ```go
  func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
      // value receiver: must return m
      m.child.Update(msg) // child must return updated value
      return m, nil
  }
  ```
- When composing child models, always capture the returned model:
  ```go
  var cmd tea.Cmd
  m.form, cmd = m.form.Update(msg) // CORRECT: capture returned model
  return m, cmd
  ```
- Do NOT use pointer receivers for `Update()` or `View()` on any tea.Model.

**Warning signs:**
- TUI state not updating after key press.
- Form inputs not appearing despite key handling.
- `go test -race` showing data races in TUI code.

**Phase to address:**
TUI-02 (TUI Componentization). Establish the value-receiver rule before splitting any code.

---

### Pitfall 8: TUI Sub-Models Not Receiving WindowSizeMsg

**What goes wrong:**
After decomposing `tui.go` into sub-components (nav, content, status bar), only the root model receives `tea.WindowSizeMsg`. Child components (form, upstream list, model selector) never get updated dimensions, causing layout breakage. Borders overflow, text gets cut off, or the status bar disappears.

**Why it happens:**
bubbletea only sends `tea.WindowSizeMsg` to the top-level model's `Update()`. The current monolithic `model` handles it correctly (tui.go:181-182). After splitting, the root model must explicitly forward `WindowSizeMsg` to all child models. This is a required step that is easy to forget.

**How to avoid:**
- In the root model's `Update()`, always forward `tea.WindowSizeMsg` to ALL child models:
  ```go
  case tea.WindowSizeMsg:
      m.width = msg.Width
      m.height = msg.Height
      m.nav.SetSize(msg.Width, 1)         // forward dimensions
      m.content.SetSize(msg.Width-4, msg.Height-4)
      m.statusBar.SetSize(msg.Width, 1)
  ```
- Use lipgloss `Height()` and `Width()` methods to compute child dimensions, not hardcoded values. The current code already has the brittleness pattern: `renderStatus()` uses `m.width` directly (tui.go:815).
- Write a test: resize the terminal to 40x10 and verify no layout overflow.

**Warning signs:**
- Content area not filling terminal after resize.
- Status bar overflowing or disappearing.
- Border rendering artifacts on small terminals.

**Phase to address:**
TUI-02 (TUI Componentization). Dimension propagation must be built into every sub-model from the start.

---

### Pitfall 9: Callback-to-EventBus Migration Introduces Ordering Bugs

**What goes wrong:**
The current codebase uses 6 direct callbacks set in `main.go` (lines 88-136): `OnUpstreamAdded`, `OnUpstreamUpdated`, `OnUpstreamDeleted`, `OnUpstreamToggled`, `OnDefaultModelChanged`, `OnUpstreamModelSelected`, `OnReload`. These callbacks execute synchronously in order. When replaced with an asynchronous Event Bus, events may arrive out of order. For example, `UpstreamAdded` event arrives AFTER `UpstreamModelSelected` event for the same upstream, causing the model selection to reference a non-existent upstream.

**Why it happens:**
Go channels do not guarantee ordering across multiple goroutines sending to the same channel. Even with a single goroutine, if the event bus has multiple subscribers processing at different speeds, the observable state can be inconsistent. The current callbacks are synchronous and ordered: add upstream, then update load balancer, then persist config -- all in sequence. The event bus makes this asynchronous.

**How to avoid:**
- For events that must be ordered (add before model-select, update before persist), use synchronous event dispatching for the critical path:
  ```go
  func (eb *EventBus) PublishSync(e Event) {
      // Synchronous: all subscribers process before PublishSync returns
      for _, handler := range eb.handlers {
          handler(e)
      }
  }
  ```
- Only use asynchronous dispatch for fire-and-forget events (logging, metrics, TUI display).
- Alternatively, use a single-event-loop pattern where all events are processed sequentially by one goroutine, preserving order.

**Warning signs:**
- TUI shows stale upstream list after rapid add/delete.
- Config file contains wrong upstream state after multiple quick operations.
- Load balancer routing to deleted upstream.

**Phase to address:**
ARCH-02 (Event Bus). Define which events are synchronous vs asynchronous before implementation. The TUI's CRUD operations (add/edit/delete/toggle) MUST be synchronous.

---

### Pitfall 10: Event Bus Replaces Callbacks But Retains Callbacks As Well

**What goes wrong:**
During incremental migration from callbacks to Event Bus, both systems coexist. The TUI model still has `OnUpstreamAdded` callback fields while also subscribing to events. This leads to double-execution: upstream is added once via callback, then added again via event handler. Or worse, the callback persists config, then the event handler also persists config, causing a file write race.

**Why it happens:**
Incremental migration is the correct approach, but forgetting to remove old callbacks after wiring the Event Bus creates duplicate execution. The `main.go` callback setup (lines 88-136) is deeply coupled -- each callback touches `sharedUpstreams`, `lb`, and `persistConfig()`. If the Event Bus subscriber also does these things, they run twice.

**How to avoid:**
- Migrate ONE callback at a time. For each callback:
  1. Add Event Bus subscriber that does the same work.
  2. Verify the subscriber works.
  3. Remove the callback field and the callback assignment in `main()`.
  4. Compile and test.
- Use a checklist: mark each callback as migrated.
- After all callbacks are migrated, remove the callback fields from the `model` struct entirely.
- The target state: `model` struct has ZERO callback fields. All communication goes through Event Bus or `tea.Cmd`.

**Warning signs:**
- Duplicate config file writes visible in logs.
- `persistConfig()` called twice for a single TUI action.
- Upstream appears twice in load balancer after add.

**Phase to address:**
ARCH-02 (Event Bus). Plan the callback-to-event migration as an explicit checklist in the phase plan.

---

### Pitfall 11: Directory Restructuring Breaks Single-Binary Deployment

**What goes wrong:**
The project constraint is "single binary, local deployment." After restructuring to `cmd/agent-router/main.go`, the build output changes. The existing `agent-router` binary in the project root (used by users) breaks. Relative paths to `config.yaml` and `usage.db` (computed from `os.Executable()` in `main.go:31`) may resolve differently if the binary is built to a different output path.

**Why it happens:**
`execPath, _ := os.Executable()` returns the path of the running binary, not the source directory. After restructuring, `go build` output defaults to the current directory. If the build command changes (e.g., `go build -o bin/agent-router ./cmd/agent-router`), the binary location changes, and `filepath.Dir(execPath)` no longer points to the directory containing `config.yaml`.

**How to avoid:**
- Keep the build output path consistent: `go build -o agent-router ./cmd/agent-router` builds to project root, same as before.
- Add a Makefile or build script that enforces this:
  ```makefile
  build:
      go build -o agent-router ./cmd/agent-router
  ```
- Test the binary after restructuring: run `./agent-router` from the project directory and verify it finds `config.yaml`.
- Consider adding a fallback: if `config.yaml` is not found next to the binary, check the current working directory.

**Warning signs:**
- "Failed to load config from .../config.yaml: no such file or directory" on startup.
- `usage.db` created in unexpected directory.
- Binary works when run with `go run ./cmd/agent-router` but not when built and moved.

**Phase to address:**
ARCH-01 (Directory Restructuring). The build command and path resolution must be verified in the FIRST commit of restructuring.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Keep callbacks alongside Event Bus during migration | Faster migration, incremental | Double execution, confusion | Migration period only (1-2 commits), never in committed code |
| Package-level globals in new internal packages | Faster restructuring, compiles immediately | Tests can't parallelize, hidden coupling | Never -- eliminate during restructuring |
| Single file for Event Bus instead of separate publisher/subscriber | Simpler initial implementation | Hard to test subscribers independently | MVP only, split once subscribers grow beyond 3 |
| Middleware ordering hardcoded in main.go | Simple, no config needed | Reordering requires code change | Acceptable for this project (fixed middleware set) |
| Skip layout arithmetic refactoring in TUI split | Faster componentization | Breaks on resize, hard to debug | Never -- lipgloss.Height/Width from the start |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| Event Bus + Config Reload | Reload re-creates bus but subscribers still listen on old bus | Bus is a singleton; reload publishes event, subscribers react; bus is never replaced |
| Middleware + Existing Admin Routes | Admin routes handled in middleware chain but ProxyHandler still checks `/admin/*` | Move admin routing to a separate handler/mux BEFORE the middleware chain |
| TUI + Event Bus | TUI directly calls `bus.Publish()` from `Update()` (blocking) | TUI publishes events via `tea.Cmd` (non-blocking), receives responses as `tea.Msg` |
| Directory Restructure + GORM | `initDB()` moved to `internal/usage/` but still reads global `db` | Pass `*gorm.DB` to all functions that need it; `usage` package receives it in constructor |
| Middleware + Retry Logic | Retry implemented as middleware wrapping `next`, re-executing auth middleware on each retry | Retry wraps only the upstream call, not the entire middleware chain |

---

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Event Bus fan-out blocks on slow subscriber | All events stall when TUI is slow to consume | Use buffered channels per subscriber, `select/default` to drop stale events | > 10 events/sec with slow TUI |
| Middleware copies request body for each layer | Memory grows per request, GC pressure | Read body once in auth/logging middleware, store in context | > 100 concurrent requests |
| TUI re-renders on every event bus message | Terminal flickering, high CPU | Throttle View() calls to 100ms intervals; batch events | Any sustained event rate > 5/sec |
| Import cycle workaround via interface reflection | Slow type assertions at runtime | Keep shared types package dependency-free; use interfaces, not reflection | Always (design smell) |

---

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Event bus logs contain API keys from upstream config | Key leakage in logs, TUI display, SQLite | Scrub sensitive fields before publishing events; use separate event types for display vs internal |
| Middleware order allows unauthenticated admin access | Anyone can reload config or view status | Auth middleware must wrap ALL routes including admin; test with missing auth header |
| Config reload via event bus without validation | Malformed config.yaml accepted, breaking service | Validate config before publishing reload event; reject invalid, keep old config |

---

## "Looks Done But Isn't" Checklist

- [ ] **Event Bus:** Bus publishes events but subscribers never unsubscribe -- verify lifecycle with `go test -race`
- [ ] **Event Bus:** Bus shuts down without panic -- verify `Publish()` after `Close()` returns silently, not panics
- [ ] **Middleware Chain:** All middleware call `next` OR write complete response -- verify with chain execution test
- [ ] **Middleware Chain:** Retry middleware does NOT re-execute auth -- verify retry only wraps upstream call
- [ ] **Directory Restructure:** All global variables eliminated -- verify with `grep -r "^var " internal/`
- [ ] **Directory Restructure:** Binary finds `config.yaml` from new build path -- verify by building and running
- [ ] **TUI Split:** Sub-models receive `WindowSizeMsg` -- verify by resizing terminal to 40x10
- [ ] **TUI Split:** Value receivers on all `tea.Model` methods -- verify no pointer receivers on `Init/Update/View`
- [ ] **Callback Migration:** Zero callback fields remain on model struct -- verify `grep "On[A-Z]" tui/`
- [ ] **Config Reload:** Event bus survives config reload (bus not replaced) -- verify reload does not re-create bus

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Goroutine leak from missing unsubscribe | HIGH | Restart process, add unsubscribe mechanism, use `goleak` test package |
| Send on closed channel panic | MEDIUM | Add `sync.RWMutex` + `closed` flag guard to `Publish()` |
| Broken middleware chain (missing next) | LOW | Add chain execution test, fix missing `next.ServeHTTP()` call |
| Import cycle | MEDIUM | Extract shared types to dependency-free package, use interfaces |
| Global state in internal packages | HIGH | Refactor all constructors to accept dependencies, add parallel tests |
| TUI state not updating | MEDIUM | Check receiver types, ensure value receivers return updated model |
| Out-of-order events | HIGH | Add synchronous dispatch for CRUD operations, or single-event-loop |
| Binary path resolution wrong | LOW | Fix build command output path, add CWD fallback |

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Event Bus goroutine leak (P1) | ARCH-02 | `go test -race`, check `runtime.NumGoroutine()` before/after reload |
| Send on closed channel (P2) | ARCH-02 | Publish after Close must not panic (unit test) |
| Middleware chain break (P3) | ARCH-03 | Chain execution count test |
| Wrong middleware order (P4) | ARCH-03 | Integration test: unauth request logged, rate-limit after auth |
| Import cycle (P5) | ARCH-01 | `go build ./...` passes after each package split |
| Lingering global state (P6) | ARCH-01 | `grep -r "^var " internal/` returns zero mutable globals |
| Value receiver violation (P7) | TUI-02 | `grep -n "func (.*) \*" tui/*.go` finds no Init/Update/View |
| WindowSizeMsg not forwarded (P8) | TUI-02 | Resize terminal to 40x10, verify layout |
| Event ordering bugs (P9) | ARCH-02 | Rapid CRUD test: add+delete same upstream in <10ms |
| Duplicate callback+event (P10) | ARCH-02 | `grep "On[A-Z]" tui/` returns zero results after migration |
| Build path breakage (P11) | ARCH-01 | `go build -o agent-router ./cmd/agent-router && ./agent-router` starts correctly |

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| ARCH-01: Create `internal/types/` | Pitfall P5: import cycle on first package split | Create types package with ALL shared structs first, compile, then split |
| ARCH-01: Move files to packages | Pitfall P6: globals move to package level | Constructor injection for every package |
| ARCH-01: Verify build | Pitfall P11: binary path changes | Test build+run after restructuring |
| ARCH-02: Design Event Bus | Pitfall P1: no unsubscribe mechanism | Include lifecycle in design, not bolted on later |
| ARCH-02: Replace callbacks | Pitfall P10: callbacks coexist with events | Migrate one-by-one with explicit checklist |
| ARCH-02: Async events | Pitfall P9: ordering breaks CRUD | Use synchronous dispatch for CRUD operations |
| ARCH-03: Middleware chain | Pitfall P3: missing next call | Chain execution test before individual middleware |
| ARCH-03: Middleware order | Pitfall P4: auth after logging | Define order contract, integration test |
| TUI-02: Split model struct | Pitfall P7: pointer receivers break Elm model | Value receivers only for tea.Model methods |
| TUI-02: Sub-model dimensions | Pitfall P8: WindowSizeMsg not forwarded | Forward to all children, test resize |

---

## Sources

- Go channel closing principles: [oldme.net/article/18](https://oldme.net/article/18) -- Channel Closing Principle
- Go concurrency pitfalls: [go101.org/article/concurrent-common-mistakes](https://go101.org/article/concurrent-common-mistakes.html) -- Go 101 reference
- Goroutine leak prevention: [bytesizego.com/blog/common-goroutine-leaks](https://www.bytesizego.com/blog/common-goroutine-leaks) -- Forgotten receivers, missing context cancellation
- Middleware chain patterns: [alexedwards.net/blog/making-and-using-middleware](https://www.alexedwards.net/blog/making-and-using-middleware) -- Classic Go middleware guide
- Middleware gotchas: [blog.stackademic.com](https://blog.stackademic.com/untangling-the-web-practical-middleware-patterns-in-go-40fa3ebae901) -- Missing next.ServeHTTP()
- Go project layout: [github.com/golang-standards/project-layout](https://github.com/golang-standards/project-layout) -- Standard directory conventions
- Go official module layout: [go.dev/doc/modules/layout](https://go.dev/doc/modules/layout) -- Official cmd/internal guidance
- Import cycle solutions: [stackoverflow.com/questions/45609236](https://stackoverflow.com/questions/45609236/golang-import-cycle-not-allowed-after-splitting-up-my-program-into-subpackages) -- Breaking cycles
- bubbletea nested models: [donderom.com/posts/managing-nested-models-with-bubble-tea/](https://donderom.com/posts/managing-nested-models-with-bubble-tea/) -- Model Stack architecture
- bubbletea tips: [leg100.github.io/en/posts/building-bubbletea-programs/](https://leg100.github.io/en/posts/building-bubbletea-programs/) -- Value receivers, event loop speed, message ordering
- Existing codebase analysis: main.go (272 LOC), proxy.go (334 LOC), tui.go (837 LOC), upstream.go (158 LOC), config.go (67 LOC), usage.go (78 LOC), admin.go (144 LOC)

---

*Pitfalls research for: Agent Router v2.0 Architecture Refactor*
*Researched: 2026-04-05*
