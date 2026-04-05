# Phase 4: Foundation Restructure - Research

**Researched:** 2026-04-05
**Domain:** Go 项目重构 — 目录布局、依赖注入、TUI model-select 功能修复
**Confidence:** HIGH

## Summary

Phase 4 将 7 文件 Go 单体应用（1890 LOC）重构为标准 Go 项目布局（`cmd/` + `internal/`），同时消除全部 7+ 全局变量并用 App struct 替代，修复 TUI [m] model-select 功能将其重新设计为 "Primary Upstream" 选择模式。

当前代码库是一个平坦的 `package main` 结构，7 个全局变量在 `main.go` 中声明但被 `admin.go`、`proxy.go`、`tui.go`、`usage.go` 直接引用。重构的核心挑战是：(1) 避免循环导入（Go 编译器强制禁止），(2) 确保 `RequestLog` 等共享类型的归属正确，(3) 保持每一步都能编译通过。

Primary Upstream 功能基于现有的 LoadBalancer.SelectNext 重试机制扩展——当用户 pin 某个上游时，所有请求首先路由到该上游，失败时利用现有的指数退避重试自动切换到其他上游。

**Primary recommendation:** 自底向上逐文件迁移，先创建无依赖的叶子包（config、upstream），再创建依赖前者的中间包（storage、proxy），最后连接顶层 App struct。共享类型 `RequestLog` 放在独立的 `internal/types` 包中避免循环导入。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Target structure: `cmd/agent-router/main.go` + `internal/{config,proxy,tui,upstream,storage,admin}/`
- **D-02:** Split tui.go (837 lines) by responsibility into 5 files: `app.go` (Model struct, NewModel), `update.go` (Update + handlers), `view.go` (View + render methods), `form.go` (form state, validation), `styles.go` (lipgloss colors, style vars)
- **D-03:** admin.go moves to `internal/admin/admin.go`
- **D-04:** usage.go moves to `internal/storage/usage.go`
- **D-05:** Remaining files map 1:1 to packages: config.go -> internal/config/, proxy.go -> internal/proxy/, upstream.go -> internal/upstream/
- **D-06:** `[m]` key redesign: shows upstream list + "Auto (hash)" option at top. Selecting an upstream pins it as Primary. Selecting "Auto" returns to hash-based distribution.
- **D-07:** Default behavior: FNV hash load balancing (unchanged from v1.0). Pinning is optional.
- **D-08:** When primary upstream is set: all requests route to primary first. On failure (5xx/timeout/429), auto-fallback to other enabled upstreams using existing exponential backoff retry (1s/2s/4s, max 3 retries).
- **D-09:** Model name transformation: silently replace outgoing model with the upstream's configured model. Claude Code always sees standard model names, upstream receives its configured model.
- **D-10:** TUI status bar shows "Active Upstream: {name}" when pinned, "Auto (hash)" when in distribution mode
- **D-11:** Fallback events appear in TUI log: "[Fallback] {name} failed, trying {next}..."
- **D-12:** App struct holds all top-level dependencies (cfg, db, lb, proxy, storage). Each package receives dependencies through constructor functions.
- **D-13:** App manages full lifecycle: `NewApp(cfg) *App`, `Run() error`, `Shutdown()`. Signal handling (SIGINT/SIGTERM/SIGHUP) inside App.
- **D-14:** TUI callbacks become App methods. TUI model receives a `Callbacks` struct with function fields, wired by App. This eliminates the 6+ closures in main.go that reference globals.
- **D-15:** No interfaces for now -- 1890 LOC tool doesn't need the abstraction. Direct struct dependencies are sufficient.
- **D-16:** All 7 mutable globals eliminated: `db`, `usageChan`, `execPath`, `sharedUpstreams`, `lb`, `proxyHandler`, `cfg` (+ `startTime`, `stats`) -- all become App struct fields.
- **D-17:** Bottom-up, one file at a time. Order: config -> upstream -> storage -> admin -> proxy -> tui -> main.go -> cmd/
- **D-18:** Each step must compile: `go build ./...` and `go vet ./...` pass after every file move
- **D-19:** Verification: build + vet each step, smoke test at end (start server, send request, check TUI displays correctly)

