# Phase 2: Resilience - Research

**Researched:** 2026-04-04
**Domain:** Go HTTP proxy resilience, bubbletea TUI form handling, graceful shutdown
**Confidence:** HIGH

## Summary

Phase 2 implements three major feature areas: (1) automatic failover with exponential backoff retry, (2) TUI-based upstream CRUD management with keyboard navigation, and (3) graceful shutdown with confirmation. The existing codebase uses Go 1.25.3 with bubbletea v1.3.10 and lipgloss v1.1.0. All decisions are locked in CONTEXT.md -- research focuses on implementation patterns.

**Primary recommendation:** Implement retry loop in `proxy.go` using `time.Sleep` with exponential backoff (1s, 2s, 4s), add `SelectNext()` method to LoadBalancer for failover, use mutex-protected shared state for TUI upstreams, and handle graceful shutdown via `http.Server.Shutdown(ctx)` with 10s timeout.

---

## User Constraints (from CONTEXT.md)

### Locked Decisions

**Failover Behavior (FAIL-01, FAIL-02, FAIL-03, FAIL-04):**
- D-01: Retry trigger: 5xx responses OR timeout (context deadline exceeded) -- NOT 4xx except 429
- D-02: Exponential backoff: 1s → 2s → 4s delays between retries
- D-03: Maximum 3 retries per request (4 total attempts including initial)
- D-04: After all retries exhausted, return error code 1001 with `{"error": {"type": "upstream_error", "message": "All upstreams failed", "code": 1001}}`
- D-05: If no upstreams enabled at all, return error code 1001 immediately without retry

**Failover State Tracking:**
- D-06: Add `RetryAttempt int` and `RetryCount int` fields to `RequestLog` struct
- D-07: Log entry written for each retry attempt with upstream name and retry number
- D-08: Final failure log entry shows all retries that were attempted

**TUI Upstream Management (TUI-05, TUI-06, TUI-07):**
- D-09: Press `a` to enter "add upstream" inline form mode
- D-10: Press `e` to edit selected upstream (or Enter on selected)
- D-11: Press `d` to delete selected upstream with "Press Enter to confirm" prompt
- D-12: Upstream form fields: Name, URL, API Key, Auth Type (bearer/x-api-key), Timeout (seconds), Enabled (toggle)
- D-13: Form validation: Name and URL required, URL must be valid http/https, Timeout minimum 5s
- D-14: TUI maintains mutable `upstreams` slice -- changes take effect immediately in LoadBalancer

**TUI Keyboard Navigation (TUI-08):**
- D-15: Arrow keys (↑/↓) navigate upstream list when in navigation mode
- D-16: `a`/`e`/`d` keys trigger actions on selected upstream (when not in form mode)
- D-17: `Esc` cancels current form or returns to navigation mode
- D-18: `q` or `ctrl+c` initiates graceful shutdown confirmation (not immediate quit)

**Graceful Shutdown (TUI-09):**
- D-19: Press `q` or `ctrl+c` → show confirmation dialog: "Shutdown? [y/n]"
- D-20: `y` or `Enter` confirms shutdown, any other key cancels
- D-21: On confirm: stop accepting new requests, wait for in-flight requests (max 10s timeout), then exit

**Integration with Existing Code:**
- D-22: `ProxyHandler` gains `SelectNext()` method on LoadBalancer to get next upstream after failure
- D-23: `ProxyHandler` gains retry loop with exponential backoff using `time.Sleep`
- D-24: TUI `Update()` handles new message types: `UpstreamAdded`, `UpstreamDeleted`, `UpstreamUpdated`
- D-25: Shared config state protected by mutex for concurrent reads during TUI edits

### Claude's Discretion (Open for Implementation Decisions)
- Exact lipgloss colors for retry state (e.g., warning color for mid-retry)
- Form input field ordering and default values
- Whether to show retry attempts as separate log lines or aggregated
- Confirmation dialog styling (border, placement)

### Deferred Ideas (OUT OF SCOPE)
None -- Phase 2 scope is well-defined.

