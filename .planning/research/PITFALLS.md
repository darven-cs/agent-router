# Pitfalls Research

**Domain:** Go API Proxy Service (Local Claude Router)
**Researched:** 2026-04-03
**Confidence:** MEDIUM

*Note: WebSearch access denied. Findings based on Go best practices, standard library documentation, and established patterns. Low/Medium confidence on ecosystem-specific details (bubbletea internals).*

---

## Critical Pitfalls

### Pitfall 1: HTTP Client Never Closing Response Bodies

**What goes wrong:**
Memory leaks and connection exhaustion. Each request that doesn't drain `response.Body` holds a connection open indefinitely, causing "too many open files" errors at scale.

**Why it happens:**
The `http.Client` does not automatically drain response bodies. Developers often write:
```go
resp, err := client.Do(req)
if err != nil { return err }
// ... use resp ...
// Forgets: defer resp.Body.Close()
```

Even with `defer resp.Body.Close()`, if the body isn't read to EOF, the connection isn't returned to the pool.

**How to avoid:**
- Always use `io.Copy(os.Stdout, resp.Body)` or `io.DrainReader` pattern to fully consume the body
- Use `httputil.DumpResponse` for debugging (consumes and returns body bytes)
- Wrap client calls in a helper that always drains and closes:
```go
func doRequest(client *http.Client, req *http.Request) (*http.Response, error) {
    resp, err := client.Do(req)
    if err != nil { return nil, err }
    // Force drain on error path too
    if resp.StatusCode >= 400 {
        io.Copy(io.Discard, resp.Body)
    }
    resp.Body.Close()
    return resp, err
}
```

**Warning signs:**
- `too many open files` system errors
- Connections in `TIME_WAIT` state piling up
- Memory growing unbounded under load

**Phase to address:**
Phase 2 (HTTP Client & Connection Pooling) - Required before any upstream testing.

---

### Pitfall 2: Race Condition on Config Reload During Active Requests

**What goes wrong:**
Request hits router with config A, config reloads to config B mid-request, upstream selection uses inconsistent state, causing wrong upstream selection or nil pointer dereference.

**Why it happens:**
Go maps are not goroutine-safe. The config map is read on every request but written on reload. Without proper synchronization:
```go
var config *Config // read by handlers

func reload(newCfg *Config) {
    config = newCfg // RACE: concurrent read/write
}
```

**How to avoid:**
Use `sync.RWMutex` and copy-on-write pattern:
```go
type ConfigManager struct {
    mu     sync.RWMutex
    config atomic.Value // stores *Config
}

func (cm *ConfigManager) Get() *Config {
    return cm.config.Load().(*Config)
}

func (cm *ConfigManager) Reload(newCfg *Config) {
    cm.mu.Lock()
    defer cm.mu.Unlock()
    cm.config.Store(newCfg)
}
```
Or use `atomic.Value` directly for lock-free reads if only replacing entire config.

**Warning signs:**
- `fatal error: concurrent map read and map write`
- Random upstream selection changes mid-flight
- Occasional panics on config access

**Phase to address:**
Phase 3 (Config Hot Reload) - Config reload must be goroutine-safe before production use.

---

### Pitfall 3: Unbounded Goroutine Spawning on Concurrent Requests

**What goes wrong:**
Each incoming request spawns goroutines for all upstream attempts without limit. Under load, this creates thousands of goroutines, exhausting memory and CPU.

**Why it happens:**
Simple loop-based proxy:
```go
func handle(req) {
    for _, upstream := range upstreams {
        go func(u string) {
            callUpstream(u, req) // No limit, no context cancellation
        }(u)
    }
}
```

**How to avoid:**
- Use a `semaphore` or worker pool to limit concurrent upstream calls
- Use `context.WithCancel` to cancel all pending calls when one succeeds
- Forward cancel signal from client request context:
```go
ctx, cancel := context.WithCancel(req.Context())
defer cancel()

// All upstream goroutines share this context
// First success calls cancel() to stop others
```

**Warning signs:**
- Goroutine count grows linearly with request count
- 10K+ goroutines under load test
- Memory profiling shows goroutine stacks consuming stack

