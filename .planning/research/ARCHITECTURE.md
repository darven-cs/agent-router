# Architecture Research

**Domain:** Go API Proxy Service (Local Claude API Router)
**Researched:** 2026-04-03
**Confidence:** HIGH

## Standard Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         API Layer                                │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │
│  │   HTTP      │  │   TUI       │  │  Metrics    │              │
│  │   Server    │  │   Render    │  │  Endpoint   │              │
│  │  (net/http) │  │  (bubbletea)│  │  /metrics   │              │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘              │
│         │                │                │                     │
├─────────┴────────────────┴────────────────┴─────────────────────┤
│                     Middleware Chain                             │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐              │
│  │  Auth   │→ │ Logging │→ │ Rate    │→ │ Request │              │
│  │         │  │         │  │ Limit   │  │ ID      │              │
│  └────┬────┘  └────┬────┘  └────┬────┘  └────┬────┘              │
│       │            │            │            │                   │
├───────┴────────────┴────────────┴────────────┴──────────────────┤
│                      Router Core                                 │
│  ┌─────────────────────────────────────────────────────────┐      │
│  │              Load Balancer + Retry Logic                │      │
│  │   ┌─────────┐  ┌─────────┐  ┌─────────┐                │      │
│  │   │Provider1│  │Provider2│  │Provider3│                │      │
│  │   └─────────┘  └─────────┘  └─────────┘                │      │
│  └─────────────────────────────────────────────────────────┘      │
├─────────────────────────────────────────────────────────────────┤
│                     Data Layer                                    │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐            │
│  │   SQLite     │  │   Config     │  │   Usage      │            │
│  │   (journal)  │  │   Store      │  │   Tracker    │            │
│  └──────────────┘  └──────────────┘  └──────────────┘            │
└─────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Typical Implementation |
|-----------|----------------|------------------------|
| HTTP Server | Accept requests, apply middleware, route to handler | `net/http.Server` with custom `ServeMux` |
| TUI Renderer | Display status, logs, metrics in terminal | `github.com/charmbracelet/bubbletea` |
| Middleware Chain | Authenticate, log, rate-limit, add request context | Function adapters returning `http.Handler` |
| Load Balancer | Distribute requests across providers | Round-robin, weighted, or least-latency |
| Provider Manager | Track upstream health, manage connections | Interface with health checks |
| Retry Logic | Handle transient failures with backoff | Exponential backoff with jitter |
| SQLite Tracker | Record API usage per provider/customer | `database/sql` with WAL mode |
| Config Store | Manage providers, API keys, routing rules | In-memory with file persistence |

## Recommended Project Structure

```
cmd/
├── proxy/                 # Main proxy binary
│   └── main.go           # Entry point, signal handling
└── cli/                  # CLI tooling (optional)

internal/
├── proxy/
│   ├── handler.go        # HTTP request handlers
│   ├── middleware/
│   │   ├── auth.go       # API key validation
│   │   ├── logging.go    # Request/response logging
│   │   ├── ratelimit.go  # Rate limiting
│   │   └── requestid.go  # Request ID propagation
│   ├── router/
│   │   ├── balancer.go   # Load balancing strategies
│   │   ├── provider.go   # Upstream provider management
│   │   ├── retry.go      # Retry with backoff
│   │   └── router.go     # Main routing logic
│   └── transport/
│       └── client.go     # HTTP client for upstream calls
│
├── storage/
│   ├── sqlite.go         # SQLite connection and operations
│   ├── usage.go          # Usage tracking queries
│   └── config.go        # Configuration persistence
│
├── tui/
│   ├── app.go            # Bubbletea application
│   ├── views/
│   │   ├── dashboard.go  # Main dashboard view
│   │   ├── logs.go       # Log stream view
│   │   └── providers.go  # Provider status view
│   └── state.go          # TUI state management
│
└── config/
    └── config.go         # Configuration types and loading

pkg/
├── models/
│   ├── provider.go       # Provider model
│   ├── request.go        # API request/response models
│   └── usage.go          # Usage record model
│
└── logging/
    └── logger.go         # Structured logging setup
```

