# Phase 3: Persistence - Research

**Researched:** 2026-04-04
**Domain:** Go SQLite persistence (GORM), hot config reload (SIGHUP/TUI/API), admin HTTP API
**Confidence:** HIGH

## Summary

Phase 3 adds three major feature areas: (1) SQLite usage tracking with async writes, (2) config hot reload via SIGHUP signal/TUI button/HTTP API, and (3) admin HTTP endpoints for status and reload. The existing codebase (Go 1.25.3, bubbletea v1.3.10) needs two new dependencies: GORM v1.25.x + gorm.io/driver/sqlite for SQLite, and fsnotify for file watching. The existing `RequestLog` struct in `proxy.go` needs expansion to include token tracking, and the async write pattern follows the goroutine-channel pattern identified in PITFALLS.md.

**Primary recommendation:** Add GORM dependency, create `UsageLog` model, spawn a background worker that drains `usageChan chan UsageLog`, implement `doReload()` as the single function invoked by SIGHUP/TUI/API, and add `/admin/*` routes to the existing HTTP server mux.

---

## User Constraints (from CONTEXT.md)

### Locked Decisions

**SQLite Schema & ORM (USAGE-04):**
- D-01: Use GORM v1.25.x with gorm.io/driver/sqlite for SQLite operations
- D-02: Per-request log table: timestamp, request_id, upstream_name, input_tokens, output_tokens, latency_ms, status_code
- D-03: Index on timestamp and upstream_name for efficient queries

**Usage Tracking Model (USAGE-01, USAGE-02, USAGE-03):**
- D-04: Full per-request logging: input_tokens, output_tokens, latency_ms, upstream_name, status_code
- D-05: Asynchronous writes via goroutine channel (per STATE.md prior decision: "SQLite writes will be async via goroutine channel")
- D-06: Usage data survives service restart (persisted to usage.db)

**Async Write Implementation (USAGE-05):**
- D-07: ProxyHandler logs to a goroutine channel, background worker drains and writes to SQLite
- D-08: SQLite writes do NOT block HTTP response (async, fire-and-forget with error logging)

**Config Hot Reload (CONF-01, CONF-02, CONF-03):**
- D-09: SIGHUP signal triggers config reload: re-read config.yaml, reinitialize LoadBalancer
- D-10: TUI button click triggers same reload function
- D-11: POST /admin/reload triggers same reload function
- D-12: All three triggers (SIGHUP, TUI, API) invoke identical reload logic

**Dynamic Upstream Changes (CONF-04, CONF-05, CONF-06):**
- D-13: TUI add/edit/delete/enable/disable changes take effect immediately in LoadBalancer
- D-14: TUI changes persist only in-memory (NOT to config.yaml) -- runtime-only
- D-15: New upstreams from TUI are added to SharedUpstreams and LoadBalancer immediately
- D-16: Deleted upstreams are removed from SharedUpstreams and LoadBalancer immediately

**Admin API (ADMIN-01, ADMIN-02):**
- D-17: GET /admin/status returns: service_name, version, uptime, total_requests, total_tokens_in, total_tokens_out, per_upstream_counts, enabled_channels list
- D-18: POST /admin/reload triggers hot config reload (same as SIGHUP)
- D-19: Admin endpoints use same authentication as /v1/messages (x-api-key or Bearer token)

### Claude's Discretion (Open for Implementation Decisions)

- Exact GORM model struct field names and tags
- Channel buffer size for async writes (default unbounded, log errors if full)
- TUI reload button placement and styling
- /admin/status JSON response structure details
- Error handling when config.yaml is missing/corrupt on reload

### Deferred Ideas (OUT OF SCOPE)

None -- Phase 3 scope is well-defined.

