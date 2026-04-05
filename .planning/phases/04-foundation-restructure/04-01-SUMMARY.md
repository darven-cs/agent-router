---
phase: 04-foundation-restructure
plan: 01
subsystem: infra
tags: [go, internal-packages, package-migration, load-balancer, type-alias]

# Dependency graph
requires:
  - phase: v1.0
    provides: "Flat package main codebase with config.go, upstream.go, usage.go"
provides:
  - "internal/config package: Config, ServiceConfig, UpstreamConfig, LoadConfig, SaveConfig"
  - "internal/upstream package: Upstream, SharedUpstreams, LoadBalancer with primary upstream support"
  - "internal/storage package: UsageLog, UsageStats, Stats, InitDB"
  - "Primary upstream methods: SetPrimary, ClearPrimary, GetPrimary, SelectForRequest"
  - "SharedUpstreams.ReplaceAll for atomic replacement"
affects: [04-02, 04-03, 05-event-driven, 06-request-pipeline]

# Tech tracking
tech-stack:
  added: []
  patterns: ["type-alias-wrapper pattern for incremental migration", "internal/ package layout"]

key-files:
  created:
    - internal/config/config.go
    - internal/upstream/upstream.go
    - internal/storage/usage.go
  modified:
    - config.go
    - upstream.go
    - usage.go
    - main.go
    - proxy.go
    - admin.go

key-decisions:
  - "LoadBalancer changed from value type to pointer type (*LoadBalancer) to accommodate sync.RWMutex for primary field -- go vet requires mutexes are never copied"
  - "UsageStats gained Record and GetCounts methods to encapsulate private field access from main package"
  - "StartUsageWorker stays in root usage.go pending proxy.go migration in Plan 02 (depends on RequestLog type)"

patterns-established:
  - "Type-alias wrapper pattern: root .go files re-export internal types for backward compatibility during incremental migration"
  - "Internal package dependency order: config -> upstream -> storage (no cycles)"

requirements-completed: [ARCH-01, ARCH-02, TUI-01]

# Metrics
duration: 7min
completed: 2026-04-05
---

# Phase 4 Plan 1: Leaf Package Migration Summary

**三个内部包 (config/upstream/storage) 迁移到 internal/ 目录，LoadBalancer 扩展了 primary upstream 方法，使用 type alias wrapper 保持向后兼容**

## Performance

- **Duration:** 7 min
- **Started:** 2026-04-05T05:35:00Z
- **Completed:** 2026-04-05T05:42:36Z
- **Tasks:** 3
- **Files modified:** 9

## Accomplishments
- 迁移 config.go 到 internal/config/config.go，所有类型和函数完整保留
- 迁移 upstream.go 到 internal/upstream/upstream.go，新增 SetPrimary/ClearPrimary/GetPrimary/SelectForRequest 方法和 ReplaceAll 方法
- 迁移 usage.go 到 internal/storage/usage.go，UsageStats 新增 Record/GetCounts 封装方法
- 所有根目录文件变为 thin type alias wrapper，保持增量迁移期间编译兼容

## Task Commits

Each task was committed atomically:

1. **Task 1: Migrate config.go to internal/config/config.go** - `c8e244b` (feat)
2. **Task 2: Migrate upstream.go to internal/upstream with primary upstream extension** - `fea6f43` (feat)
3. **Task 3: Migrate usage.go to internal/storage/usage.go** - `1ca3b77` (feat)
4. **Fix: Change LoadBalancer to pointer type for mutex safety** - `cdb3323` (fix)

**Plan metadata:** pending (docs: complete plan)

_Note: Additional fix commit for go vet compliance_