### Structure Rationale

- **cmd/proxy/:** Single binary entry point, easy deployment
- **internal/proxy/:** Core proxy logic, not importable by other modules
- **internal/storage/:** SQLite operations isolated for testing and migration
- **internal/tui/:** TUI separated to avoid importing terminal libs in API server
- **internal/proxy/middleware/:** Composable middleware, easy to add/remove
- **internal/proxy/router/:** Routing logic isolated from transport
- **pkg/models/:** Shared types between components
- **pkg/logging/:** Centralized logging configuration

## Architectural Patterns

### Pattern 1: Middleware Chain (Decorator Pattern)

**What:** Chain of http.HandlerFunc decorators that process requests before/after the core handler.

**When to use:** Cross-cutting concerns like auth, logging, rate-limiting.

**Trade-offs:** Simple to implement, but order matters and deep chains add latency.

**Example:**
```go
// Middleware type
type Middleware func(http.Handler) http.Handler

// Chain applies middlewares left-to-right
func Chain(h http.Handler, middlewares ...Middleware) http.Handler {
    for i := len(middlewares) - 1; i >= 0; i-- {
        h = middlewares[i](h)
    }
    return h
}

// Usage
handler := Chain(
    myHandler,
    withAuth,
    withLogging,
    withRateLimit,
)

// Implementation
func withAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if !validateAPIKey(r.Header.Get("Authorization")) {
            http.Error(w, "Unauthorized", 401)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

### Pattern 2: Provider Pool with Health Checks

**What:** Maintain a pool of upstream providers with health tracking and automatic failover.

**When to use:** Multiple API providers with failover requirements.

**Trade-offs:** More complex than single provider, but provides resilience.

**Example:**
```go
type Provider struct {
    Name    string
    BaseURL string
    APIKey  string
    Weight  int // for weighted routing

    mu           sync.RWMutex
    healthy      bool
    latencyP99   time.Duration
    failureCount int
}

type ProviderPool struct {
    providers []*Provider
    current   atomic.Int64 // round-robin index
}

func (p *ProviderPool) Next() *Provider {
    // Round-robin with health filtering
    n := int(p.current.Add(1))
    filtered := p.HealthyProviders()
    if len(filtered) == 0 {
        return p.providers[n % len(p.providers)] // fallback to all
    }
    return filtered[n % len(filtered)]
}