**Phase to address:**
Phase 2 (Concurrent Request Handling) - Must implement semaphore/goroutine limit before load testing.

---

### Pitfall 4: SQLite Writes Blocking HTTP Handlers

**What goes wrong:**
SQLite has a single-writer model. Write locks cause all HTTP handlers to block, creating latency spikes and potential deadlocks if SQLite is accessed from multiple goroutines incorrectly.

**Why it happens:**
GORM's default behavior opens a connection pool. Multiple goroutines writing simultaneously causes:
- `database is locked` errors
- Write operations serializing behind lock
- HTTP timeouts under write load

**How to avoid:**
- Use a single writer goroutine with channel-based work queue:
```go
type WriteRequest struct {
    query string
    args  []interface{}
    resp  chan error
}

var writeQueue = make(chan WriteRequest, 100)

func writeWorker() {
    db := getDB()
    for req := range writeQueue {
        req.resp <- db.Exec(req.query, req.args...).Error
    }
}
```
- Or use WAL mode and batch writes:
```go
db.Exec("PRAGMA journal_mode=WAL")
db.Exec("PRAGMA synchronous=NORMAL")
```
- Set `MaxOpenConns = 1` to force serialization if using GORM pool

**Warning signs:**
- `database is locked` errors in logs
- HTTP p99 latency spikes during writes
- Lock contention visible in profiler

**Phase to address:**
Phase 3 (SQLite Usage Persistence) - Write serialization must be designed before integrating SQLite.

---

### Pitfall 5: TUI Blocking HTTP Event Loop

**What goes wrong:**
Bubbletea's TUI runs in the same process. If TUI rendering blocks (e.g., slow terminal, massive log output), HTTP request handling is delayed, causing timeouts.

**Why it happens:**
Default bubbletea setup runs the TUI as the main event loop. HTTP server callbacks that do heavy work block the TUI, and vice versa.

**How to avoid:**
- Run HTTP server in separate goroutine:
```go
go func() {
    http.ListenAndServe(":8080", mux)
}()

// TUI runs on main thread
p := tea.NewProgram(model)
p.Run()
```
- Use `bubblegone` patterns with separate goroutines for HTTP
- Don't do heavy computation in TUI update functions
- Ship HTTP status updates via channel to TUI, don't poll

**Warning signs:**
- HTTP requests timing out during TUI redraws
- Terminal lag when logs scroll rapidly
- Intermittent 503s during TUI interaction

**Phase to address:**
Phase 1 (TUI + HTTP Server Integration) - Must verify HTTP and TUI run independently.

---

### Pitfall 6: Connection Pool Misconfiguration - Idle Connections Closed Prematurely

**What goes wrong:**
HTTP client's default `MaxIdleConns` and `MaxIdleConnsPerHost` cause connections to close and reopen frequently. Upstream sees high connection churn, latency spikes.

**Why it happens:**
Default `http.Transport` settings:
- `MaxIdleConns = 100` (total idle)
- `MaxIdleConnsPerHost = 2` (per upstream)

For a proxy hitting 3 upstreams, only 6 connections kept warm. Under burst load, connections thrash.

**How to avoid:**
Tune for your upstream count:
```go
transport := &http.Transport{
    MaxIdleConns:        100,
    MaxIdleConnsPerHost: 30, // Per upstream - keep more warm
    MaxConnsPerHost:     100, // Total per host limit
    IdleConnTimeout:     90 * time.Second,
    DialContext: (&net.Dialer{
        Timeout:   30 * time.Second,
        KeepAlive: 30 * time.Second,
    }).DialContext,
}
```

**Warning signs:**
- High connection establishment latency every few requests
- `dial: connection refused` errors when ramping up
- Upstream complaining about connection instability

**Phase to address:**
Phase 2 (HTTP Client & Connection Pooling) - Transport tuning is foundation layer.

---

### Pitfall 7: No Request Timeout on Upstream Calls

**What goes wrong:**
Slow upstream causes request to hang indefinitely. Client timeout (if any) fires, but proxy holds resources.

**Why it happens:**
Forgetting to set timeout on upstream request:
```go
req, _ := http.NewRequest("POST", upstream, body)
// Missing: req = req.WithContext(context.WithTimeout(req.Context(), 10*time.Second))
```

