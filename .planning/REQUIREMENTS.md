# Requirements: Agent Router

**Defined:** 2026-04-03
**Core Value:** Claude Code 请求永不中断 — 多上游自动切换保障可用性，负载均衡优化成本

## v1 Requirements

### Core API

- [x] **CORE-01**: Service exposes POST /v1/messages endpoint compatible with Claude official SDK
- [x] **CORE-02**: Service authenticates requests via x-api-key or Bearer token
- [x] **CORE-03**: Service accepts and forwards all standard Claude message types
- [x] **CORE-04**: Service returns standard Claude response format with usage metadata

### Upstream Management

- [x] **UPST-01**: Service manages multiple upstream providers (Zhipu, Aicodee, Minimax)
- [x] **UPST-02**: Each upstream has configurable URL, API key, auth type (bearer/x-api-key)
- [x] **UPST-03**: Each upstream has enable/disable toggle
- [x] **UPST-04**: Each upstream has configurable timeout (default 30s)

### Load Balancing

- [x] **LB-01**: Requests distributed using modulo hash algorithm
- [x] **LB-02**: Hash based on request ID or client IP for even distribution
- [x] **LB-03**: Distribution均匀分布 across enabled upstreams

### Failover

- [x] **FAIL-01**: On 5xx or timeout, automatically switch to next upstream
- [x] **FAIL-02**: Retry with exponential backoff (1s → 2s → 4s)
- [x] **FAIL-03**: Maximum 3 retries per request
- [x] **FAIL-04**: If all upstreams fail, return proper error with code 1001

### Usage Tracking

- [ ] **USAGE-01**: Track total request count (success + failure)
- [ ] **USAGE-02**: Track input/output tokens per request
- [ ] **USAGE-03**: Track per-upstream request counts
- [ ] **USAGE-04**: Store usage data in local SQLite (usage.db)
- [ ] **USAGE-05**: Async writes to SQLite to not block requests

### TUI Interface

- [x] **TUI-01**: Display service status (name, version, port, uptime)
- [x] **TUI-02**: Display channel list with status (enabled/disabled) and request counts
- [x] **TUI-03**: Display real-time request log with latency and token usage
- [x] **TUI-04**: Display total usage statistics (tokens, requests, success rate)
- [ ] **TUI-05**: Allow adding new upstream via TUI
- [ ] **TUI-06**: Allow editing existing upstream via TUI
- [ ] **TUI-07**: Allow deleting upstream via TUI
- [ ] **TUI-08**: Support keyboard navigation (↑/↓ to select, a/e/d for actions)
- [ ] **TUI-09**: Press q or ctrl+c to gracefully shutdown

### Config Hot Reload

- [ ] **CONF-01**: Reload config on SIGHUP signal
- [ ] **CONF-02**: Reload config via TUI button
- [ ] **CONF-03**: Reload config via POST /admin/reload API
- [ ] **CONF-04**: Support adding new upstream channels dynamically
- [ ] **CONF-05**: Support removing upstream channels dynamically
- [ ] **CONF-06**: Support enabling/disabling channels dynamically

### Admin API

- [ ] **ADMIN-01**: GET /admin/status returns service status with usage stats
- [ ] **ADMIN-02**: POST /admin/reload triggers config hot reload

## v2 Requirements

Deferred to future release.

### Enhanced TUI

- **TUI-10**: Drag-and-drop channel priority ordering
- **TUI-11**: Historical usage charts
- **TUI-12**: Alert thresholds and notifications

### Enhanced Reliability

- **HEALTH-01**: Periodic health checks on all upstreams
- **HEALTH-02**: Automatic disable of unhealthy upstreams
- **HEALTH-03**: Automatic re-enablement when upstream recovers

### Metrics

- **METR-01**: Prometheus-compatible metrics endpoint
- **METR-02**: Per-channel latency percentiles
- **METR-03**: Cost estimation per upstream

## Out of Scope

| Feature | Reason |
|---------|--------|
| Non-Claude API endpoints | 仅支持 `/v1/messages` 作为 API 中转 |
| Cloud deployment | 纯本地运行方案 |
| User authentication | 内部使用工具 |
| OAuth / SSO | 简单 API key 鉴权足够 |
| Video/audio processing | 非 LLM API 代理范围 |
| Multi-tenancy | 单用户本地工具 |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| CORE-01 | Phase 1 | Complete |
| CORE-02 | Phase 1 | Complete |
| CORE-03 | Phase 1 | Complete |
| CORE-04 | Phase 1 | Complete |
| UPST-01 | Phase 1 | Complete |
| UPST-02 | Phase 1 | Complete |
| UPST-03 | Phase 1 | Complete |
| UPST-04 | Phase 1 | Complete |
| LB-01 | Phase 1 | Complete |
| LB-02 | Phase 1 | Complete |
| LB-03 | Phase 1 | Complete |
| TUI-01 | Phase 1 | Complete |
| TUI-02 | Phase 1 | Complete |
| TUI-03 | Phase 1 | Complete |
| TUI-04 | Phase 1 | Complete |
| FAIL-01 | Phase 2 | Complete |
| FAIL-02 | Phase 2 | Complete |
| FAIL-03 | Phase 2 | Complete |
| FAIL-04 | Phase 2 | Complete |
| TUI-05 | Phase 2 | Pending |
| TUI-06 | Phase 2 | Pending |
| TUI-07 | Phase 2 | Pending |
| TUI-08 | Phase 2 | Pending |
| TUI-09 | Phase 2 | Pending |
| USAGE-01 | Phase 3 | Pending |
| USAGE-02 | Phase 3 | Pending |
| USAGE-03 | Phase 3 | Pending |
| USAGE-04 | Phase 3 | Pending |
| USAGE-05 | Phase 3 | Pending |
| CONF-01 | Phase 3 | Pending |
| CONF-02 | Phase 3 | Pending |
| CONF-03 | Phase 3 | Pending |
| CONF-04 | Phase 3 | Pending |
| CONF-05 | Phase 3 | Pending |
| CONF-06 | Phase 3 | Pending |
| ADMIN-01 | Phase 3 | Pending |
| ADMIN-02 | Phase 3 | Pending |

**Coverage:**
- v1 requirements: 37 total
- Mapped to phases: 37
- Unmapped: 0

---
*Requirements defined: 2026-04-03*
*Last updated: 2026-04-03 after roadmap creation*