### Claude's Discretion
- Exact App struct field names and constructor signatures
- Primary upstream state storage (in LoadBalancer or separate field)
- TUI "Auto (hash)" option styling in model-select view
- Fallback log message formatting
- go.mod module path naming

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| ARCH-01 | Developer can find code organized by domain in cmd/ and internal/ directories (Standard Go Project Layout) | 本文档 Standard Stack + Architecture Patterns 部分提供目标目录结构和迁移策略 |
| ARCH-02 | App struct replaces all 7 global variables, each package receives dependencies through constructors | 本文档 App Struct Design 部分详细说明全局变量 -> App struct 字段映射，以及构造函数签名 |
| TUI-01 | User can select upstream model via [m] key and proxy immediately uses that model for routing | 本文档 Primary Upstream Feature 部分详细说明 [m] 键重新设计和路由逻辑变更 |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go | 1.25.3 (已安装) | 编译器和运行时 | go.mod 指定 go 1.24.0，实际运行 1.25.3 |
| bubbletea | v1.3.10 | TUI 框架 | go.mod 已锁定版本 |
| lipgloss | v1.1.0 | TUI 样式 | go.mod 已锁定版本 |
| gorm | v1.31.1 | SQLite ORM | go.mod 已锁定版本 |
| gorm.io/driver/sqlite | v1.6.0 | SQLite 驱动 | go.mod 已锁定版本 |
| gopkg.in/yaml.v3 | v3.0.1 | YAML 配置解析 | go.mod 已锁定版本 |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| mattn/go-sqlite3 | v1.14.40 | SQLite CGO 驱动 | gorm sqlite driver 的间接依赖 |

**Installation:**
无需安装新依赖。本阶段不引入任何新第三方库。

**Version verification:** 所有版本已从 go.mod 和 go.sum 验证。本阶段仅做代码重组和功能添加，不改依赖。

## Architecture Patterns

### Recommended Project Structure

```
agent-router/
├── cmd/
│   └── agent-router/
│       └── main.go              # 入口点：解析配置、创建 App、调用 App.Run()
├── internal/
│   ├── config/
│   │   └── config.go            # Config, ServiceConfig, UpstreamConfig, LoadConfig, SaveConfig
│   ├── upstream/
│   │   └── upstream.go          # Upstream, SharedUpstreams, LoadBalancer
│   ├── storage/
│   │   └── usage.go             # UsageLog, UsageStats, InitDB, StartUsageWorker
│   ├── proxy/
│   │   └── proxy.go             # ProxyHandler, transformModelName, isRetryable
│   ├── admin/
│   │   └── admin.go             # handleAdminStatus, handleAdminReload, writeAdminError
│   └── tui/
│       ├── app.go               # Model struct, NewModel, Init, Callbacks struct
│       ├── update.go            # Update method, handleConfirm, handleModelSelect, handleFormInput
│       ├── view.go              # View method, renderNavigation, renderContent, renderStatus
│       ├── form.go              # renderForm, submitForm, handleFormTextInput, handleFormBackspace
│       └── styles.go            # 所有 lipgloss 颜色变量和样式定义
├── go.mod
├── go.sum
└── config.yaml
```

**关键设计决策：共享类型归属**

`RequestLog` 类型当前定义在 `proxy.go` 中，但被以下文件引用：
- `main.go` (第 19, 64, 67 行) — usageChan 类型
- `tui.go` (第 120, 162, 242 行) — logs 字段类型, Update 消息类型
- `usage.go` (第 53 行) — StartUsageWorker 参数类型
- `proxy.go` (第 47, 48, 52, 65, 303, 317 行) — 定义和使用

由于 Go 不允许循环导入，如果 `RequestLog` 放在 `internal/proxy` 包中，那么 `internal/storage` 和 `internal/tui` 都需要导入 `internal/proxy`。这本身不会造成循环（storage 和 tui 不被 proxy 导入），所以 `RequestLog` 放在 `internal/proxy` 是可行的。

但更清晰的方案是将 `RequestLog` 放在一个独立的 `internal/types` 包中，这样所有包都可以导入它而不引入不必要的依赖。不过，考虑到项目规模仅 1890 LOC 且 D-15 明确 "不需要抽象"，保持简单——将 `RequestLog` 放在 `internal/proxy` 中是正确的选择，因为：
1. 它是 proxy 请求流程的核心数据类型
2. storage 和 tui 导入 proxy 是单向的，不会循环
3. storage 包中 `UsageLog`（GORM 模型）是独立的，不需要知道 `RequestLog`