---

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| FAIL-01 | On 5xx or timeout, automatically switch to next upstream | Retry loop pattern in proxy.go using SelectNext() |
| FAIL-02 | Retry with exponential backoff (1s → 2s → 4s) | time.Sleep with exponential backoff pattern |
| FAIL-03 | Maximum 3 retries per request | Retry counter with maxRetries=3 |
| FAIL-04 | If all upstreams fail, return proper error with code 1001 | Error format: `{"error": {"type": "upstream_error", "message": "All upstreams failed", "code": 1001}}` |
| TUI-05 | Allow adding new upstream via TUI | Add form mode with tea.KeyMsg handling |
| TUI-06 | Allow editing existing upstream via TUI | Edit form mode pre-populated with selected upstream |
| TUI-07 | Allow deleting upstream via TUI | Delete confirmation with "Press Enter to confirm" |
| TUI-08 | Support keyboard navigation (↑/↓ to select, a/e/d for actions) | tea.KeyMsg arrow key detection in Update() |
| TUI-09 | Press q or ctrl+c to gracefully shutdown | Confirmation dialog before shutdown, http.Server.Shutdown(ctx) |

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go native net/http | 1.25.3 | HTTP server and client | Standard library, zero dependencies |
| Go context | stdlib | Timeout and cancellation | For detecting deadline exceeded |
| Go sync.Mutex | stdlib | Protecting shared state | D-25: mutex for TUI-Proxy config sharing |
| time.Sleep | stdlib | Backoff delays | D-02: exponential backoff 1s→2s→4s |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| charmbracelet/bubbletea | v1.3.10 | TUI framework | Elm-like architecture for TUI |
| charmbracelet/lipgloss | v1.1.0 | TUI styling | Lipgloss styles already in tui.go |

**Installation:**
No new external dependencies required -- all features use standard library or existing project dependencies.

---

## Architecture Patterns

### Recommended Project Structure
```
agent-router/
├── main.go          # Entry point, graceful shutdown wiring
├── config.go       # Config struct (no changes needed)
├── upstream.go     # Add SelectNext() to LoadBalancer
├── proxy.go        # Add retry loop, SelectNext() call
└── tui.go          # Add form state, keyboard handling, confirmation dialog
```

### Pattern 1: Exponential Backoff Retry Loop

**What:** Retry failed requests with increasing delays (1s, 2s, 4s)
**When to use:** FAIL-01, FAIL-02, FAIL-03

**Implementation pattern:**
```go
// Source: Go standard library + established patterns
const (
    baseDelay    = 1 * time.Second
    maxRetries   = 3
    maxDelay     = 4 * time.Second
)

func (h *ProxyHandler) proxyWithRetry(w http.ResponseWriter, r *http.Request, requestID string) {
    enabled := h.lb.GetEnabled()
    if len(enabled) == 0 {
        h.writeError(w, http.StatusBadGateway, "upstream_error", "No upstream available", 1001)
        return
    }

    var lastErr error
    retryCount := 0
    delay := baseDelay

    for attempt := 0; attempt <= maxRetries; attempt++ {
        upstream := h.lb.SelectNext(enabled, lastSelected) // needs implementation
        if upstream == nil {
            upstream = enabled[0] // wrap around
        }

        err := h.proxyRequest(w, r, upstream, requestID, attempt, retryCount)
        if err == nil {
            return // success
        }
        lastErr = err

        // Check if retryable: 5xx OR timeout
        if !isRetryable(err, resp) {
            break
        }

        if attempt < maxRetries {
            time.Sleep(delay)
            delay *= 2
            if delay > maxDelay {
                delay = maxDelay
            }
        }
    }

    // All retries exhausted
    h.writeError(w, http.StatusBadGateway, "upstream_error", "All upstreams failed", 1001)
}

func isRetryable(err error, resp *http.Response) bool {
    if err != nil {
        // Timeout check: url.Error.Timeout() or context.DeadlineExceeded
        var urlErr *url.Error
        if errors.As(err, &urlErr) && urlErr.Timeout() {
            return true
        }
        if errors.Is(err, context.DeadlineExceeded) {
            return true
        }
        return false
    }
    if resp != nil && resp.StatusCode >= 500 {
        return true
    }
    if resp != nil && resp.StatusCode == 429 {
        return true
    }
    return false
}
```

**Key insight:** Use `url.Error.Timeout()` to detect HTTP client timeouts (set via `Client.Timeout`), and `context.DeadlineExceeded` for context-based timeouts. The existing code uses `upstream.Timeout` on the `http.Client` -- this sets a client-side timeout that returns a timeout error.

### Pattern 2: LoadBalancer.SelectNext() for Failover

**What:** Get next upstream after current one fails
**When to use:** FAIL-01, FAIL-04

