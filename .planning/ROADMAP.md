# Roadmap: Agent Router

## Overview

Agent Router is a local API proxy that routes Claude Code requests to multiple upstream providers (Zhipu, Aicodee, Minimax) with automatic failover and load balancing. The journey builds foundation (working proxy), resilience (failover), and persistence (usage tracking + hot reload).

## Phases

- [x] **Phase 1: Foundation** - Core API proxy with single upstream routing and basic TUI status display
- [ ] **Phase 2: Resilience** - Automatic failover with exponential backoff and full TUI upstream management
- [ ] **Phase 3: Persistence** - SQLite usage tracking, config hot reload, and admin API

## Phase Details

### Phase 1: Foundation
**Goal**: Working API proxy that routes Claude SDK requests to a single upstream provider with basic TUI
**Depends on**: Nothing (first phase)
**Requirements**: CORE-01, CORE-02, CORE-03, CORE-04, UPST-01, UPST-02, UPST-03, UPST-04, LB-01, LB-02, LB-03, TUI-01, TUI-02, TUI-03, TUI-04
**Success Criteria** (what must be TRUE):
  1. Claude SDK requests to POST /v1/messages are proxied to configured upstream and return valid Claude responses
  2. Requests without valid x-api-key or Bearer token are rejected with 401
  3. Service starts, binds to configured port, and displays uptime in TUI
  4. TUI shows service name, version, port, and list of configured upstreams with enabled/disabled status
  5. Request log in TUI shows each request with latency and upstream response status
  6. Load balancer distributes requests evenly across enabled upstreams using modulo hash
**Plans**: 1 plan
- [x] 01-01-PLAN.md - Initialize project with working API proxy and basic TUI

### Phase 2: Resilience
**Goal**: Claude Code requests never fail due to upstream issues - automatic failover保障可用性
**Depends on**: Phase 1
**Requirements**: FAIL-01, FAIL-02, FAIL-03, FAIL-04, TUI-05, TUI-06, TUI-07, TUI-08, TUI-09
**Success Criteria** (what must be TRUE):
  1. When upstream returns 5xx or times out, request automatically retries with next upstream using exponential backoff (1s, 2s, 4s)
  2. After 3 failed retries across all upstreams, client receives error with code 1001
  3. TUI displays real-time failover events and current retry state
  4. User can add new upstream via TUI (a key) with name, URL, API key, auth type
  5. User can edit existing upstream via TUI (e key)
  6. User can delete upstream via TUI (d key) with confirmation
  7. User can navigate upstreams with arrow keys and perform actions with keyboard shortcuts
  8. Press q or ctrl+c triggers graceful shutdown with TUI shutdown confirmation
**Plans**: 2 plans
- [x] 02-01-PLAN.md - Failover logic with exponential backoff retry (Wave 1)
- [ ] 02-02-PLAN.md - TUI upstream management and graceful shutdown (Wave 2)

### Phase 3: Persistence
**Goal**: Usage data persisted to SQLite, config hot reload, admin API for operations
**Depends on**: Phase 2
**Requirements**: USAGE-01, USAGE-02, USAGE-03, USAGE-04, USAGE-05, CONF-01, CONF-02, CONF-03, CONF-04, CONF-05, CONF-06, ADMIN-01, ADMIN-02
**Success Criteria** (what must be TRUE):
  1. Total request count, input/output tokens, and per-upstream counts are stored in usage.db after each request
  2. SQLite writes happen asynchronously without blocking HTTP response
  3. After service restart, usage statistics are preserved and displayable
  4. Sending SIGHUP to process triggers config reload without restart
  5. TUI button click triggers config hot reload
  6. POST /admin/reload triggers config hot reload
  7. New upstream channels can be added dynamically without restart
  8. Upstream channels can be removed dynamically without restart
  9. Channels can be enabled/disabled dynamically without restart
  10. GET /admin/status returns service status and usage statistics
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation | 1/1 | Completed | 2026-04-04 |
| 2. Resilience | 1/2 | In Progress|  |
| 3. Persistence | 0/? | Not started | - |