**推荐方案：`RequestLog` 保留在 `internal/proxy` 包中。** 其他需要它的包直接 `import "agent-router/internal/proxy"` 即可。

### Pattern 1: App Struct 构造函数注入
**What:** 用 App struct 聚合所有依赖，每个包通过构造函数接收依赖
**When to use:** 替代全局变量，这是 Go 中最惯用的依赖注入方式
**Example:**
```go
// internal/app/app.go (或在 cmd/agent-router/main.go 中定义)
type App struct {
    cfg             *config.Config
    db              *gorm.DB
    sharedUpstreams *upstream.SharedUpstreams
    lb              *upstream.LoadBalancer
    proxyHandler    *proxy.ProxyHandler
    storage         *storage.Storage
    usageChan       chan proxy.RequestLog
    startTime       time.Time
    stats           *storage.UsageStats
    execPath        string
    server          *http.Server

    // Primary upstream feature
    primaryUpstream *upstream.Upstream  // nil = auto hash mode
}

func NewApp(cfgPath string) (*App, error) {
    app := &App{startTime: time.Now()}
    // 1. Load config
    // 2. Init upstreams + LoadBalancer
    // 3. Init storage (SQLite)
    // 4. Create proxy handler
    // 5. Create TUI model with callbacks
    // 6. Setup HTTP server
    return app, nil
}

func (app *App) Run() error {
    // Start HTTP server, signal handling, run TUI
}

func (app *App) Shutdown() error {
    // Graceful shutdown: stop server, close channels
}
```

### Pattern 2: TUI Callbacks Struct
**What:** TUI Model 通过 Callbacks struct 接收 App 方法绑定的函数字段
**When to use:** 替代 main.go 中 6+ 个引用全局变量的闭包
**Example:**
```go
// internal/tui/app.go
type Callbacks struct {
    OnUpstreamAdded         func(u *upstream.Upstream)
    OnUpstreamUpdated       func(u *upstream.Upstream, oldName string)
    OnUpstreamDeleted       func(name string)
    OnUpstreamToggled       func(u *upstream.Upstream)
    OnDefaultModelChanged   func(model string)
    OnUpstreamModelSelected func(u *upstream.Upstream)
    OnReload                func() error
    OnPrimarySelected       func(u *upstream.Upstream)  // NEW: primary upstream selection
    OnPrimaryCleared        func()                       // NEW: return to auto mode
}
```

### Pattern 3: Primary Upstream 路由
**What:** 在 LoadBalancer 中添加 primary upstream 概念，修改 proxyWithRetry 的上游选择逻辑
**When to use:** 用户 pin 某个上游时的路由行为
**Example:**
```go
// 在 LoadBalancer 中添加
func (lb *LoadBalancer) SetPrimary(u *upstream.Upstream) {
    lb.primary = u  // nil = auto mode
}

// 在 proxyWithRetry 中修改选择逻辑
func (h *ProxyHandler) proxyWithRetry(w, r, requestID) {
    enabled := h.lb.GetEnabled()
    if len(enabled) == 0 { ... }

    var firstUpstream *Upstream
    if h.lb.primary != nil {
        firstUpstream = h.lb.primary  // 使用 pinned upstream
    } else {
        firstUpstream = h.lb.SelectNext(nil)  // 原有 hash 模式
    }
    // 后续重试逻辑不变，SelectNext 自然跳到其他 upstream
}
```

