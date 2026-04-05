---
phase: 04-foundation-restructure
plan: 02
subsystem: infra
tags: [go, internal-packages, app-struct, constructor-injection, dependency-injection]

# Dependency graph
requires:
  - phase: 04-01
    provides: "internal/config, internal/upstream, internal/storage packages with type alias wrappers"
provides:
  - "internal/proxy package: ProxyHandler, RequestLog, SetLoadBalancer, SetDefaultModel"
  - "internal/admin package: AdminHandler with constructor injection (cfg, db, stats, sharedUpstreams, startTime, reloadFn)"
  - "App struct in root main.go replacing all 7 global variables"
  - "StartUsageWorker migrated to internal/storage accepting proxy.RequestLog"
  - "Path-based HTTP mux routing /admin/* to AdminHandler, rest to ProxyHandler"
affects: [04-03, 05-event-driven, 06-request-pipeline]

# Tech tracking
tech-stack:
  added: []
  patterns: ["App struct lifecycle pattern (NewApp/Run/Shutdown)", "constructor injection for internal handlers", "setter methods for hot-reloadable fields (SetLoadBalancer, SetDefaultModel)"]

key-files:
  created:
    - internal/proxy/proxy.go
    - internal/admin/admin.go
  modified:
    - internal/storage/usage.go
    - internal/proxy/proxy.go
    - main.go
    - usage.go

key-decisions:
  - "ProxyHandler.lb is *upstream.LoadBalancer (pointer) so doReload can swap the instance via SetLoadBalancer setter"
  - "ProxyHandler gains SetLoadBalancer and SetDefaultModel setter methods for App.doReload to update after config reload"
  - "AdminHandler receives reloadFn as func() error callback instead of directly calling doReload -- decouples admin from App lifecycle"
  - "NewLoadBalancer returns *LoadBalancer (already pointer from Plan 01), no need for extra address-of"

patterns-established:
  - "App struct pattern: NewApp() creates dependencies, Run() starts server+TUI, Shutdown() cleans up, doReload() hot-swaps config"
  - "Constructor injection: internal packages receive all dependencies via NewXxxHandler constructors"
  - "Setter pattern for hot-reloadable fields: SetLoadBalancer/SetDefaultModel allow App.doReload to update ProxyHandler without recreating it"

requirements-completed: [ARCH-01, ARCH-02]

# Metrics
duration: 7min
completed: 2026-04-05
---

# Phase 4 Plan 2: Core Package Migration + App Struct Summary

**proxy.go 和 admin.go 迁移到 internal 包，App struct 替代全部 7 个全局变量，构造器注入实现依赖解耦**

## Performance

- **Duration:** 7 min
- **Started:** 2026-04-05T05:48:52Z
- **Completed:** 2026-04-05T05:56:34Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- proxy.go 迁移到 internal/proxy/proxy.go，移除 admin 路由，仅处理 POST /v1/messages
- admin.go 迁移到 internal/admin/admin.go，AdminHandler 通过构造器接收全部依赖
- main.go 重写为 App struct (NewApp/Run/Shutdown/doReload/persistConfig)，消除全部 7 个全局变量
- StartUsageWorker 迁移到 internal/storage/usage.go，接受 proxy.RequestLog 类型
- doReload 通过 setter 方法同步更新 ProxyHandler 的 lb 和 defaultModel

## Task Commits

Each task was committed atomically:

1. **Task 1: Migrate proxy.go to internal/proxy/proxy.go** - `3d6ce40` (feat)
2. **Task 2: Migrate admin.go to internal/admin, create App struct in main.go** - `767d0c9` (feat)

## Files Created/Modified
- `internal/proxy/proxy.go` - ProxyHandler, RequestLog, transformModelName, proxyWithRetry, isRetryable, SetLoadBalancer, SetDefaultModel
- `internal/admin/admin.go` - AdminHandler with ServeHTTP/HandleStatus/HandleReload, AdminStatus/UpstreamStats response types
- `internal/storage/usage.go` - 新增 StartUsageWorker(db *gorm.DB, usageChan <-chan proxy.RequestLog)
- `main.go` - App struct 完全重写 (NewApp, Run, Shutdown, doReload, persistConfig)
- `usage.go` - 简化为 type alias wrapper (UsageLog, UsageStats, RequestLog, Stats, initDB, StartUsageWorker)

## Decisions Made
- **ProxyHandler setter 方法**: 新增 SetLoadBalancer 和 SetDefaultModel，让 App.doReload 可以在不重建 ProxyHandler 的情况下更新其内部状态。这比重建 ProxyHandler 更简洁，因为 ProxyHandler 持有 logChan/usageChan 引用不应在 reload 时改变
- **AdminHandler reloadFn 回调**: AdminHandler 不直接依赖 App，而是通过 func() error 回调触发 reload，解耦 admin 包和 main 包
- **NewLoadBalancer 已返回指针**: Plan 01 已将 NewLoadBalancer 改为返回 *LoadBalancer，无需再取地址

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] ProxyHandler 缺少 setter 方法导致 main.go 编译失败**
- **Found during:** Task 2 编译阶段
- **Issue:** main.go 的 doReload() 和 TUI 回调需要更新 proxyHandler.lb 和 proxyHandler.defaultModel，但这些是未导出字段，无法从 main 包直接修改
- **Fix:** 在 internal/proxy/proxy.go 添加 SetLoadBalancer(*upstream.LoadBalancer) 和 SetDefaultModel(string) setter 方法
- **Files modified:** internal/proxy/proxy.go
- **Verification:** go build . 通过
- **Committed in:** 767d0c9

**2. [Rule 1 - Bug] main.go 指针类型错误 -- NewLoadBalancer 已返回 *LoadBalancer**
- **Found during:** Task 2 编译阶段
- **Issue:** 代码写 `lb := NewLoadBalancer(...); app.lb = &lb` 导致双重指针 (**)，因为 NewLoadBalancer 已返回 *LoadBalancer
- **Fix:** 改为 `app.lb = upstream.NewLoadBalancer(app.cfg.Upstreams)` 直接赋值
- **Files modified:** main.go
- **Verification:** go build . 通过
- **Committed in:** 767d0c9

**3. [Rule 3 - Blocking] main.go 缺少 gorm.io/gorm import**
- **Found during:** Task 2 编译阶段
- **Issue:** App struct 的 db 字段类型 *gorm.DB 需要 import gorm.io/gorm
- **Fix:** 在 import 块添加 "gorm.io/gorm"
- **Files modified:** main.go
- **Verification:** go build . 通过
- **Committed in:** 767d0c9

---

**Total deviations:** 3 auto-fixed (2 bug, 1 blocking)
**Impact on plan:** 所有修复为迁移必需的编译/类型问题，无功能变更，无范围膨胀

## Issues Encountered
None beyond the auto-fixed deviations above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- internal/proxy, internal/admin 两个核心包已就绪
- App struct 完整管理生命周期，全局变量全部消除
- Plan 03 可迁移 tui.go 到 internal/tui 包，根目录将只剩 main.go
- doReload 的 setter 模式为后续事件驱动架构奠定基础

---
*Phase: 04-foundation-restructure*
*Completed: 2026-04-05*

## Self-Check: PASSED

- FOUND: internal/proxy/proxy.go
- FOUND: internal/admin/admin.go
- FOUND: internal/storage/usage.go
- FOUND: main.go
- FOUND: .planning/phases/04-foundation-restructure/04-02-SUMMARY.md
- FOUND: 3d6ce40 (Task 1)
- FOUND: 767d0c9 (Task 2)