---

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| USAGE-01 | Track total request count (success + failure) | UsageLog model + aggregation queries |
| USAGE-02 | Track input/output tokens per request | UsageLog.InputTokens, OutputTokens fields |
| USAGE-03 | Track per-upstream request counts | UsageLog.UpstreamName field + GROUP BY query |
| USAGE-04 | Store usage data in local SQLite (usage.db) | GORM + gorm.io/driver/sqlite, WAL mode |
| USAGE-05 | Async writes to SQLite to not block requests | Goroutine channel worker pattern |
| CONF-01 | Reload config on SIGHUP signal | signal.Notify(sigChan, syscall.SIGHUP) |
| CONF-02 | Reload config via TUI button | TUI key handler sends reload message |
| CONF-03 | Reload config via POST /admin/reload API | HTTP handler calls doReload() |
| CONF-04 | Support adding new upstream channels dynamically | Already implemented in Phase 2 (D-13, D-15) |
| CONF-05 | Support removing upstream channels dynamically | Already implemented in Phase 2 (D-13, D-16) |
| CONF-06 | Support enabling/disabling channels dynamically | Already implemented in Phase 2 (D-13) |
| ADMIN-01 | GET /admin/status returns service status with usage stats | HTTP handler + SQLite aggregation |
| ADMIN-02 | POST /admin/reload triggers config hot reload | HTTP handler calls doReload() |

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|--------|---------|--------------|
| Go native net/http | 1.25.3 | HTTP server & client | Already in use, standard library |
| gorm.io/gorm | v1.25.x | ORM for SQLite operations | De facto standard for Go ORMs |
| gorm.io/driver/sqlite | v1.5.x | SQLite driver for GORM | Official GORM SQLite driver |
| github.com/mattn/go-sqlite3 | v1.14.x | SQLite driver (CGO) | Required by gorm SQLite driver |
| github.com/fsnotify/fsnotify | v1.7.x | File system watching | Hot config reload via SIGHUP |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| gopkg.in/yaml.v3 | v3.0.1 | YAML config parsing | Already in use |

**Installation:**
```bash
go get gorm.io/gorm@v1.25.11
go get gorm.io/driver/sqlite@v1.5.7
go get github.com/mattn/go-sqlite3@v1.14.28
go get github.com/fsnotify/fsnotify@v1.7.0
```

Note: gorm.io/driver/sqlite@v1.5.7 requires mattn/go-sqlite3. CGO is acceptable for this local tool (per CLAUDE.md).

**Version verification:** As of research date (2026-04-04), training data suggests GORM v1.25.11 and gorm.io/driver/sqlite v1.5.7 are current. Verify at implementation via `go list -m -versions gorm.io/gorm`.

---

## Architecture Patterns

### Recommended Project Structure
```
agent-router/
├── main.go           # Entry point, signal handling, start usage worker
├── config.go         # Config struct and LoadConfig (no changes needed)
├── proxy.go         # Add InputTokens/OutputTokens to RequestLog, usageChan logging
├── upstream.go      # SharedUpstreams already exists (Phase 2)
├── tui.go           # Add reload button, reload message type
├── usage.go         # NEW: UsageLog model, usage worker, SQLite operations
└── admin.go         # NEW: Admin HTTP handlers (/admin/status, /admin/reload)
```

### Pattern 1: GORM SQLite Model with Async Write Worker

**What:** Persist usage data to SQLite without blocking HTTP handlers
**When to use:** USAGE-01 through USAGE-05

**UsageLog model (D-02):**
```go
// Source: GORM v1 documentation + D-02 decision
type UsageLog struct {
    ID            uint      `gorm:"primaryKey"`
    Timestamp     time.Time `gorm:"index:idx_timestamp_upstream"`
    RequestID     string    `gorm:"index"`
    UpstreamName  string    `gorm:"index:idx_timestamp_upstream"`
    InputTokens   int       `gorm:"default:0"`
    OutputTokens  int       `gorm:"default:0"`
    LatencyMs     int64
    StatusCode    int
}
```

**Async write worker (D-05, D-07, D-08):**
```go
// Source: Pitfalls research (Pitfall 4), D-07, D-08
// usage.go

type UsageStats struct {
    mu             sync.RWMutex
    totalRequests  int64
    totalTokensIn  int64
    totalTokensOut int64
}

var stats = &UsageStats{}

func StartUsageWorker(db *gorm.DB, usageChan <-chan RequestLog) {
    go func() {
        for log := range usageChan {
            // Parse tokens from log if available (log already has tokens in Phase 3)
            ul := UsageLog{
                Timestamp:    log.Timestamp,
                RequestID:    log.RequestID,
                UpstreamName: log.UpstreamName,
                InputTokens:  log.InputTokens,
                OutputTokens: log.OutputTokens,
                LatencyMs:    log.LatencyMs,
                StatusCode:   log.StatusCode,
            }
            if err := db.Create(&ul).Error; err != nil {
                fmt.Fprintf(os.Stderr, "Usage write error: %v\n", err)
            } else {
                // Update in-memory stats (fire-and-forget, non-blocking)
                stats.mu.Lock()
                stats.totalRequests++
                stats.totalTokensIn += int64(log.InputTokens)
                stats.totalTokensOut += int64(log.OutputTokens)
                stats.mu.Unlock()
            }
        }
    }()
}
```