### Anti-Patterns to Avoid
- **循环导入:** internal/proxy 导入 internal/tui，同时 internal/tui 导入 internal/proxy -> 编译错误。解决方案：tui 通过 Callbacks struct（函数字段）与业务逻辑解耦，不直接导入 proxy 包的类型（除了 RequestLog 用于 Update 消息匹配）
- **过度拆包:** 创建太多 internal 子包导致导入路径冗长。当前 6 个包（config, upstream, storage, proxy, admin, tui）对于 1890 LOC 已经足够
- **先移 tui.go 再处理依赖:** tui.go 是最大的文件（837 行），且依赖最多类型（Upstream, RequestLog, Callbacks）。必须先完成 config/upstream/proxy 的迁移，最后再移 tui
- **一次移动多个文件:** 违反 D-18 的"每步必须编译通过"原则

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 依赖注入框架 | wire, dig, fx | 构造函数注入 + App struct | D-15 明确规定不用 DI 框架，7 个全局变量用 App struct 足够 |
| 路由框架 | gin, echo, chi | net/http + ServeHTTP switch | 当前 proxy.go 已有基于路径的 switch，无需引入路由库 |
| 配置管理 | viper | yaml.v3 + os.ExpandEnv | CLAUDE.md 明确禁止 viper，当前 config.go 已实现完整功能 |
| 日志框架 | zap, zerolog | fmt.Fprintf(os.Stderr) | 项目规模小，Phase 6 才引入 structured logging |

**Key insight:** 本阶段的核心是代码重组而非功能新增（Primary Upstream 除外），不应引入任何新的第三方依赖。

## Runtime State Inventory

> 本阶段涉及代码重构和功能增强，检查运行时状态。

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | SQLite usage.db — 表 UsageLog 的 UpstreamName 字段存储上游名称 | 无需变更 — upstream 名称不受重构影响 |
| Live service config | config.yaml 在运行时被 persistConfig() 写回，字段名不变 | 代码编辑 — persistConfig() 从 main.go 迁移到 App 方法 |
| OS-registered state | 无 | 无 |
| Secrets/env vars | config.yaml 中 api_key 字段使用环境变量扩展 `${VAR}` | 无需变更 — config.go 的 os.ExpandEnv 逻辑不变 |
| Build artifacts | agent-router 二进制文件（如果存在） | 重构后需重新 `go build ./cmd/agent-router` |

**Nothing found in category (OS-registered state):** 明确声明——项目不注册 OS 级别的任务或服务。

## Common Pitfalls

### Pitfall 1: 循环导入
**What goes wrong:** 移动代码到 internal/ 子包时，A 导入 B，B 也导入 A，导致编译失败
**Why it happens:** Go 编译器严格禁止循环导入。当前所有文件都在 package main 中，不存在此问题。拆包后依赖关系会变化。
**How to avoid:**
1. 严格遵守迁移顺序：config（无依赖）-> upstream（依赖 config 类型）-> storage（依赖 upstream 类型）-> proxy（依赖 upstream）-> tui（依赖 upstream, proxy.RequestLog）-> admin（依赖 config, upstream, storage）-> main.go（依赖所有包）
2. 共享类型向下流动：被多个包使用的类型（如 UpstreamConfig）放在更底层的包中
3. TUI 通过 Callbacks struct 避免反向导入业务逻辑包
**Warning signs:** `go build` 报错 `import cycle not allowed`

### Pitfall 2: RequestLog 类型归属
**What goes wrong:** RequestLog 定义在 proxy 包中，但 storage 和 tui 都需要引用它，可能导致 storage -> proxy 的依赖显得不自然
**Why it happens:** RequestLog 是 proxy 请求流程的产物，但被 storage 持久化和 tui 显示
**How to avoid:** 如果未来 storage 和 tui 都需要导入 proxy 包时感觉奇怪，可以将 RequestLog 提取到一个 `internal/types` 包。但目前单方向依赖（storage -> proxy）不会循环，保持简单即可
**Warning signs:** 某个包的 import 列表包含"不应该依赖"的包

### Pitfall 3: LoadBalancer 值类型 vs 指针类型
**What goes wrong:** 当前 `lb` 全局变量是 `LoadBalancer`（值类型），`doReload()` 中 `lb = newUpstreams` 替换整个值。迁移到 App struct 后，如果 proxy handler 持有 LoadBalancer 的值拷贝，替换 App.lb 不会影响已存在的 handler
**Why it happens:** Go 值语义导致 LoadBalancer 被拷贝。当前代码通过 `proxyHandler.lb = lb` 手动同步。
**How to avoid:** 将 LoadBalancer 改为指针类型 `*LoadBalancer`，或在 App struct 中用指针。ProxyHandler 和 AdminHandler 都持有 App 的指针（或通过方法访问）
**Warning signs:** 修改 LoadBalancer 后请求仍然路由到旧的上游