**How to avoid:**
Always wrap upstream requests with timeout context:
```go
ctx, cancel := context.WithTimeout(req.Context(), 10*time.Second)
defer cancel()
req = req.WithContext(ctx)
```

Combine with overall request timeout in router:
```go
// Total request time including all retries
ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
defer cancel()
```

**Warning signs:**
- Hanging requests in traces
- Goroutines in `syscall.RWLock` or `net.dial` states
- Load test stalls

**Phase to address:**
Phase 2 (Request Timeout Handling) - Timeouts must be on every upstream call.

---

### Pitfall 8: Silent Error Swallowing in Retry Logic

**What goes wrong:**
Retry loop catches errors but logs nothing when all retries fail, making debugging impossible. User sees generic timeout.

**Why it happens:**
```go
for i := 0; i < maxRetries; i++ {
    resp, err := callUpstream(upstream)
    if err == nil { return resp }
    // Falls through silently on final failure
}
return nil, fmt.Errorf("all retries failed") // No upstream details
```

**How to avoid:**
Collect all errors and return composite:
```go
var errs []error
for i := 0; i < maxRetries; i++ {
    resp, err := callUpstream(upstream)
    if err == nil { return resp }
    errs = append(errs, fmt.Errorf("attempt %d: %w", i+1, err))
}
return nil, fmt.Errorf("all retries exhausted: %v", errs)
```

**Warning signs:**
- Logs show "request failed" but no upstream details
- Users report generic errors with no actionable info
- Debugging requires adding temporary logging

**Phase to address:**
Phase 2 (Error Propagation) - Error handling is part of MVP foundation.

---

### Pitfall 9: Load Balancer Hash Collision at Startup

**What goes wrong:**
Modulo-based routing uses request count, not consistent hash. When upstreams are added/removed, ALL existing mappings shift, breaking in-flight requests.

**Why it happens:**
Simple modulo:
```go
upstream := upstreams[requestCount%len(upstreams)]
requestCount++
```
Adding one upstream mid-flight remaps every pending request to different upstream.

**How to avoid:**
- Use consistent hashing ring (jump hash, ketama)
- Or graceful: only remap NEW requests after config reload, existing requests keep old mapping
- Accept brief inconsistency during reload, log it

**Warning signs:**
- Requests suddenly routing to wrong upstream after config change
- Upstream receiving requests for non-existent sessions
- "Unknown session" errors spike after adding upstream

**Phase to address:**
Phase 2 (Load Balancing Algorithm) - Algorithm choice is fundamental.

---

### Pitfall 10: Missing Graceful Shutdown - In-Flight Requests Dropped

**What goes wrong:**
SIGTERM received, process exits immediately, in-flight requests return connection reset to clients.

**Why it happens:**
Default `http.Server` behavior:
```go
http.ListenAndServe(":8080", mux)
// Process exits, all in-flight requests die
```

**How to avoid:**
Implement graceful shutdown:
```go
srv := &http.Server{Addr: ":8080", Handler: mux}

go func() {
    srv.ListenAndServe()
}()

sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
<-sigChan

// Give in-flight requests 30s to complete
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
srv.Shutdown(ctx)
```

**Warning signs:**
- Clients seeing connection reset on deploy
- In-flight request metrics dropping to zero suddenly
- "connection closed" errors in client logs

**Phase to address:**
Phase 3 (Graceful Shutdown) - Required for production reliability.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| `http.DefaultClient` | No setup | Shared state, no tuning, global pollution | Never in production |
| Single global mutex for all config | Simple | All reads block each other | MVP only, must fix before load |
| In-memory queue for TUI logs | No complexity | Memory grows unbounded | MVP only, replace with ring buffer |
| fmt.Printf debugging | No deps | Can't disable, pollutes stdout | Development only |
| No structured logging | No learning curve | Unparseable logs | MVP only, add zerolog/zap early |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| Upstream APIs | Not respecting `Retry-After` header | Honor 429 backoff, use upstream-suggested delay |
| Claude SDK | Assuming streaming always works | Test both streaming and non-streaming paths |
| Terminal | Not handling resize events | Subscribe to window size changes in bubbletea |
| SQLite | Using database file path relative to CWD | Use absolute path or os.Executable() based path |
| Config file | Watching file without debounce | Debounce file system events (250ms) |

