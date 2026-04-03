# Project Research Summary

**Project:** Go Local API Proxy/Router
**Domain:** Local Claude API Router/Proxy Service
**Researched:** 2026-04-03
**Confidence:** MEDIUM

## Executive Summary

This project is a local API proxy that routes LLM requests to multiple upstream providers (Anthropic, OpenAI, OpenRouter) while presenting an OpenAI-compatible interface. Experts build such proxies using Go's native net/http as the foundation, layering GORM + SQLite for usage tracking, and charmbracelet/bubbletea for the terminal dashboard. The recommended approach prioritizes a minimal MVP with the OpenAI-compatible endpoint and single upstream passthrough, then layers on load balancing, failover, and the TUI dashboard in subsequent phases.

The key risks are concurrency-related: unbounded goroutines under load, race conditions on config reloads, and SQLite write contention blocking HTTP handlers. These must be addressed in Phase 2 before any load testing. The TUI and HTTP server must run in separate goroutines from the start to avoid blocking. Research confidence is MEDIUM overall due to training data reliance and unverified version numbers.

## Key Findings

### Recommended Stack

**From STACK.md (Confidence: MEDIUM)**

Go's standard library plus three key ecosystem libraries form the core stack. The native `net/http` package handles HTTP server and client duties with zero dependencies. GORM v1.25.x with the official SQLite driver provides usage tracking without external database requirements. The charmbracelet stack (bubbletea v1.x + lipgloss v2.x) creates the terminal dashboard with an Elm-like declarative architecture.

**Core technologies:**
- **Go native net/http 1.21+**: HTTP server and client — zero dependencies, production-proven
- **GORM v1.25.x + gorm.io/driver/sqlite**: ORM for SQLite usage tracking — standard Go ORM with excellent SQLite support
- **charmbracelet/bubbletea v1.x**: TUI framework — declarative Elm-like architecture
- **charmbracelet/lipgloss v2.x**: TUI styling — composable styles with 256-color support
- **gopkg.in/yaml.v3 + fsnotify**: Config management and hot reload — standard Go YAML parsing with file watching

### Expected Features

**From FEATURES.md (Confidence: LOW — web tools unavailable)**

Users expect these table stakes features: OpenAI-compatible `/v1/chat/completions` endpoint, single upstream proxy passthrough, API key validation, basic request logging, upstream health checks, and static YAML configuration. Without these, the product feels incomplete.

**Must have (table stakes):**
- OpenAI-compatible `/v1/chat/completions` endpoint — existing code expects this API shape
- Single upstream proxy passthrough — core value proposition
- API key passthrough — security baseline, don't break existing auth
- Basic logging to stdout — debuggability
- Static YAML configuration — standard local-tool expectation

**Should have (competitive differentiators):**
- Real-time TUI dashboard — visual feedback distinguishes CLI tools from services
- Automatic failover with health checks — resilience without user intervention
- Round-robin load balancing — spread traffic across providers
- Cost tracking per upstream — budget visibility

**Defer (v2+):**
- Dynamic config reload — complex with concurrent requests, risky
- Per-model routing — provider support matrix adds complexity
- Rate limiting per consumer — multi-tenant complexity
- Streaming support — SSE/chunked responses, stateful

### Architecture Approach

**From ARCHITECTURE.md (Confidence: HIGH)**

The architecture follows a layered pattern: HTTP Server and TUI run concurrently, passing state through channels to a shared middleware chain and router core. The router core contains the load balancer, provider manager, and retry logic. SQLite handles async, non-blocking usage tracking via a background goroutine.

**Major components:**
1. **HTTP Server (net/http)**: Accepts requests, applies middleware, routes to handlers
2. **TUI Renderer (bubbletea)**: Displays dashboard, logs, provider status in terminal
3. **Middleware Chain**: Auth, logging, rate-limiting as composable http.Handler decorators
4. **Router Core**: Load balancing + retry logic with provider health tracking
5. **SQLite Tracker**: Async usage recording with WAL mode for concurrency

Key patterns: Middleware Chain (decorator pattern), Provider Pool with Health Checks, Retry with Exponential Backoff + jitter, Concurrent TUI and HTTP via channels, SQLite WAL mode for non-blocking writes.

### Critical Pitfalls

**From PITFALLS.md (Confidence: MEDIUM)**

1. **HTTP Response Body Not Drained**: Every upstream response body must be fully consumed and closed. Undrained bodies cause connection exhaustion and "too many open files" errors.
2. **Race Condition on Config Reload**: Go maps are not goroutine-safe. Config reads during active requests concurrent with reloads cause panics. Use `sync.RWMutex` or `atomic.Value`.
3. **Unbounded Goroutine Spawning**: Without semaphore limits, concurrent requests spawn unlimited goroutines for upstream calls. Use context cancellation and worker pools.
4. **SQLite Writes Blocking HTTP Handlers**: SQLite serializes writes. Synchronous writes block HTTP responses. Queue writes to a goroutine channel.
5. **TUI Blocking HTTP Event Loop**: TUI rendering on the main thread blocks HTTP handling. Run HTTP server in a separate goroutine.