**Modified RequestLog (D-04 -- add InputTokens/OutputTokens):**
```go
// In proxy.go, add to existing RequestLog struct:
type RequestLog struct {
    Timestamp     time.Time
    LatencyMs     int64
    UpstreamName  string
    StatusCode    int
    RequestID     string
    RetryAttempt  int
    RetryCount    int
    InputTokens   int   // NEW
    OutputTokens  int   // NEW
}
```

**Token extraction from upstream response:**
Claude API responses include usage in the `usage` field of the response body. The proxy currently passes through the response as-is, so token extraction requires reading the response body. Two approaches:

1. **Parse response body for usage field** (more accurate):
```go
// Extract usage from Claude response before copying to client
var respData map[string]interface{}
if err := json.NewDecoder(resp.Body).Decode(&respData); err == nil {
    if usage, ok := respData["usage"].(map[string]interface{}); ok {
        if inputTokens, ok := usage["input_tokens"].(float64); ok {
            latencyMs = int64(inputTokens) // store for logging
        }
        // ... extract output_tokens ...
    }
}
// Re-encode for client
```

2. **Pass through and extract from log** (simpler but less accurate):
Claude SDK includes usage in response metadata -- the proxy can log the raw response for later analysis. Token extraction from response body adds latency due to decode/re-encode.

Recommendation: For Phase 3, implement token extraction by peeking at the response body before passing through. The response body must still be passed to the client after extraction.

### Pattern 2: Single Config Reload Function (D-12)

**What:** SIGHUP, TUI button, and POST /admin/reload all call identical reload logic
**When to use:** CONF-01, CONF-02, CONF-03

**Implementation:**
```go
// Source: D-09 through D-12
// admin.go or main.go

// doReload reads config.yaml, reinitializes LoadBalancer, returns error
func doReload() error {
    configPath := filepath.Join(filepath.Dir(execPath), "config.yaml")
    newCfg, err := LoadConfig(configPath)
    if err != nil {
        return fmt.Errorf("failed to load config: %w", err)
    }

    // Re-initialize LoadBalancer with new config
    newUpstreams := NewLoadBalancer(newCfg.Upstreams)

    // Update shared upstreams (thread-safe via mutex)
    sharedUpstreams.mu.Lock()
    sharedUpstreams.upstreams = newUpstreams.GetEnabled()
    sharedUpstreams.mu.Unlock()

    // Update proxy handler's LoadBalancer reference
    proxyHandler.lb = newUpstreams

    fmt.Println("Config reloaded successfully")
    return nil
}
```

**SIGHUP handler:**
```go
// Source: D-09
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGHUP)
go func() {
    for range sigChan {
        fmt.Println("Received SIGHUP, reloading config...")
        if err := doReload(); err != nil {
            fmt.Fprintf(os.Stderr, "Reload failed: %v\n", err)
        }
    }
}()
```

**TUI reload message:**
```go
// Source: D-10
// In tui.go, add message type:
type ReloadRequest struct{}
type ReloadComplete struct{ Error error }

// In TUI Update():
case ReloadRequest:
    go func() {
        err := doReload()
        p.Send(ReloadComplete{Error: err})
    }()

// Add 'r' key in navigation mode to trigger reload
case "r":
    return m, func() tea.Msg { return ReloadRequest{} }
```

### Pattern 3: Admin HTTP Handlers (ADMIN-01, ADMIN-02)

**What:** GET /admin/status and POST /admin/reload endpoints
**When to use:** ADMIN-01, ADMIN-02, CONF-03

