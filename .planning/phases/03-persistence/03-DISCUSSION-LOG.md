# Phase 3: Persistence - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-04
**Phase:** 03-persistence
**Areas discussed:** SQLite Schema + ORM, Usage Tracking Model, Hot Reload Mechanism, Admin API Design

---

## SQLite Schema + ORM

| Option | Description | Selected |
|--------|-------------|----------|
| GORM (Recommended) | Recommended in CLAUDE.md, handles schema migrations, cleaner Go code | ✓ |
| Raw SQL | Direct sqlx queries, more control, less abstraction overhead | |

**User's choice:** GORM (Recommended)

---

## Schema Design

| Option | Description | Selected |
|--------|-------------|----------|
| Per-request log (Recommended) | RequestLog table: timestamp, request_id, upstream, tokens_in, tokens_out, latency_ms, status | ✓ |
| Aggregated counters | Counter table: upstream, date/hour, request_count, total_tokens_in, total_tokens_out | |

**User's choice:** Per-request log (Recommended)

---

## Usage Tracking Model

| Option | Description | Selected |
|--------|-------------|----------|
| Full per-request (Recommended) | Log every request: input_tokens, output_tokens, latency_ms, upstream_name, status_code | ✓ |
| Summary only | Just total counts + per-upstream counts | |

**User's choice:** Full per-request (Recommended)

---

## Hot Reload Mechanism (SIGHUP)

| Option | Description | Selected |
|--------|-------------|----------|
| SIGHUP triggers reload (Recommended) | Catch syscall.SIGHUP, reload config.yaml, reinitialize LoadBalancer | ✓ |
| SIGHUP + TUI button + API | CONF-01, CONF-02, CONF-03 all trigger same reload function | |

**User's choice:** SIGHUP triggers reload (Recommended)

---

## TUI Upstream Changes Persistence

| Option | Description | Selected |
|--------|-------------|----------|
| In-memory only (Recommended) | Phase 3 scope: config hot reload FROM file. TUI changes persist only while process runs. | ✓ |
| Persist to config.yaml | Write changes back to config.yaml. TUI becomes the config editor plus runtime state. | |

**User's choice:** In-memory only (Recommended)

---

## Admin API: GET /admin/status

| Option | Description | Selected |
|--------|-------------|----------|
| Status + usage stats (Recommended) | Return: service_name, version, uptime, total_requests, total_tokens_in, total_tokens_out, per_upstream counts | ✓ |
| Minimal status only | Just service_name, version, uptime, number of enabled channels | |

**User's choice:** Status + usage stats (Recommended)

---

## Admin API Authentication

| Option | Description | Selected |
|--------|-------------|----------|
| Same API key (Recommended) | Reuse existing x-api-key or Bearer auth | ✓ |
| Separate admin key | Require different API key for /admin/* endpoints | |

**User's choice:** Same API key (Recommended)

---

## Claude's Discretion

- Exact GORM model struct field names and tags
- Channel buffer size for async writes
- TUI reload button placement and styling
- /admin/status JSON response structure details
- Error handling when config.yaml is missing/corrupt on reload