## Implications for Roadmap

Based on research, suggested phase structure:

### Phase 1: Foundation — HTTP Server + TUI + Single Provider
**Rationale:** Core functionality must exist before adding complexity. TUI and HTTP must run concurrently from the start to avoid architectural rework.
**Delivers:** Working HTTP server with OpenAI-compatible endpoint, single upstream passthrough, API key passthrough, basic logging, static YAML config, manual failover via restart, TUI dashboard showing provider status.
**Addresses:** Table stakes features from FEATURES.md
**Avoids:** Pitfall #5 (TUI blocking HTTP) — HTTP and TUI run in separate goroutines from start

### Phase 2: Resilience — Connection Pooling + Failover + Health Checks
**Rationale:** Load testing will expose concurrency issues. Connection pooling and goroutine limits are foundational before testing at scale. Health checks enable automatic failover.
**Delivers:** Tuned HTTP transport with proper idle connection limits, request timeouts on all upstream calls, health check system, automatic failover with retry logic, bounded goroutine spawning via semaphore, composite error logging with upstream details.
**Addresses:** Differentiators — automatic failover, retry with backoff, load balancing readiness
**Avoids:** Pitfalls #1 (response body draining), #3 (unbounded goroutines), #6 (connection pool misconfig), #7 (no timeout), #8 (silent error swallowing), #9 (hash collision at startup)

### Phase 3: Persistence + Polish — SQLite + Graceful Shutdown + Config Hot Reload
**Rationale:** SQLite integration requires careful async design. Graceful shutdown must be tested before production. Hot reload requires config race condition fixes.
**Delivers:** SQLite usage tracking with WAL mode and async writes, graceful shutdown with in-flight request draining, dynamic config reload with goroutine-safe access, TUI updates via channels (not polling).
**Avoids:** Pitfalls #2 (config race), #4 (SQLite blocking), #10 (missing graceful shutdown)

### Phase Ordering Rationale

- **Phase 1 before 2**: TUI architecture must be correct from start. Foundation features (endpoint, passthrough, config) are prerequisites for failover testing.
- **Phase 2 before 3**: Concurrency fixes must be in place before integrating SQLite. Load testing Phase 2 reveals whether goroutine limits and connection pooling work.
- **Load Balancing deferred**: Round-robin is listed as v1.x feature but should come in Phase 2 after health checks. Cannot balance intelligently without knowing provider health.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 2 (Failover/Retry)**: Retry logic edge cases — need to verify backoff behavior under various failure modes, test against actual provider rate limits
- **Phase 3 (SQLite)**: WAL mode tuning for write-heavy workloads — may need benchmarking to determine optimal batch sizes

Phases with standard patterns (skip research-phase):
- **Phase 1 (Foundation)**: HTTP server patterns well-documented in Go standard library
- **Phase 1 (TUI)**: Bubbletea patterns well-documented in Charmbracelet docs

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | MEDIUM | Training data; verify versions via `go list -m -versions` before implementation |
| Features | LOW | Web search unavailable; based on training data analysis of competitors |
| Architecture | HIGH | Go standard library patterns well-documented; community consensus |
| Pitfalls | MEDIUM | Go best practices documented; specific error patterns from community post-mortems |

**Overall confidence:** MEDIUM

### Gaps to Address

- **Stack versions**: Recommend running `go list -m -versions` for all dependencies before implementation to verify version compatibility
- **Feature competitive analysis**: Unable to verify via web search. Recommend direct analysis of LiteLLM, LocalAI, and PortKey documentation before roadmap finalization
- **Provider API specifics**: Request/response transformation requirements for each upstream provider need live testing to confirm
- **Streaming support**: Researched as v2 feature but actual implementation complexity unknown without testing against real provider streaming responses

## Sources

### Primary (HIGH confidence)
- Go standard library documentation (net/http, context, database/sql) — official, verified
- Effective Go concurrency patterns — official Go documentation
- GORM documentation (gorm.io) — established documentation

### Secondary (MEDIUM confidence)
- STACK.md — Training data analysis, recommended versions should be verified
- ARCHITECTURE.md — Go community consensus on patterns, bubbletea internals MEDIUM confidence
- PITFALLS.md — Community post-mortems, standard library patterns, some inference

### Tertiary (LOW confidence)
- FEATURES.md — Web tools unavailable during research; competitive analysis based on training data; needs live verification

---
*Research completed: 2026-04-03*
*Ready for roadmap: yes*