### Pitfall 4: tui.go 837 行拆分时遗漏消息处理
**What goes wrong:** 将 Update() 逻辑拆到 update.go 后，某些消息类型的处理分支遗漏
**Why it happens:** Update() 方法包含多层嵌套 switch（先判断模式，再处理按键），拆分时容易遗漏某个分支
**How to avoid:** 拆分前先列出所有 Update() 处理的消息类型和分支路径，拆分后逐一验证
**Warning signs:** 按键无响应、模式切换失败

### Pitfall 5: doReload() 中的 SharedUpstreams 直接字段访问
**What goes wrong:** doReload() 中 `sharedUpstreams.mu.Lock()` 和 `sharedUpstreams.upstreams = newList` 直接访问私有字段。迁移到 internal/upstream 包后，main 包无法访问这些字段
**Why it happens:** SharedUpstreams 的 mu 和 upstreams 字段是小写的（包内私有），在 package main 中可以访问，但迁移后跨包不可见
**How to avoid:** 迁移 upstream.go 时，将需要跨包访问的逻辑封装为 SharedUpstreams 的公开方法（如 `ReplaceAll(upstreams []*Upstream)`）
**Warning signs:** 编译错误 `upstreams.mu undefined` 或 `upstreams.upstreams undefined`

### Pitfall 6: Primary Upstream 线程安全
**What goes wrong:** 并发请求读取 primaryUpstream 字段时，另一个 goroutine 正在通过 [m] 键修改它
**Why it happens:** TUI 运行在 bubbletea 的主 goroutine 中，proxy handler 运行在 HTTP server goroutine 中
**How to avoid:** 将 primaryUpstream 放在 LoadBalancer 中（已有 mutex 保护），或使用 sync/atomic 存储
**Warning signs:** 偶发的 nil pointer dereference 或路由到错误的上游

## Code Examples

### App Struct — 全局变量到字段的映射

```go
// 当前全局变量（main.go:17-26）
var (
    db              *gorm.DB                    // -> app.db
    usageChan       chan RequestLog             // -> app.usageChan
    execPath        string                      // -> app.execPath
    sharedUpstreams *SharedUpstreams            // -> app.sharedUpstreams
    lb              LoadBalancer                // -> app.lb (改为指针 *LoadBalancer)
    proxyHandler    *ProxyHandler               // -> app.proxyHandler
    cfg             *Config                     // -> app.cfg
    startTime       = time.Now()                // -> app.startTime
)
// usage.go:33
var stats = &UsageStats{}                       // -> app.stats (或保留在 storage 包内)

// 目标 App struct（定义在 cmd/agent-router/main.go 或 internal/app/app.go）
type App struct {
    cfg             *config.Config
    db              *gorm.DB
    sharedUpstreams *upstream.SharedUpstreams
    lb              *upstream.LoadBalancer
    proxyHandler    *proxy.ProxyHandler
    usageChan       chan proxy.RequestLog
    logChan         chan proxy.RequestLog
    startTime       time.Time
    stats           *storage.UsageStats
    execPath        string
    server          *http.Server
}
```

### SharedUpstreams — 添加 ReplaceAll 方法

```go
// internal/upstream/upstream.go
// 解决 Pitfall 5: doReload() 需要替换全部 upstreams

// ReplaceAll atomically replaces all upstreams (for config reload)
func (s *SharedUpstreams) ReplaceAll(upstreams []*Upstream) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.upstreams = upstreams
}
```

### Primary Upstream — LoadBalancer 扩展