**Implementation:**
```go
// Add to upstream.go LoadBalancer
// SelectNext returns the next upstream after 'after', wrapping to first if at end
func (lb LoadBalancer) SelectNext(after *Upstream) *Upstream {
    if len(lb.upstreams) == 0 {
        return nil
    }
    if after == nil {
        return lb.upstreams[0]
    }
    for i, us := range lb.upstreams {
        if us == after {
            // Return next, wrapping to 0 if at end
            next := (i + 1) % len(lb.upstreams)
            return lb.upstreams[next]
        }
    }
    return lb.upstreams[0] // 'after' not found, start from beginning
}
```

### Pattern 3: TUI Form State Machine

**What:** Handle keyboard input for navigation vs form modes
**When to use:** TUI-05, TUI-06, TUI-07, TUI-08

**Model state expansion:**
```go
type model struct {
    // ... existing fields ...

    // Navigation state
    selectedIndex int

    // Form state (nil when not in form mode)
    formMode    string // "", "add", "edit", "delete"
    formData    Upstream
    formField   int    // current field index for focus
    confirmMode bool   // for delete confirmation
}
```

**Update() keyboard handling:**
```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if m.formMode != "" {
            // Form mode: handle field navigation and input
            return m.handleFormInput(msg)
        }
        if m.confirmMode {
            // Confirmation mode
            return m.handleConfirm(msg)
        }
        // Navigation mode: handle arrow keys and action keys
        switch msg.String() {
        case "up":
            if m.selectedIndex > 0 {
                m.selectedIndex--
            }
        case "down":
            if m.selectedIndex < len(m.upstreams)-1 {
                m.selectedIndex++
            }
        case "a":
            m.formMode = "add"
            m.formData = Upstream{Enabled: true, Timeout: 30, AuthType: "bearer"}
            m.formField = 0
        case "e", "enter":
            if len(m.upstreams) > 0 {
                m.formMode = "edit"
                m.formData = *m.upstreams[m.selectedIndex]
                m.formField = 0
            }
        case "d":
            if len(m.upstreams) > 0 {
                m.confirmMode = true
            }
        case "q", "ctrl+c":
            m.confirmMode = true // Show shutdown confirmation
        case "esc":
            // Cancel form or return to navigation
            if m.formMode != "" {
                m.formMode = ""
            }
        }
    }
    return m, nil
}
```

### Pattern 4: Graceful Shutdown

**What:** Wait for in-flight requests before exiting
**When to use:** TUI-09

**Implementation in main.go:**
```go
// Add graceful shutdown channel
shutdownChan := make(chan struct{})

// TUI confirmation sends to this channel
go func() {
    <-p.Quit()
    shutdownChan <- struct{}{}
}()

// Or handle via tea.Quit
<-shutdownChan

// Graceful shutdown with 10s timeout (D-21)
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

if err := server.Shutdown(ctx); err != nil {
    fmt.Fprintf(os.Stderr, "Shutdown error: %v\n", err)
}
```

### Pattern 5: Mutex-Protected Shared State

**What:** Protect LoadBalancer upstreams slice during TUI edits
**When to use:** D-25, TUI-14

**Implementation:**
```go
// In main.go or new shared state struct
type SharedState struct {
    mu        sync.RWMutex
    upstreams []*Upstream
}

func (s *SharedState) GetUpstreams() []*Upstream {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.upstreams
}

func (s *SharedState) SetUpstreams(upstreams []*Upstream) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.upstreams = upstreams
}
```

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Timeout detection | Custom timer tracking | `url.Error.Timeout()` or `context.DeadlineExceeded` | Built into Go's HTTP client and context packages |
| Exponential backoff | Third-party retry library | Simple `time.Sleep` with doubling delay | 3 retries with 1s/2s/4s is simple enough |
| Graceful shutdown | Manual connection tracking | `http.Server.Shutdown(ctx)` | Built into Go standard library, handles in-flight requests |
| Concurrent state | Manual channel-based sync | `sync.RWMutex` | Simple read-heavy workload, mutex is clearer |

---

## Common Pitfalls

### Pitfall 1: Timeout Detection Missing `url.Error.Timeout()`
**What goes wrong:** Timeouts not detected, requests return success even when upstream timed out
**Why it happens:** `http.Client.Timeout` returns wrapped errors -- need `url.Error.Timeout()` to detect
**How to avoid:** Check both `urlErr.Timeout()` and `errors.Is(err, context.DeadlineExceeded)`
**Warning signs:** Requests that should retry on timeout instead succeed

### Pitfall 2: LoadBalancer.Select() Not Failover-Aware
**What goes wrong:** After failover, hash-based selection re-selects the same failed upstream
**Why it happens:** Current `Select()` uses hash modulo -- doesn't track which upstream just failed
**How to avoid:** `SelectNext(after *Upstream)` skips the failed one explicitly
**Warning signs:** Retry requests hit same failing upstream repeatedly

