# Agent Router

## What This Is

本地轻量 API 中转服务，为 Claude Code 提供统一代理出口。对外暴露标准 Claude API（`/v1/messages`），无缝对接智谱、Aicodee、Minimax 三家上游，支持负载均衡、故障切换、配置热更新、用量监控。

## Core Value

Claude Code 请求永不中断 — 多上游自动切换保障可用性，负载均衡优化成本。

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] 负载均衡：取模算法路由请求到启用的上游渠道
- [ ] 故障切换：请求失败自动切换下一个上游，指数退避重试
- [ ] 配置热更新：SIGHUP / TUI 按钮 / API 触发重载
- [ ] 用量监控：SQLite 持久化存储，TUI 实时展示
- [ ] 标准 Claude API：兼容官方 SDK，对外暴露 `/v1/messages`
- [ ] 多渠道管理：动态添加/删除/启用/禁用上游渠道
- [ ] TUI 可视化界面：实时状态、日志、统计数据

### Out of Scope

- 非 Claude API 兼容接口 — 仅支持 `/v1/messages`
- 云端部署 — 纯本地运行方案
- 用户认证/权限管理 — 内部使用工具

## Context

- **技术栈**：Go 原生 `net/http` + bubbletea/lipgloss TUI + SQLite/GORM
- **部署环境**：本地运行，单二进制文件
- **数据存储**：本地 `usage.db` SQLite 文件

## Constraints

- **Tech stack**: Go 原生 + 极简第三方库，单文件可运行
- **Storage**: 本地 SQLite，无外部服务依赖
- **Compatibility**: 完全兼容 Claude 官方 SDK

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go 原生 net/http | 无框架、轻量、高性能 | — Pending |
| bubbletea + lipgloss TUI | 社区最流行轻量 TUI 库 | — Pending |
| 取模算法负载均衡 | O(1) 效率、哈希均匀分布 | — Pending |
| 指数退避重试 | 简单可靠，避免雪崩 | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd:transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd:complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-04-03 after initialization*