```go
// internal/upstream/upstream.go — 在 LoadBalancer 中添加

type LoadBalancer struct {
    upstreams []*Upstream
    primary   *Upstream      // nil = auto hash mode
    mu        sync.RWMutex   // 保护 primary 字段
}

// SetPrimary sets the primary upstream for pinned routing
func (lb *LoadBalancer) SetPrimary(u *Upstream) {
    lb.mu.Lock()
    defer lb.mu.Unlock()
    lb.primary = u
}

// ClearPrimary returns to auto hash distribution mode
func (lb *LoadBalancer) ClearPrimary() {
    lb.mu.Lock()
    defer lb.mu.Unlock()
    lb.primary = nil
}

// GetPrimary returns the current primary upstream (nil if auto mode)
func (lb *LoadBalancer) GetPrimary() *Upstream {
    lb.mu.RLock()
    defer lb.mu.RUnlock()
    return lb.primary
}

// SelectForRequest chooses upstream based on primary or hash
func (lb *LoadBalancer) SelectForRequest(hashInput string) *Upstream {
    lb.mu.RLock()
    primary := lb.primary
    lb.mu.RUnlock()

    if primary != nil {
        // 验证 primary 仍在 enabled 列表中
        for _, u := range lb.upstreams {
            if u == primary && u.Enabled {
                return primary
            }
        }
    }
    // Fallback to hash mode
    return lb.Select(hashInput)
}
```

### Primary Upstream — ProxyHandler 路由修改

```go
// internal/proxy/proxy.go — proxyWithRetry 修改

func (h *ProxyHandler) proxyWithRetry(w http.ResponseWriter, r *http.Request, requestID string) {
    enabled := h.lb.GetEnabled()
    if len(enabled) == 0 {
        h.writeError(w, http.StatusBadGateway, "upstream_error", "No upstream available", 1001)
        return
    }

    var lastUpstream *Upstream
    var lastErr error
    retryCount := 0
    delay := baseDelay

    // 第一次选择：优先使用 primary，否则用 hash
    primary := h.lb.GetPrimary()
    if primary != nil {
        // 验证 primary 在 enabled 列表中且已启用
        found := false
        for _, u := range enabled {
            if u == primary && u.Enabled {
                found = true
                break
            }
        }
        if found {
            lastUpstream = primary
        }
    }
    if lastUpstream == nil {
        lastUpstream = h.lb.SelectNext(nil)
    }

    for attempt := 0; attempt <= maxRetries; attempt++ {
        upstream := lastUpstream
        if attempt > 0 {
            upstream = h.lb.SelectNext(lastUpstream)
        }
        if upstream == nil {
            upstream = enabled[0]
        }

        retryable, statusCode := h.proxyRequest(w, r, upstream, requestID, attempt, retryCount)
        if retryable == nil {
            return // success
        }
        lastErr = retryable
        lastUpstream = upstream

        // Fallback 日志事件（D-11）
        if h.logChan != nil {
            h.logChan <- RequestLog{
                Timestamp:    time.Now(),
                UpstreamName: upstream.Name,
                StatusCode:   statusCode,
                RequestID:    requestID,
                // 标记为 fallback 事件... (通过 StatusCode=0 或新增字段区分)
            }
        }

        if !isRetryable(lastErr, statusCode) {
            break
        }

        if attempt < maxRetries {
            time.Sleep(delay)
            delay *= 2
            if delay > maxDelay {
                delay = maxDelay
            }
            retryCount++
        }
    }

    h.writeError(w, http.StatusBadGateway, "upstream_error", "All upstreams failed", 1001)
}
```

### TUI Model-Select 重新设计

