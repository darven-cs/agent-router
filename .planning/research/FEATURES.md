# Feature Research

**Domain:** Local Claude API Router/Proxy
**Researched:** 2026-04-03
**Confidence:** LOW

*Note: Web search and web fetch tools were unavailable during research. Findings are based on training data and may be incomplete or outdated. Recommend verification via Context7, official docs, or live competitive analysis before roadmap finalization.*

## Feature Landscape

### Table Stakes (Users Expect These)

Features users assume exist. Missing these = product feels incomplete.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| OpenAI-compatible `/v1/chat/completions` endpoint | Existing code expects this API shape; drop-in replacement | LOW | Must handle both `/v1/chat/completions` and `/v1/messages` for Anthropic | |
| Single upstream proxy pass-through | Basic proxy functionality is the core value prop | LOW | Transform request/response format between providers | |
| API key validation | Security baseline; prevents unauthorized access | LOW | Simple API key check or pass-through to upstream | |
| Basic request logging | Debugging support; users need to see what's happening | MEDIUM | Log requests/responses for debugging | |
| Upstream health check | Users need to know if providers are up | LOW | Simple ping/heartbeat to detect failures | |
| Static configuration file | Standard expectation for local tools | LOW | YAML/JSON config for provider endpoints | |

### Differentiators (Competitive Advantage)

Features that set the product apart. Not required, but valuable.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Real-time TUI dashboard with usage stats | Visual feedback distinguishes CLI tools from services | MEDIUM | Terminal UI showing request rates, latency, costs per provider | |
| Dynamic config reload (hot reload) | Zero-downtime config updates without restart | MEDIUM | Watch config file, reload on change; risky with concurrent requests | |
| Automatic failover with retry | Resilience without user intervention | MEDIUM | Circuit breaker pattern to detect bad upstream and switch | |
| Load balancing across providers | Cost optimization; spread traffic to cheapest provider | MEDIUM | Round-robin, least-latency, or cost-based routing | |
| Per-model routing | Route specific models to specific providers | MEDIUM | Provider support varies by model; e.g., only Anthropic has Claude 3.5 | |
| Request/response transformation | Handle provider-specific quirks | MEDIUM | Different providers have slightly different response formats | |
| Cost tracking per upstream | Budget visibility for multi-provider setups | LOW | Aggregate tokens and estimated costs per provider | |
| Rate limiting per consumer | Prevent single client from overwhelming upstream | MEDIUM | Token bucket or sliding window rate limiter | |

### Anti-Features (Commonly Requested, Often Problematic)

Features that seem good but create problems.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Full request/response body logging to disk | "Need audit trail" | Disk I/O kills performance; storage grows unbounded | Sample logging, structured metadata only, or optional async logging |
| Real-time streaming to multiple consumers | "WebSocket support" | Complexity explosion; streaming is stateful | Simple SSE or WebSocket proxy; handle multiplexing separately |
| Authentication beyond API keys | "Need OAuth, SSO, LDAP" | Scope creep; security surface area huge | Layer auth in front (nginx, auth proxy) rather than in tool |
| Provider health scoring algorithm | "Smart routing based on latency" | Over-engineering; simple failover works | Just detect failures and route around them |

## Feature Dependencies

```
[Load Balancing]
    └──requires──> [Upstream Health Check]
                       └──requires──> [Provider Registry]

[Dynamic Config Reload]
    └──requires──> [Request buffering/drain]  (to handle in-flight requests during reload)

[Failover]
    └──requires──> [Upstream Health Check]
                       └──requires──> [Retry Logic]

[TUI Dashboard]
    └──requires──> [Usage Statistics Collection]
    └──enhances──> [Request Logging]

[Rate Limiting] ──conflicts──> [Max throughput priority]  (pick one per deployment)
```

### Dependency Notes

- **Load Balancing requires Health Check:** Cannot balance intelligently without knowing who's healthy
- **Failover requires Health Check:** Must detect failure before failing over
- **Dynamic Config Reload requires Request Buffering:** Must drain in-flight requests before reloading to avoid state corruption
- **TUI Dashboard enhances Logging:** Visual feedback makes logs actionable
- **Rate Limiting conflicts with Max Throughput:** Rate limiting restricts QPS; if max throughput is goal, skip rate limiting