---

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| JSON marshal per retry | High CPU under load | Marshal once, reuse buffer | > 100 RPS |
| Logging every request | I/O bottleneck | Log sampling, aggregate stats | > 50 RPS |
| TUI redraw on every log | Terminal flickering, high CPU | Batch updates, 100ms render throttle | Any sustained logging |
| No connection keepalive | High latency, connection overhead | Set `Transport.ExpectContinueTimeout` | All production |
| SQLite without WAL | Write contention, locked errors | `PRAGMA journal_mode=WAL` | > 10 writes/sec |

---

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Logging full request/response bodies | Credential leakage | Scrub Authorization headers from logs |
| Not validating upstream SSL certificates | MITM on upstream | Configure `TLSClientConfig` with proper cert pool |
| Exposing internal config via API | Config leakage | Never expose raw config over management API |
| Not rate limiting management endpoints | DoS on local | TUI/API endpoints should require local access |
| Storing API keys in plaintext config | Key exposure | Use environment variable substitution |

---

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Generic "request failed" errors | No actionable info | Show which upstream failed and why |
| No visibility into retry behavior | Confusion during failover | Show retry count and upstream switching in TUI |
| Silent startup failures | User doesn't know it's broken | Fail fast with clear error message |
| No way to inspect current config | Can't verify settings | TUI should show active config summary |
| TTY required but started in background | Can't use TUI features | Detect terminal and fall back to CLI mode |

---

## "Looks Done But Isn't" Checklist

- [ ] **HTTP Client:** Creates client but doesn't drain response body — verify with load test
- [ ] **Timeouts:** Sets context timeout but doesn't pass to upstream request — check every call site
- [ ] **Config Reload:** Reloads config but doesn't use mutex/atomic — race condition exists
- [ ] **Retry Logic:** Retries on error but doesn't collect/return all errors — debugging impossible
- [ ] **SQLite:** Opens database but doesn't handle `database is locked` — will fail under load
- [ ] **Graceful Shutdown:** Listens for signals but doesn't drain requests — drops in-flight
- [ ] **TUI:** Runs in main goroutine but HTTP on background — works but architecture fragile

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Connection exhaustion | HIGH | Restart process, tune pool settings, implement circuit breaker |
| Config race condition | HIGH | Restart process, implement proper synchronization |
| SQLite locked | MEDIUM | Retry with backoff, switch to WAL mode, batch writes |
| Memory leak from undrained bodies | MEDIUM | Restart, add pprof monitoring, fix body draining |
| Wrong upstream selected | LOW | Config reload fixes, implement health checks |

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| HTTP Client response body draining | Phase 2 | Load test with pprof, check for growing goroutines |
| Config race condition | Phase 3 | `go test -race`, concurrent reload during requests |
| Unbounded goroutines | Phase 2 | Load test, verify goroutine count bounded |
| SQLite write blocking | Phase 3 | Write-heavy load test, check p99 latency |
| TUI blocking HTTP | Phase 1 | Concurrent HTTP load while interacting with TUI |
| Connection pool misconfig | Phase 2 | Connection reuse metrics, latency under load |
| No request timeout | Phase 2 | Test with slow upstream, verify timeout fires |
| Silent error swallowing | Phase 2 | Verify logs contain upstream details on failure |
| Load balancer hash collision | Phase 2 | Add upstream mid-request, verify only new requests remap |
| Missing graceful shutdown | Phase 3 | Send SIGTERM, verify in-flight requests complete |

---

## Sources

- Go standard library `net/http` documentation and source code
- `golang.org/x/net/http2` for HTTP/2 client insights
- GORM SQLite documentation (journal modes, locking)
- Bubbletea architecture patterns (Tea program lifecycle)
- Community post-mortems: "Go service memory leak from undrained response bodies"
- Stack Overflow: "database is locked SQLite GORM golang"
- "The Go Programming Language" ( Donovan & Kernighan ) - concurrency patterns

---

*Pitfalls research for: Local Claude API Router*
*Researched: 2026-04-03*