## Files Created/Modified
- `internal/config/config.go` - Config 类型 (ServiceConfig, UpstreamConfig, Config) 和 LoadConfig/SaveConfig 函数
- `internal/upstream/upstream.go` - Upstream, SharedUpstreams, LoadBalancer (含 primary upstream 方法和 ReplaceAll)
- `internal/storage/usage.go` - UsageLog GORM model, UsageStats with Record/GetCounts, InitDB
- `config.go` - Thin re-export wrapper (type alias to internal/config)
- `upstream.go` - Thin re-export wrapper (type alias to internal/upstream)
- `usage.go` - Thin re-export wrapper + StartUsageWorker (保留至 Plan 02)
- `main.go` - lb 变量改为 *LoadBalancer，doReload 使用 ReplaceAll
- `proxy.go` - ProxyHandler.lb 改为 *LoadBalancer，NewProxyHandler 参数改为指针
- `admin.go` - 使用 Stats.GetCounts() 替代直接字段访问

## Decisions Made
- **LoadBalancer 改为指针类型**: 添加 sync.RWMutex 后，go vet 禁止值传递包含 mutex 的结构体。将 NewLoadBalancer 返回 *LoadBalancer，所有方法改为指针接收器
- **UsageStats 封装方法**: 添加 Record() 和 GetCounts() 方法，使外部包无需直接访问私有字段
- **StartUsageWorker 延迟迁移**: 它依赖 RequestLog 类型（定义在 proxy.go），等 Plan 02 迁移 proxy.go 时一起处理

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] LoadBalancer 值类型改为指针类型**
- **Found during:** Task 2 验证阶段
- **Issue:** LoadBalancer 新增 sync.RWMutex 字段后，go vet 报错 "passes lock by value"，Select/GetEnabled/SelectNext 使用值接收器会复制 mutex
- **Fix:** 将 NewLoadBalancer 返回 *LoadBalancer，所有方法改为指针接收器，更新 main.go 和 proxy.go 中的变量和参数类型
- **Files modified:** internal/upstream/upstream.go, main.go, proxy.go
- **Verification:** go vet ./... 零警告
- **Committed in:** cdb3323

**2. [Rule 3 - Blocking] main.go doReload 直接访问 SharedUpstreams 私有字段**
- **Found during:** Task 2 编译阶段
- **Issue:** 迁移 SharedUpstreams 到 internal/upstream 后，mu 和 upstreams 字段对 main 包不可见
- **Fix:** 使用新添加的 ReplaceAll 方法替代直接的 mu.Lock/upstreams 赋值
- **Files modified:** main.go
- **Verification:** go build ./... 通过
- **Committed in:** fea6f43

**3. [Rule 3 - Blocking] admin.go 和 usage.go 直接访问 stats 私有字段**
- **Found during:** Task 3 编译阶段
- **Issue:** UsageStats 迁移到 internal/storage 后，mu/totalRequests 等字段对 main 包不可见
- **Fix:** 在 storage 包添加 Record() 和 GetCounts() 公开方法，admin.go 使用 GetCounts()，usage.go 使用 Record()
- **Files modified:** internal/storage/usage.go, admin.go, usage.go
- **Verification:** go build ./... 通过
- **Committed in:** 1ca3b77

---

**Total deviations:** 3 auto-fixed (1 bug, 2 blocking)
**Impact on plan:** 所有修复为迁移必需的类型安全变更，无功能变更，无范围膨胀

## Issues Encountered
- go vet 对包含 sync.RWMutex 的结构体值传递检测 -- 已通过改为指针类型解决

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- internal/config, internal/upstream, internal/storage 三个基础包已就绪
- Plan 02 可迁移 proxy.go 和 admin.go，消除根目录 wrapper 文件
- LoadBalancer 的 primary upstream 方法已为 Plan 03 的 TUI 模型选择功能做好准备
- StartUsageWorker 待 Plan 02 中随 proxy.go 一起迁移

---
*Phase: 04-foundation-restructure*
*Completed: 2026-04-05*

## Self-Check: PASSED

- FOUND: internal/config/config.go
- FOUND: internal/upstream/upstream.go
- FOUND: internal/storage/usage.go
- FOUND: .planning/phases/04-foundation-restructure/04-01-SUMMARY.md
- FOUND: c8e244b (Task 1)
- FOUND: fea6f43 (Task 2)
- FOUND: 1ca3b77 (Task 3)
- FOUND: cdb3323 (Fix)
