# Agent Router

## What This Is

本地轻量 API 中转服务，为 Claude Code 提供统一代理出口。对外暴露标准 Claude API（`/v1/messages`），无缝对接智谱、Aicodee、Minimax 三家上游，支持负载均衡、故障切换、配置热更新、用量监控、TUI 管理界面。

## Core Value

Claude Code 请求永不中断 — 多上游自动切换保障可用性，负载均衡优化成本。

## Requirements

### Validated

- ✓ 标准 Claude API 兼容 — v1.0 (POST /v1/messages, 官方 SDK 兼容)
- ✓ 多上游渠道管理 — v1.0 (Zhipu/Aicodee/Minimax, 动态添加/删除/启用/禁用)
- ✓ 取模哈希负载均衡 — v1.0 (均匀分布, O(1) 效率)
- ✓ TUI 可视化管理界面 — v1.0 (bubbletea, 实时状态/日志/统计, 键盘操作 CRUD)
- ✓ 故障切换与指数退避重试 — v1.0 (1s/2s/4s, SelectNext, 最多3次重试)
- ✓ SQLite 用量追踪 — v1.0 (异步 goroutine-channel 写入, per-request tokens/latency)
- ✓ 配置写回持久化 — v1.0 (SaveConfig(), TUI 变更持久化到 config.yaml)

### Active

- [ ] 配置热更新完整实现：SIGHUP / TUI 按钮 / POST /admin/reload (CONF-01/02/03)
- [ ] Admin API 完整路由：GET /admin/status, POST /admin/reload (ADMIN-01/02)

### Out of Scope

- 非 Claude API 兼容接口 — 仅支持 `/v1/messages`，需求明确无扩展计划
- 云端部署 — 纯本地运行方案，单二进制文件
- 用户认证/权限管理 — 内部使用工具，API key 鉴权足够
- OAuth / SSO — 过度设计
- 多租户 — 单用户本地工具
- Prometheus metrics — v2 考虑

## Context

- **技术栈**: Go 原生 `net/http` + bubbletea/lipgloss TUI + SQLite/GORM
- **代码规模**: ~1890 LOC Go, 7 files
- **部署环境**: 本地运行，单二进制文件
- **数据存储**: 本地 `usage.db` SQLite 文件
- **已交付**: v1.0 MVP — 3 phases, 7 plans, 21 tasks, 54 commits

## Constraints

- **Tech stack**: Go 原生 + 极简第三方库，单文件可运行
- **Storage**: 本地 SQLite，无外部服务依赖
- **Compatibility**: 完全兼容 Claude 官方 SDK

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go 原生 net/http | 无框架、轻量、高性能 | ✓ Good — 零依赖 HTTP 代理 |
| bubbletea + lipgloss v0.6.0 | 社区最流行轻量 TUI 库 | ✓ Good — lipgloss v2 tag 不规范, v0.6.0 稳定 |
| 取模算法负载均衡 | O(1) 效率、哈希均匀分布 | ✓ Good — 简单有效 |
| 指数退避重试 (1s/2s/4s) | 简单可靠，避免雪崩 | ✓ Good — 3 retries max |
| isRetryable 默认 false | 安全默认值, 仅对 timeout/5xx/429 重试 | ✓ Good — 避免对客户端错误重试 |
| SQLite 异步写入 via goroutine channel | 避免阻塞 HTTP 响应和 database locked | ✓ Good — 单 goroutine drain |
| Admin API 复用 x-api-key 鉴权 | 统一鉴权逻辑 | ✓ Good — 无额外复杂度 |
| Config write-back via SaveConfig() | TUI 变更需要持久化 | ✓ Good — 解决 doReload 覆盖问题 |

---
*Last updated: 2026-04-05 after v1.0 MVP milestone*
