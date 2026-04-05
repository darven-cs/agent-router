---
phase: 04-foundation-restructure
plan: 03
subsystem: infra
tags: [go, internal-packages, tui-split, primary-upstream, cmd-layout, load-balancer, bubbletea]

# Dependency graph
requires:
  - phase: 04-02
    provides: "App struct in root main.go, internal/proxy, internal/admin packages"
provides:
  - "internal/tui package: 5 files (app.go, update.go, view.go, form.go, styles.go)"
  - "cmd/agent-router/main.go: final entry point with App struct and primary upstream callbacks"
  - "Primary upstream selection: [m] key -> TUI Callbacks -> App.lb.SetPrimary -> proxyWithRetry.GetPrimary"
  - "Fallback display: [Fallback] prefix in TUI log for RetryAttempt > 0"
affects: [05-event-driven, 06-request-pipeline]

# Tech tracking
tech-stack:
  added: []
  patterns: ["TUI split by responsibility (5 files)", "primary upstream pinning with auto-fallback", "Callbacks struct pattern for TUI-business logic decoupling"]

key-files:
  created:
    - internal/tui/styles.go
    - internal/tui/app.go
    - internal/tui/update.go
    - internal/tui/view.go
    - internal/tui/form.go
    - cmd/agent-router/main.go
  modified:
    - internal/proxy/proxy.go
    - internal/tui/app.go
    - internal/tui/update.go
    - internal/tui/view.go
  deleted:
    - config.go
    - upstream.go
    - usage.go
    - main.go
    - tui.go

key-decisions:
  - "DefaultModel exported (capitalized) so cmd/agent-router/main.go can set it after NewModel construction"
  - "modelSelectIndex independent from selectedIndex for primary upstream selection (0=Auto, 1..N=upstreams)"
  - "proxyWithRetry checks GetPrimary() first before retry loop; falls back to SelectNext if primary unavailable"
  - "renderNavigation shows [Primary: {name}] in styleRed when upstream is pinned"
  - "renderStatus shows active upstream in mauve (pinned) or subtextColor (auto)"

patterns-established:
  - "TUI Callbacks struct: 9 function fields replacing individual On* closures"
  - "Primary upstream flow: TUI [m] -> Callbacks.OnPrimarySelected -> App.lb.SetPrimary -> proxy reads GetPrimary()"
  - "Standard Go project layout: cmd/agent-router/main.go + internal/{config,upstream,storage,proxy,admin,tui}/"

requirements-completed: [ARCH-01, ARCH-02, TUI-01]

# Metrics
duration: 10min
completed: 2026-04-05
---

# Phase 4 Plan 3: TUI Split + Primary Upstream + Final Layout Summary

**TUI 代码拆分为 5 个文件，model-select 重设计为 primary upstream 选择，标准 Go 项目布局 (cmd/ + internal/)，所有根目录 .go 文件删除**

## Performance

- **Duration:** 10 min
- **Started:** 2026-04-05T06:00:00Z
- **Completed:** 2026-04-05T06:10:00Z
- **Tasks:** 2 (split + redesign/move)
- **Files modified:** 9 (5 new, 4 modified, 5 deleted)

## Accomplishments

**Task 1: TUI 机械拆分**
- tui.go (837 lines) 拆分为 5 个文件: styles.go, app.go, update.go, view.go, form.go
- 所有逻辑与原 tui.go 完全一致 (仅做包路径和类型引用更新)
- Callbacks struct 已创建，含 OnPrimarySelected/OnPrimaryCleared 存根
- Model struct 导出，含 modelSelectIndex 和 primaryUpstream 字段

**Task 2: Primary Upstream 重设计 + Cmd 布局**
- handleModelSelect 重设计: up/down/enter/esc 导航，index 0=Auto (hash)，1..N=upstreams
- renderModelSelect 重设计: "Select Primary Upstream" 标题，Auto 选项在上游列表之前，当前 primary 带 * 标记
- renderStatus 重设计: mauve 色显示 "Active: {name}" (pinned)，subtextColor 显示 "Active: Auto (hash)" (auto)，RetryAttempt > 0 显示红色 [Fallback] 前缀
- renderNavigation 更新: pinned 时显示红色 [Primary: {name}]
- proxyWithRetry 更新: 先检查 GetPrimary()，如果设置且有效则优先使用，否则正常 SelectNext
- cmd/agent-router/main.go 创建: App struct，tui.Callbacks 全部 9 个回调已连接，含 OnPrimarySelected 和 OnPrimaryCleared
- 根目录所有 .go 文件删除 (config.go, upstream.go, usage.go, main.go, tui.go)

## Task Commits

Each task was committed atomically:

1. **Task 1: TUI split into 5 files in internal/tui/** - `2ffb445` (feat)
2. **Task 2: Primary upstream redesign + cmd/ layout** - `2666b86` (feat)
3. **Human verification checkpoint** - auto-approved (auto_advance=true)

## Files Created/Modified

**TUI Split (internal/tui/):**
- `styles.go` - Catppuccin Mocha 调色板 (14 颜色变量, 16 样式变量)
- `app.go` - Model struct (导出), NewModel, Init, Callbacks struct, message types
- `update.go` - Update dispatcher, handleConfirm, handleModelSelect (primary upstream 逻辑), handleFormInput, submitForm, form helpers
- `view.go` - View, renderNavigation, renderContent, renderForm, renderConfirmation, renderUpstreamList, renderModelSelect (重设计), renderStatus (重设计), stripAnsi
- `form.go` - 占位符文件

**Proxy Update (internal/proxy/proxy.go):**
- `proxyWithRetry` 增加 GetPrimary() 检查: 优先使用 primary 上游，失败后自动切换

**Cmd Layout (cmd/agent-router/main.go):**
- App struct 完全重写，所有 9 个 Callbacks 已连接
- OnPrimarySelected: `app.lb.SetPrimary(u)`
- OnPrimaryCleared: `app.lb.ClearPrimary()`

**Root Files Deleted:**
- config.go, upstream.go, usage.go, main.go, tui.go (全部 thin wrapper 或已迁移到 internal/ 或 cmd/)

## Decisions Made

- **DefaultModel 大写导出**: 内部类型 `defaultModel` 改为 `DefaultModel`，允许 cmd/agent-router/main.go 在 NewModel 后设置初始值
- **modelSelectIndex 独立索引**: 区别于 selectedIndex，专用于 primary upstream 选择界面的导航 (0=Auto, 1..N=upstreams)
- **proxyWithRetry 优先主上游**: GetPrimary() 返回非 nil 且在 enabled 列表中则优先使用，否则正常 SelectNext 轮询
- **TUI Callbacks struct**: 9 个函数字段替代原有的 6+ 个独立闭包字段

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing] DefaultModel 未导出导致 cmd/agent-router/main.go 无法设置初始值**
- **Found during:** Task 2 编译阶段
- **Issue:** cmd/agent-router/main.go 需要 `tuiModel.defaultModel = app.cfg.Service.Model`，但 defaultModel 是小写未导出字段
- **Fix:** 将 `defaultModel` 改为 `DefaultModel` (大写导出)，更新 internal/tui/view.go 中所有引用
- **Files modified:** internal/tui/app.go, internal/tui/view.go, cmd/agent-router/main.go
- **Verification:** go build ./cmd/agent-router 通过
- **Committed in:** 2666b86

**2. [Rule 1 - Bug] form.go 包含未使用的 import 导致编译警告**
- **Found during:** Task 1 编译阶段
- **Issue:** form.go 占位符文件导入了 time, upstream, tea 但未使用
- **Fix:** 移除 form.go 中的未使用 import，只保留包声明和注释
- **Files modified:** internal/tui/form.go
- **Verification:** go build ./internal/tui 通过
- **Committed in:** 2ffb445

---

**Total deviations:** 2 auto-fixed (1 missing feature, 1 compilation warning)
**Impact on plan:** 均为迁移必需的类型导出修正，无功能变更

## Known Stubs

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 4 完全完成: cmd/ + internal/ 标准布局，App struct 管理所有依赖，primary upstream 选择功能就绪
- Phase 5 (Event-Driven Decoupling) 可开始: 事件总线替换 TUI callbacks，goroutine 泄漏防护
- Phase 6 (Request Pipeline) 可开始: 中间件链组合，admin API 共享认证

---
*Phase: 04-foundation-restructure*
*Completed: 2026-04-05*

## Self-Check: PASSED

- FOUND: internal/tui/app.go
- FOUND: internal/tui/update.go
- FOUND: internal/tui/view.go
- FOUND: internal/tui/form.go
- FOUND: internal/tui/styles.go
- FOUND: cmd/agent-router/main.go
- FOUND: internal/proxy/proxy.go
- FOUND: .planning/phases/04-foundation-restructure/04-03-SUMMARY.md
- FOUND: 2ffb445 (Task 1)
- FOUND: 2666b86 (Task 2)
- FOUND: go build ./cmd/agent-router passes
- FOUND: go vet ./... passes
- FOUND: No root .go files (config.go, upstream.go, usage.go, main.go, tui.go all deleted)
- FOUND: GetPrimary in internal/proxy/proxy.go
- FOUND: OnPrimarySelected/OnPrimaryCleared in cmd/agent-router/main.go
