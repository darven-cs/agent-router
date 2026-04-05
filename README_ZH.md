# Agent Router

面向 Claude Code 的本地 API 中转服务，支持多上游（智谱、Aicodee、Minimax）自动切换、负载均衡、故障恢复、用量监控与 TUI 可视化界面。

**核心价值：** Claude Code 请求永不中断 — 多上游自动切换保障可用性，负载均衡优化成本。

[English](README.md) | [中文](README_ZH.md)

---

## 功能特性

- **兼容 Claude 官方 API** — 暴露 `POST /v1/messages` 接口，与 Claude SDK 完全兼容
- **多上游支持** — 配置多个上游渠道，支持独立启用/禁用
- **负载均衡** — FNV-1a 哈希取模算法，均衡分配请求
- **主上游优先** — 可设置首选上游，连续失败 3 次后自动切换
- **自动故障切换** — 超时/5xx/429 响应时指数退避重试
- **配置热更新** — SIGHUP 信号、TUI 按钮或管理 API 无需重启即可重载配置
- **实时 TUI 界面** — 监控服务状态、上游健康、请求日志
- **用量追踪** — 本地 SQLite 持久化记录每次请求的 token

## 快速开始

### 前置条件

- Go 1.21+
- 至少一个上游提供商的 API Key

### 构建

```bash
make deps
make build
```

### 运行

```bash
./agent-router
```

### 发送请求

```bash
curl -X POST http://localhost:6856/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: your-router-api-key" \
  -d '{
    "model": "claude-sonnet-4-6",
    "messages": [{"role": "user", "content": "你好"}]
  }'
```

### 管理 API

```bash
# 重载配置
curl -X POST http://localhost:6856/admin/reload \
  -H "x-api-key: your-router-api-key"

# 查看状态
curl http://localhost:6856/admin/status \
  -H "x-api-key: your-router-api-key"
```

## TUI 操作说明

| 按键 | 功能 |
|------|------|
| `↑` / `↓` | 在上游列表中导航 |
| `Space` | 切换上游启用/禁用 |
| `a` | 添加新上游 |
| `e` | 编辑选中的上游 |
| `d` | 删除选中的上游 |
| `m` | 选择主上游 |
| `r` | 重载 config.yaml |
| `q` | 退出程序 |

## 技术栈

| 组件 | 技术选型 |
|------|----------|
| HTTP 服务 | Go 原生 `net/http` |
| TUI 框架 | `charmbracelet/bubbletea` |
| TUI 样式 | `charmbracelet/lipgloss` |
| 配置解析 | `gopkg.in/yaml.v3` |
| 数据存储 | SQLite + `gorm.io` |

## 开源协议

MIT