### Pitfall 3: TUI State Not Protected During Proxy Reads
**What goes wrong:** Concurrent map access panic when TUI edits upstreams while proxy reads them
**Why it happens:** TUI goroutine edits slice while proxy goroutines read from it
**How to avoid:** RWMutex protecting shared upstreams slice (D-25)
**Warning signs:** `fatal error: concurrent map read and map write` panic

### Pitfall 4: Shutdown Without In-Flight Wait
**What goes wrong:** In-flight requests killed during shutdown
**Why it happens:** Calling `server.Close()` instead of `server.Shutdown(ctx)`
**How to avoid:** Use `server.Shutdown(ctx)` with sufficient timeout (D-21: 10s)
**Warning signs:** Clients report interrupted requests on shutdown

### Pitfall 5: Form State Bleed Between Modes
**What goes wrong:** Previous form data visible when entering new form
**Why it happens:** Not resetting form state when switching modes
**How to avoid:** Clear `formData` and `formField` when entering any form mode
**Warning signs:** Pre-populated fields on add mode

---

## Code Examples

### Retry Logic with Exponential Backoff (proxy.go expansion)
```go
// Source: Go net/http documentation
func (h *ProxyHandler) proxyRequest(w http.ResponseWriter, r *http.Request, upstream *Upstream, requestID string, retryAttempt, retryCount int) error {
    // ... existing proxy logic ...
    // On failure, return error for retry loop to handle
    if err != nil {
        return err
    }
    return nil
}
```

### TUI Confirmation Dialog (D-19, D-20)
```go
// Add to View()
func (m model) View() string {
    var s string
    // ... existing view ...

    if m.confirmMode {
        s += "\n"
        s += styleError.Render("╭───────────────────────────────────────╮\n")
        s += styleError.Render("│  Shutdown? [y/n]                     │\n")
        s += styleError.Render("╰───────────────────────────────────────╯\n")
    }

    return s
}
```

### Shared State Between TUI and Proxy
```go
// main.go
type AppState struct {
    mu        sync.RWMutex
    upstreams []*Upstream
}

var appState = &AppState{}

func main() {
    // Initialize with config upstreams
    appState.upstreams = lb.GetEnabled()

    // Pass to both TUI and Proxy
    proxyHandler := NewProxyHandler(appState, cfg.Service.APIKey, logChan)
}
```

---

## Open Questions

1. **TUI-14: How does ProxyHandler get updated upstreams after TUI edit?**
   - What we know: D-25 says "mutex-protected shared state"
   - What's unclear: Whether to use channel-based updates (TUI sends message to Proxy) or direct mutex access
   - Recommendation: Use mutex-protected pointer to LoadBalancer, TUI updates in-place

2. **Should retry log entries be separate lines or aggregated?**
   - What we know: D-07 says "Log entry written for each retry attempt"
   - What's unclear: Aggregate view vs detailed view in TUI log display
   - Recommendation: Each retry gets its own log line with retry number; TUI aggregates in stats

---

## Environment Availability

Step 2.6: SKIPPED (no external dependencies beyond existing Go toolchain)

- Go 1.25.3: Available
- All dependencies already in go.mod (bubbletea v1.3.10, lipgloss v1.1.0)

---

## Sources

### Primary (HIGH confidence)
- Go standard library `net/http` -- http.Server.Shutdown(), http.Client.Do(), url.Error.Timeout()
- Go standard library `context` -- context.DeadlineExceeded, context.WithTimeout()
- Go standard library `sync` -- sync.Mutex, sync.RWMutex
- Go standard library `time` -- time.Sleep for backoff
- Project CLAUDE.md -- bubbletea v1.x, lipgloss v2.x, go.mod dependencies verified

### Secondary (MEDIUM confidence)
- Existing codebase patterns verified (proxy.go, upstream.go, tui.go, main.go)
- CONTEXT.md decisions (D-01 through D-25) -- user-confirmed locked decisions

---

## Metadata

**Confidence breakdown:**
- Standard Stack: HIGH -- verified via go.mod, all locked decisions from CONTEXT.md
- Architecture: HIGH -- patterns match existing codebase structure (single-file Go, tea.Model)
- Pitfalls: MEDIUM -- identified from Go/net/http best practices, verified via Go docs

**Research date:** 2026-04-04
**Valid until:** 2026-05-04 (30 days -- stable domain)