```go
// internal/tui/update.go — handleModelSelect 重新设计

func (m model) handleModelSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
    if msg, ok := msg.(tea.KeyMsg); ok {
        switch msg.String() {
        case "up":
            if m.modelSelectIndex > 0 {
                m.modelSelectIndex--
            }
        case "down":
            // 0 = "Auto (hash)", 1..N = upstreams
            if m.modelSelectIndex < len(m.upstreams) {
                m.modelSelectIndex++
            }
        case "enter":
            if m.modelSelectIndex == 0 {
                // 选择了 "Auto (hash)"
                if m.Callbacks.OnPrimaryCleared != nil {
                    m.Callbacks.OnPrimaryCleared()
                }
            } else {
                // 选择了某个 upstream
                idx := m.modelSelectIndex - 1
                if idx < len(m.upstreams) {
                    us := m.upstreams[idx]
                    if m.Callbacks.OnPrimarySelected != nil {
                        m.Callbacks.OnPrimarySelected(us)
                    }
                }
            }
            m.modelSelectMode = false
        case "esc":
            m.modelSelectMode = false
        }
    }
    return m, nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| 全局变量共享状态 | App struct 构造函数注入 | Go 社区长期最佳实践 | 消除隐式依赖，提高可测试性 |
| 单 package main | cmd/ + internal/ 标准布局 | Go 1.4+ (internal), 社区约定 | 编译器强制 internal 封装 |
| tui.go 837 行单文件 | 按职责拆分 5 文件 | bubbletea 社区推荐 | 每个文件 < 300 行，职责单一 |
| model-select 选择上游模型 | primary upstream pin 模式 | 本阶段设计变更 | 路由行为更直观，保留 fallback 能力 |

**Deprecated/outdated:**
- gorilla/mux: 已停止维护，当前项目未使用
- viper: 过度复杂，CLAUDE.md 明确禁止

## Open Questions

1. **App struct 定义位置**
   - What we know: D-12 要求 App struct 管理完整生命周期
   - What's unclear: App 定义在 `cmd/agent-router/main.go` 还是独立的 `internal/app/app.go`
   - Recommendation: 定义在 `cmd/agent-router/main.go` 中。项目只有一个入口点，没有其他 binary 需要复用 App struct。保持简单。

2. **go.mod module path**
   - What we know: 当前 module path 是 `agent-router`（无域名前缀）
   - What's unclear: Claude's Discretion 中提到 module path naming
   - Recommendation: 保持 `agent-router` 不变。这是本地工具，不需要发布到公共仓库。修改 module path 会影响所有 import 语句，增加无谓的工作量。

3. **Primary Upstream 状态持久化**
   - What we know: 用户通过 [m] 键选择 primary upstream
   - What's unclear: primary 选择是否应该持久化到 config.yaml
   - Recommendation: 不持久化。primary upstream 是临时路由偏好，重启后回到 auto hash 模式。这符合"配置文件描述部署拓扑，运行时状态描述路由偏好"的分离原则。

4. **Fallback 日志消息格式**
   - What we know: D-11 要求 "[Fallback] {name} failed, trying {next}..."
   - What's unclear: 是作为 RequestLog 的新字段，还是作为独立的 TUI 消息类型
   - Recommendation: 通过 RequestLog 的特殊标记（如 RetryAttempt > 0）在 TUI 渲染时添加 fallback 文本。不需要新的消息类型。

5. **model select 模式中的 selectedIndex 复用**
   - What we know: 当前 modelSelectMode 复用 navigation 的 selectedIndex
   - What's unclear: 拆分后 model-select 是否需要独立的 index（因为多了 "Auto (hash)" 选项）
   - Recommendation: 添加独立的 `modelSelectIndex` 字段，初始值为 0（指向 "Auto (hash)"），不复用 navigation 的 selectedIndex。避免模式切换后 index 位置混乱。

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | 全部 | ✓ | 1.25.3 | -- |
| CGO (gcc) | SQLite driver | ✓ (系统自带) | -- | -- |
| golangci-lint | 代码质量检查 | -- (未检查) | -- | 使用 go vet |

**Missing dependencies with no fallback:**
无 -- 所有必需依赖已就绪。

**Missing dependencies with fallback:**
- golangci-lint 未检查是否安装。使用 `go vet` 作为替代进行静态分析。

## Migration Dependency Graph

理解包之间的依赖关系对于避免循环导入至关重要：

```
cmd/agent-router/main.go
    ├── internal/config
    ├── internal/upstream  (依赖 internal/config)
    ├── internal/storage   (依赖 internal/upstream)
    ├── internal/proxy     (依赖 internal/upstream)
    ├── internal/admin     (依赖 internal/config, internal/upstream, internal/storage)
    └── internal/tui       (依赖 internal/upstream, internal/proxy.RequestLog)
