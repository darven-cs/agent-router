# Quick Task 260404-q4m: TUI 新快捷键导致模型切换问题修复 - Context

**Gathered:** 2026-04-04
**Status:** Ready for planning

<domain>
## Task Boundary

修复 TUI 新快捷键实现中的模型切换问题。当前问题：切换下游的模型名字，导致每次都需要重新选择模型，很麻烦。

</domain>

<decisions>
## Implementation Decisions

### 模型选择作用域
- **全局 + 单上游双重设置**
- 全局默认模型：统一入口模型名（如 `claude-sonnet-4-20250514`），用于显示和连接
- 各上游独立模型：每个上游配置自己的模型名，作为该上游专属模型避免连接失败

### 已选模型持久化
- **自动恢复**
- 每个上游记住上次选择的模型
- 下次启用该上游时自动恢复，无需手动重新选择
- 持久化到 config.yaml

### 多上游负载均衡模型行为
- **各上游用自己的专属模型**
- 负载均衡时每个上游用自己的模型名请求
- 全局默认模型仅用于单上游场景或 TUI 导航栏显示

### Claude's Discretion
- TUI 交互细节（空格/m 键行为）按原始描述实现
- 0-9 快速选择编号由 TUI 列表顺序决定

</decisions>

<specifics>
## Specific Ideas

当前问题分析：
- 切换下游模型名字后，每次都需要重新选择
- 根因：模型选择可能没有按上游独立保存，或保存后没有恢复

修复方向：
- 每个上游独立记录自己的默认模型
- 切换上游时自动切换到该上游上次选择的模型
- 负载均衡时各上游使用自己的模型配置

</specifics>

<canonical_refs>
## Canonical References

- 原始需求：空格键启用/禁用上游，m 键进入模型选择模式
- 导航栏显示当前默认模型

</canonical_refs>