**Auth (D-19):** Same auth as /v1/messages -- extract token from x-api-key or Authorization: Bearer header.

**GET /admin/status (D-17):**
```go
// Source: D-17
type AdminStatus struct {
    ServiceName    string                  `json:"service_name"`
    Version        string                  `json:"version"`
    Uptime         string                  `json:"uptime"`
    TotalRequests  int64                   `json:"total_requests"`
    TotalTokensIn  int64                  `json:"total_tokens_in"`
    TotalTokensOut int64                  `json:"total_tokens_out"`
    PerUpstream    map[string]UpstreamStats `json:"per_upstream_counts"`
    EnabledChannels []string              `json:"enabled_channels"`
}

type UpstreamStats struct {
    RequestCount int   `json:"request_count"`
    TotalTokensIn int64 `json:"total_tokens_in"`
    TotalTokensOut int64 `json:"total_tokens_out"`
}

func (h *ProxyHandler) handleAdminStatus(w http.ResponseWriter, r *http.Request) {
    // Auth check (D-19)
    if !h.authenticate(r) {
        h.writeError(w, http.StatusUnauthorized, "authentication_error", "Invalid API key", 0)
        return
    }

    stats.mu.RLock()
    totalReqs := stats.totalRequests
    totalIn := stats.totalTokensIn
    totalOut := stats.totalTokensOut
    stats.mu.RUnlock()

    // Query per-upstream counts from SQLite
    var upstreamCounts []struct {
        UpstreamName string
        Count        int
        TokensIn     int64
        TokensOut    int64
    }
    db.Model(&UsageLog{}).
        Select("upstream_name, COUNT(*) as count, SUM(input_tokens) as tokens_in, SUM(output_tokens) as tokens_out").
        Group("upstream_name").
        Scan(&upstreamCounts)

    perUpstream := make(map[string]UpstreamStats)
    for _, uc := range upstreamCounts {
        perUpstream[uc.UpstreamName] = UpstreamStats{
            RequestCount:  uc.Count,
            TotalTokensIn: uc.TokensIn,
            TotalTokensOut: uc.TokensOut,
        }
    }

    enabledChannels := make([]string, 0)
    for _, us := range h.lb.GetEnabled() {
        enabledChannels = append(enabledChannels, us.Name)
    }

    status := AdminStatus{
        ServiceName:     cfg.Service.Name,
        Version:         cfg.Service.Version,
        Uptime:          time.Since(startTime).String(),
        TotalRequests:   totalReqs,
        TotalTokensIn:   totalIn,
        TotalTokensOut:  totalOut,
        PerUpstream:     perUpstream,
        EnabledChannels: enabledChannels,
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(status)
}

func (h *ProxyHandler) handleAdminReload(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        h.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST allowed", 0)
        return
    }
    if !h.authenticate(r) {
        h.writeError(w, http.StatusUnauthorized, "authentication_error", "Invalid API key", 0)
        return
    }

    if err := doReload(); err != nil {
        h.writeError(w, http.StatusInternalServerError, "reload_error", err.Error(), 0)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w, Encode(map[string]string{"status": "reloaded"}))
}
```

**HTTP routing for admin endpoints:**
The existing code uses `proxyHandler` as the HTTP handler directly. Admin endpoints can be added via a custom ServeHTTP that dispatches:
```go
func (h *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    switch {
    case r.Method == http.MethodPost && r.URL.Path == "/admin/reload":
        h.handleAdminReload(w, r)
    case r.Method == http.MethodGet && r.URL.Path == "/admin/status":
        h.handleAdminStatus(w, r)
    case r.URL.Path == "/v1/messages" && r.Method == http.MethodPost:
        h.handleMessages(w, r)
    default:
        http.NotFound(w, r)
    }
}
```

### Pattern 4: TUI Reload Button

**What:** Add reload functionality to TUI navigation bar
**When to use:** CONF-02, D-10

**TUI changes:**
- Add 'r' key in navigation mode to trigger reload
- Add ReloadRequest/ReloadComplete message types
- Display reload status in navigation bar or status area