## MVP Definition

### Launch With (v1)

Minimum viable product — what's needed to validate the concept.

- [ ] **OpenAI-compatible `/v1/chat/completions` endpoint** — core value prop; if this doesn't work, nothing matters
- [ ] **Single upstream proxy pass-through** — basic functionality; transform format
- [ ] **API key passthrough** — security baseline; don't break existing auth
- [ ] **Basic request logging to stdout** — debuggability; print request metadata
- [ ] **Static YAML configuration** — standard local-tool expectation
- [ ] **Manual failover via config** — restart to switch upstream; acceptable for v1

### Add After Validation (v1.x)

Features to add once core is working.

- [ ] **Automatic failover with health check** — detect bad upstream, switch automatically
- [ ] **Load balancing (round-robin)** — spread across providers
- [ ] **TUI dashboard** — visual feedback, usage stats, logs
- [ ] **Cost tracking** — tokens and estimated cost per provider

### Future Consideration (v2+)

Features to defer until product-market fit is established.

- [ ] **Dynamic config reload** — hot reload without restart; complex with concurrent requests
- [ ] **Per-model routing** — route models to specific providers; provider support matrix complexity
- [ ] **Rate limiting per consumer** — prevents abuse; complexity in multi-tenant scenario
- [ ] **Streaming support** — SSE/chunked responses; stateful complexity

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| OpenAI-compatible endpoint | HIGH | LOW | P1 |
| Single upstream passthrough | HIGH | LOW | P1 |
| API key passthrough | HIGH | LOW | P1 |
| Basic logging to stdout | MEDIUM | LOW | P1 |
| Static YAML config | MEDIUM | LOW | P1 |
| Manual failover | MEDIUM | LOW | P1 |
| Automatic failover + health check | HIGH | MEDIUM | P2 |
| Round-robin load balancing | MEDIUM | MEDIUM | P2 |
| TUI dashboard | MEDIUM | MEDIUM | P2 |
| Cost tracking | MEDIUM | LOW | P2 |
| Dynamic config reload | MEDIUM | MEDIUM | P3 |
| Per-model routing | MEDIUM | HIGH | P3 |
| Rate limiting | LOW | MEDIUM | P3 |

**Priority key:**
- P1: Must have for launch
- P2: Should have, add when possible
- P3: Nice to have, future consideration

## Competitor Feature Analysis

| Feature | LiteLLM | LocalAI | OpenRouter | Our Approach |
|---------|---------|---------|------------|--------------|
| OpenAI-compatible API | YES | YES | YES | YES - core requirement |
| Multi-provider routing | YES | YES | YES | YES - local-first |
| Load balancing | YES (multiple strategies) | YES | YES | YES (round-robin for v1) |
| Automatic failover | YES (with health checks) | YES | YES | YES (health check + retry) |
| TUI dashboard | NO | NO | N/A (web) | YES - differentiates locally |
| Dynamic config reload | YES (via API) | YES (file watch) | N/A | YES (file watch) |
| Cost tracking | YES | NO | YES | YES (per-provider) |
| Rate limiting | YES | YES | YES | Optional (v2+) |
| Local-only mode | NO | YES | NO | YES - no cloud dependency |
| Streaming | YES | YES | YES | YES (future) |

**Key competitive insights:**
- LiteLLM is the closest analog; comprehensive but requires external DB for some features
- LocalAI is local-first but focuses on self-hosted model inference
- OpenRouter is cloud-only; good reference for routing logic but not applicable
- **Differentiator:** Local TUI + local-only operation without cloud dependency

## Sources

- Training data analysis of: LiteLLM documentation, LocalAI GitHub, OpenRouter
- LOW confidence: Unable to verify via web search or official docs during this research session
- Recommend direct competitive analysis of LiteLLM, LocalAI, and PortKey before finalizing roadmap

---

*Feature research for: Local Claude API Router*
*Researched: 2026-04-03*
*Confidence: LOW (web tools unavailable)*