func (p *ProviderPool) HealthyProviders() []*Provider {
    p.mu.RLock()
    defer p.mu.RUnlock()
    healthy := make([]*Provider, 0)
    for _, prov := range p.providers {
        if prov.IsHealthy() {
            healthy = append(healthy, prov)
        }
    }
    return healthy
}
```

### Pattern 3: Retry with Exponential Backoff

**What:** Failed requests are retried with increasing delays and jitter.

**When to use:** Unreliable upstream services, network transient failures.

**Trade-offs:** Increases success rate but adds latency for failing requests.

**Example:**
```go
func WithRetry(ctx context.Context, fn func() error, maxRetries int) error {
    backoff := 100 * time.Millisecond

    for attempt := 0; attempt <= maxRetries; attempt++ {
        err := fn()
        if err == nil {
            return nil
        }

        // Don't retry on context cancellation or non-retryable errors
        if ctx.Err() != nil || !isRetryable(err) {
            return err
        }

        if attempt < maxRetries {
            // Exponential backoff with jitter
            jitter := time.Duration(rand.Int63n(int64(backoff / 2)))
            sleep := backoff + jitter

            select {
            case <-ctx.Done():
                return ctx.Err()
            case <-time.After(sleep):
            }

            backoff *= 2
            if backoff > 30*time.Second {
                backoff = 30 * time.Second
            }
        }
    }
    return fmt.Errorf("max retries exceeded")
}
```

### Pattern 4: Concurrent TUI and HTTP Server

**What:** Run HTTP server and TUI in separate goroutines with shared state via channels.

**When to use:** CLI tool that needs both API serving and terminal UI.

**Trade-offs:** Requires careful synchronization but provides real-time feedback.

**Example:**
```go
func Run() error {
    // Shared state via channels
    stateCh := make(chan UIState, 100)
    logCh := make(chan LogEntry, 1000)

    // Start HTTP server
    srv := &http.Server{Addr: ":8080", Handler: buildHandler(stateCh, logCh)}
    go func() {
        if err := srv.ListenAndServe(); err != http.ErrServerClosed {
            log.Fatal(err)
        }
    }()

    // Start TUI
    p := tea.NewProgram(app.New(stateCh, logCh))
    if _, err := p.Run(); err != nil {
        return fmt.Errorf("TUI error: %w", err)
    }

    // Shutdown HTTP server after TUI exits
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    return srv.Shutdown(ctx)
}
```

### Pattern 5: SQLite Usage Tracking with WAL Mode

**What:** Use SQLite with WAL mode for concurrent reads during writes.

**When to use:** Tracking API usage without blocking request handling.

**Trade-offs:** SQLite limits write concurrency; WAL helps but not unlimited.

**Example:**
```go
func InitDB(path string) (*sql.DB, error) {
    db, err := sql.Open("sqlite3", path)
    if err != nil {
        return nil, err
    }

    // Enable WAL mode for better concurrency
    if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
        return nil, err
    }

    // Increase connection pool for concurrent reads
    db.SetMaxOpenConns(10)
    db.SetMaxIdleConns(5)

    return db, nil
}

// Non-blocking usage record
func RecordUsage(ctx context.Context, db *sql.DB, usage UsageRecord) error {
    // Use separate goroutine to avoid blocking request
    errCh := make(chan error, 1)
    go func() {
        _, err := db.ExecContext(ctx,
            `INSERT INTO usage (provider, customer_id, tokens, latency_ms, timestamp)
             VALUES (?, ?, ?, ?, ?)`,
            usage.Provider, usage.CustomerID, usage.Tokens, usage.LatencyMs, time.Now())
        errCh <- err
    }()

    select {
    case err := <-errCh:
        return err
    case <-ctx.Done():
        return ctx.Err() // Don't let SQLite slow down the request
    }
}
```

## Data Flow

### Request Flow

```
Client Request
    │
    ▼
┌─────────────────┐
│  HTTP Server    │ Accept connection, read request
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Middleware    │ Auth → Log → RateLimit → RequestID
│  Chain          │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Router         │ Extract target, select provider
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Load Balancer  │ Select healthy provider (round-robin/weighted)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Retry Logic    │ Execute with backoff on failure
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Transport      │ Forward request to upstream
│  Client         │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  SQLite         │ Record usage (async, non-blocking)
└────────┬────────┘
         │
         ▼
Response ← Client
```

### State Management

```
┌──────────────────────────────────────────────────────┐
│                   Provider State                      │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐              │
│  │Healthy  │  │Latency  │  │Failures │              │
│  │ flag    │  │ p99     │  │ count   │              │
│  └────┬────┘  └────┬────┘  └────┬────┘              │
│       │            │            │                     │
│       └────────────┴────────────┘                     │
│                    │                                   │
│                    ▼ (RWMutex protect)                 │
│              Provider Manager                          │
└────────────────────────┬───────────────────────────────┘
                         │
                         ▼ (channel)