```go
// In tui.go Update():
case tea.KeyMsg:
    switch msg.String() {
    // ... existing navigation ...
    case "r":
        // Trigger reload
        return m, func() tea.Msg { return ReloadRequest{} }
    }

// Handle reload response:
case ReloadComplete:
    if msg.Error != nil {
        // Could show error in status bar
    }
```

### Pattern 5: SQLite WAL Mode for Async Writes

**What:** Enable WAL journal mode for better concurrent read/write performance
**When to use:** USAGE-04, USAGE-05

```go
// Source: Pitfalls research (Pitfall 4: SQLite without WAL)
// In usage.go initDB():
func initDB(dbPath string) (*gorm.DB, error) {
    db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
    if err != nil {
        return nil, err
    }

    // Enable WAL mode for better concurrent performance
    db.Exec("PRAGMA journal_mode=WAL")
    db.Exec("PRAGMA synchronous=NORMAL")

    // Auto-migrate schema
    db.AutoMigrate(&UsageLog{})

    return db, nil
}
```

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| SQLite write serialization | Custom queue with mutex | GORM channel worker pattern | WAL mode + single writer goroutine avoids locked errors |
| Config reload sync | fsnotify file watching alone | SIGHUP signal is sufficient | File modification doesn't always fire events; SIGHUP is the standard Unix hot-reload signal |
| Admin auth | Custom auth middleware | Reuse existing token check | D-19 explicitly reuses /v1/messages auth |
| Token extraction | Ignore usage data | Parse response body usage field | USAGE-02 requires input/output tracking |

**Key insight:** The goroutine-channel async write pattern (D-05, D-07) directly addresses Pitfall 4 from PITFALLS.md. The single-writer model (one goroutine draining the channel) avoids `database is locked` errors that occur with concurrent GORM writes.

---

## Common Pitfalls

### Pitfall 1: SQLite WAL vs Locked Errors

**What goes wrong:** `database is locked` errors under write load
**Why it happens:** Default SQLite journal mode (DELETE) causes writer contention
**How to avoid:** Enable WAL mode + single writer goroutine (D-05, D-07)
**Warning signs:** Intermittent `database is locked` in usage writes

### Pitfall 2: Response Body Consumed Before Client Forward

**What goes wrong:** Client receives empty response after token extraction
**Why it happens:** `io.Copy(w, resp.Body)` drains the body after json.Decode already read it
**How to avoid:** Use `ioutil.ReadAll` to buffer body, decode for tokens, then write buffered bytes to client
**Warning signs:** Client gets empty response body, no usage logged

### Pitfall 3: Config Reload Race with In-Flight Requests

**What goes wrong:** Request in progress uses old config while reload updates LoadBalancer
**Why it happens:** No synchronization between reload and active requests
**How to avoid:** Use atomic.Value or RWMutex for LoadBalancer reference; proxyRequest uses snapshot of enabled upstreams at start
**Warning signs:** Inconsistent upstream selection mid-request

### Pitfall 4: Missing Auth on Admin Endpoints

**What goes wrong:** Admin endpoints accessible without authentication
**Why it happens:** Forgetting to apply same auth check as /v1/messages
**How to avoid:** Extract auth check into shared function, apply to all endpoints (D-19)
**Warning signs:** curl localhost:port/admin/status returns data without api-key

### Pitfall 5: Token Extraction Fails Silently

**What goes wrong:** Usage logs show 0 tokens for all requests
**Why it happens:** Claude API response format varies, JSON path `usage.input_tokens` may not exist
**How to avoid:** Log when token extraction fails, default to 0, don't fail the request
**Warning signs:** All UsageLog.InputTokens == 0 despite successful requests

---

## Code Examples

### Token Extraction from Claude Response (proxy.go modification)

```go
// Source: D-04, USAGE-02
// In proxy.go proxyRequest(), after receiving response:

// Buffer the response body for token extraction
bodyBytes, err := ioutil.ReadAll(resp.Body)
if err != nil {
    return err, statusCode
}

// Try to extract usage tokens (Claude API format)
var inputTokens, outputTokens int
var respData map[string]interface{}
if err := json.Unmarshal(bodyBytes, &respData); err == nil {
    if usage, ok := respData["usage"].(map[string]interface{}); ok {
        if v, ok := usage["input_tokens"].(float64); ok {
            inputTokens = int(v)
        }
        if v, ok := usage["output_tokens"].(float64); ok {
            outputTokens = int(v)
        }
    }
}

// Write buffered response to client
w.WriteHeader(statusCode)
w.Write(bodyBytes)

// Log with tokens
h.logToChan(requestID, latencyMs, upstream.Name, statusCode, retryAttempt, retryCount, inputTokens, outputTokens)
```