```

**安全的迁移顺序（D-17 的详细版本）：**

1. **internal/config** — 零外部依赖。直接移入。只需改 package 声明。
2. **internal/upstream** — 依赖 internal/config（类型 UpstreamConfig）。移入后所有 Upstream 相关类型和方法归位。
3. **internal/types**（可选） — 如果 RequestLog 需要独立。推荐放在 internal/proxy 中。
4. **internal/storage** — 依赖 internal/upstream（Upstream 类型）。移入 UsageLog, UsageStats, InitDB, StartUsageWorker。
5. **internal/proxy** — 依赖 internal/upstream（LoadBalancer, Upstream）。移入 ProxyHandler, RequestLog, transformModelName, isRetryable。
6. **internal/admin** — 依赖 internal/config, internal/upstream, internal/storage。需要接收 App 引用来替代全局变量访问。
7. **internal/tui** — 依赖 internal/upstream（Upstream 类型）, internal/proxy（RequestLog 类型）。拆分为 5 文件。通过 Callbacks struct 解耦。
8. **cmd/agent-router/main.go** — 入口点。创建 App struct，wire 所有依赖。最小的 main 函数。

**每步验证命令：**
```bash
go build ./... && go vet ./...
```

## Existing Code Analysis — 关键发现

### 全局变量引用统计

| 全局变量 | 定义位置 | 引用文件 | 引用次数 |
|----------|----------|----------|----------|
| `cfg` | main.go:24 | main.go, admin.go | 12 |
| `db` | main.go:18 | main.go, admin.go | 3 |
| `sharedUpstreams` | main.go:21 | main.go, admin.go | 7 |
| `lb` | main.go:22 | main.go | 7 |
| `proxyHandler` | main.go:23 | main.go | 3 |
| `usageChan` | main.go:19 | main.go | 3 |
| `execPath` | main.go:20 | main.go | 4 |
| `startTime` | main.go:25 | admin.go | 1 |
| `stats` | usage.go:33 | admin.go | 4 |

### admin.go 中的全局变量访问

admin.go 是重构的难点之一。它通过全局变量访问：
- `cfg.Service.APIKey` (认证, 第 58, 88 行)
- `cfg.Service.Name`, `cfg.Service.Version` (状态信息, 第 132-133 行)
- `stats` (统计信息, 第 94-98 行)
- `db` (SQLite 查询, 第 102 行)
- `sharedUpstreams.GetAll()` (启用列表, 第 125 行)
- `startTime` (uptime 计算, 第 134 行)
- `doReload()` 函数 (第 63 行)

迁移方案：admin handler 构造函数接收需要的依赖：
```go
type AdminHandler struct {
    cfg             *config.Config
    db              *gorm.DB
    stats           *storage.UsageStats
    sharedUpstreams *upstream.SharedUpstreams
    startTime       time.Time
    reloadFn        func() error  // App.doReload 方法的函数引用
}
```

### doReload() 函数分析

doReload() 是最复杂的迁移函数，它修改多个全局变量：
1. 重新读取 config.yaml
2. 创建新 LoadBalancer
3. 更新 SharedUpstreams.upstreams（直接访问私有字段！）
4. 替换全局 lb
5. 更新 proxyHandler.lb 和 proxyHandler.defaultModel
6. 替换全局 cfg

迁移后成为 App 方法，直接操作 App 字段。需要给 SharedUpstreams 添加 `ReplaceAll([]*Upstream)` 方法解决私有字段访问问题。

## Sources

### Primary (HIGH confidence)
- 项目源代码直接分析 — 7 个 Go 文件，1890 LOC 全部阅读
- go.mod 依赖版本 — 已验证
- Go 官方编译器行为 — import cycle 禁止规则
- golang-standards/project-layout — cmd/ + internal/ 约定

### Secondary (MEDIUM confidence)
- [Go import cycle strategies](https://www.dolthub.com/blog/2025-03-14-go-import-cycle-strategies/) — 循环依赖解决策略
- [Constructor injection in Go](https://medium.com/codex/forget-go-dependency-injection-frameworks-do-this-instead-1f2e37d2bf70) — App struct 模式
- [Bubbletea nested models](https://donderom.com/posts/managing-nested-models-with-bubble-tea) — TUI 组合模式

### Tertiary (LOW confidence)
- 无低置信度来源。所有关键发现基于源代码直接分析。

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — 依赖已锁定在 go.mod 中，无需变更
- Architecture: HIGH — 基于源代码分析和 Go 社区最佳实践，迁移路径明确
- Pitfalls: HIGH — 通过源代码审计识别，特别是 SharedUpstreams 私有字段访问问题
- Primary Upstream design: HIGH — 基于现有 LoadBalancer.SelectNext 机制扩展

**Research date:** 2026-04-05
**Valid until:** 2026-05-05 (30 days — Go 项目结构约定稳定)