┌──────────────────────────────────────────────────────┐
│                    TUI State                          │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐              │
│  │Dashboard│  │Provider │  │  Log    │              │
│  │  View   │  │  View   │  │  View   │              │
│  └─────────┘  └─────────┘  └─────────┘              │
└──────────────────────────────────────────────────────┘
```

### Key Data Flows

1. **Request path:** HTTP Request → Middleware → Router → LoadBalancer → Provider → Response
2. **Usage tracking:** Request completion → Usage record → SQLite (async goroutine)
3. **Health updates:** Provider health check → State update → TUI notification via channel
4. **Log streaming:** Any component → Log channel → TUI LogView

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|--------------------------|
| 0-1k users | Monolith fine; single provider; SQLite adequate |
| 1k-10k users | Multiple providers; connection pooling; WAL mode critical |
| 10k-100k users | Redis instead of SQLite; provider autoscaling; circuit breakers |
| 100k+ users | Consider request queuing; dedicated metrics DB; CDN front |

### Scaling Priorities

1. **First bottleneck:** SQLite write contention
   - Fix: Move usage tracking to background goroutine, use WAL mode
2. **Second bottleneck:** Single provider saturation
   - Fix: Add provider pool with load balancing
3. **Third bottleneck:** TUI blocking on slow renders
   - Fix: Batch updates, use virtual scrolling

## Anti-Patterns

### Anti-Pattern 1: Blocking SQLite on Request Path

**What people do:** Execute `db.ExecContext` synchronously in the request handler.

**Why it's wrong:** SQLite locks on writes; slow writes block the response.

**Do this instead:** Queue usage records to a goroutine channel; write async.

```go
// BAD - blocks response
func handler(w http.ResponseWriter, r *http.Request) {
    resp := proxyRequest(r)
    db.Exec("INSERT INTO usage ...", resp) // BLOCKS!
    json.NewEncoder(w).Encode(resp)
}

// GOOD - non-blocking
func handler(w http.ResponseWriter, r *http.Request) {
    resp := proxyRequest(r)
    go recordUsage(resp) // async
    json.NewEncoder(w).Encode(resp)
}
```

### Anti-Pattern 2: No Provider Health Tracking

**What people do:** Always route to same provider or random provider without health awareness.

**Why it's wrong:** Failing provider causes cascading errors.

**Do this instead:** Track latency, failures, mark unhealthy providers; exclude from rotation.

### Anti-Pattern 3: Global Mutex for All Provider State

**What people do:** Use single `sync.Mutex` protecting entire provider pool.

**Why it's wrong:** All read operations block each other.

**Do this instead:** Use `sync.RWMutex` for read-heavy workloads, or atomics for simple counters.

```go
// BAD
type Pool struct {
    mu    sync.Mutex
    provs []*Provider
}

// GOOD
type Pool struct {
    mu    sync.RWMutex
    provs []*Provider
}

func (p *Pool) IsHealthy(name string) bool {
    p.mu.RLock() // Multiple readers OK
    defer p.mu.RUnlock()
    // ...
}
```

### Anti-Pattern 4: Retry Without Jitter

**What people do:** Retries with fixed backoff intervals.

**Why it's wrong:** Synchronized retries from multiple clients cause thundering herd.

**Do this instead:** Add random jitter to backoff windows.

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| Anthropic API | HTTP client with custom transport | Set timeouts, handle streaming |
| OpenRouter | HTTP client | Different endpoint structure |
| LocalAI | HTTP client | May need special headers |
| SQLite | database/sql driver | Use WAL mode, limit connections |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| HTTP Handler ↔ Router | Direct function call | Context propagation critical |
| Router ↔ Provider | Interface method | Easy to mock for testing |
| Provider ↔ SQLite | database/sql with goroutine | Non-blocking writes |
| HTTP Server ↔ TUI | Channels | Buffered channels prevent deadlock |

## Sources

- [net/http package documentation](https://pkg.go.dev/net/http) (HIGH)
- [Effective Go - Concurrency](https://go.dev/doc/effective_go#concurrency) (HIGH)
- [context package patterns](https://pkg.go.dev/context) (HIGH)
- [database/sql connection pooling](https://pkg.go.dev/database/sql) (HIGH)
- [bubbletea TUI framework](https://github.com/charmbracelet/bubbletea) (MEDIUM)
- [go-sqlite3 driver](https://github.com/mattn/go-sqlite3) (MEDIUM)

---
*Architecture research for: Go API Proxy Service*
*Researched: 2026-04-03*