### SIGHUP Signal Handler (main.go modification)

```go
// Source: D-09
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGHUP)
go func() {
    for {
        <-sigChan
        fmt.Println("Received SIGHUP, reloading configuration...")
        if err := doReload(); err != nil {
            fmt.Fprintf(os.Stderr, "Config reload failed: %v\n", err)
        }
    }
}()
```

### TUI Reload Key Handler (tui.go modification)

```go
// Source: D-10
// Add message types at package level:
type ReloadRequest struct{}
type ReloadComplete struct{ Error error }

// In Update() switch:
case "r":
    return m, func() tea.Msg { return ReloadRequest{} }

// Handle reload completion:
case ReloadComplete:
    if msg.Error != nil {
        // Could flash error in status
    }
```

---

## Open Questions

1. **How to extract tokens from streaming responses?**
   - What we know: Claude API may return streaming responses with `data: {..."usage":{...}}` format
   - What's unclear: Whether streaming requires different token extraction logic
   - Recommendation: For Phase 3, only handle non-streaming responses. Log warning if streaming detected.

2. **Should doReload() notify TUI of successful reload?**
   - What we know: D-10 says "TUI button click triggers same reload function"
   - What's unclear: Whether TUI needs visual confirmation of reload success/failure
   - Recommendation: Send ReloadComplete message to TUI with error if any; TUI shows brief status message

3. **How to handle config.yaml missing on reload?**
   - What we know: D-09 says "re-read config.yaml" on SIGHUP
   - What's unclear: Whether to fall back to current config or report error
   - Recommendation: Report error, keep current config active (safer than switching to broken config)

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | All | Yes | 1.25.3 | N/A |
| SQLite | Usage DB | Yes | 3.44.4 | N/A |
| CGO | go-sqlite3 driver | Yes | Available | N/A (local tool acceptable) |
| fsnotify | SIGHUP detection | N/A (new dep) | v1.7.0 | Could use polling |
| GORM | Usage tracking | N/A (new dep) | v1.25.x | N/A |

**Missing dependencies with no fallback:**
- GORM v1.25.x and gorm.io/driver/sqlite v1.5.x -- must be installed via go get

**Missing dependencies with fallback:**
- fsnotify -- if unavailable, could use time.Ticker polling for config file modification (less efficient but functional)

---

## Sources

### Primary (HIGH confidence)
- Go standard library `net/http`, `os/signal`, `encoding/json`, `io/ioutil`
- Go standard library `sync` -- sync.RWMutex, sync.Mutex for stats
- GORM v1 documentation (gorm.io) -- model definition, AutoMigrate, Create
- gorm.io/driver/sqlite documentation -- WAL mode, connection handling
- Project CLAUDE.md -- GORM v1.25.x, gorm.io/driver/sqlite v1.5.x, fsnotify v1.7.x

### Secondary (MEDIUM confidence)
- Existing codebase patterns verified (proxy.go, upstream.go, tui.go, main.go)
- CONTEXT.md decisions (D-01 through D-19) -- user-confirmed locked decisions
- PITFALLS.md Pitfall 4 -- SQLite write serialization identified
- STACK.md -- fsnotify and GORM patterns documented

### Tertiary (LOW confidence)
- GORM version numbers (v1.25.11, v1.5.7) -- training data, verify at implementation via `go list -m -versions`

---

## Metadata

**Confidence breakdown:**
- Standard Stack: HIGH -- GORM + sqlite is established stack; CLAUDE.md specifies exact versions
- Architecture: HIGH -- patterns match existing codebase structure; channel-worker pattern aligns with PITFALLS.md
- Pitfalls: HIGH -- all pitfalls identified from verified sources (PITFALLS.md, Go docs)

**Research date:** 2026-04-04
**Valid until:** 2026-05-04 (30 days -- stable domain: GORM API, Go signal handling)
