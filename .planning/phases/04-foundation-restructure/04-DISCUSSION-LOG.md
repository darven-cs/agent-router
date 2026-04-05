# Phase 4: Foundation Restructure - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-05
**Phase:** 04-foundation-restructure
**Areas discussed:** Directory Layout, Model Select / Primary Upstream, App Struct, Migration Strategy

---

## Directory Layout

| Option | Description | Selected |
|--------|-------------|----------|
| Split tui into 4-5 files | tui/app.go, update.go, view.go, form.go, styles.go — aligns with Phase 5 componentization | ✓ |
| Move tui.go whole | Keep single file, Phase 5 splits later | |
| Extract styles only | Minimal split: styles + types separate | |

**User's choice:** Split by responsibility (5 files)
**Notes:** User confirmed this aligns with Phase 5 TUI componentization goals

### Admin package location

| Option | Description | Selected |
|--------|-------------|----------|
| internal/admin/admin.go | Separate package, cleaner separation from proxy | ✓ |
| Merge into proxy package | Admin endpoints in internal/proxy/admin.go | |

**User's choice:** internal/admin/

### Storage package naming

| Option | Description | Selected |
|--------|-------------|----------|
| internal/storage/usage.go | Matches roadmap naming | ✓ |
| internal/db/usage.go | More descriptive for single-concern | |

**User's choice:** internal/storage/

## Model Select / Primary Upstream

| Option | Description | Selected |
|--------|-------------|----------|
| Global default model | Pick model name from list, becomes global default | |
| Per-upstream model override | Pick upstream, its model overrides global | |
| Two-step: upstream then model | Most flexible but more complex | |

**User's choice:** None of the above — user provided custom design
**Notes:** User wants "Primary Upstream" concept — pin a preferred upstream, auto-fallback on failure. [m] shows upstream list + "Auto (hash)" option. Model name silently replaced per upstream config.

### Startup behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Hash default, pin optional | FNV hash distribution by default, [m] pins to specific upstream | ✓ |
| Always require primary | No auto-distribution, must select primary | |

### Fallback strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Existing retry with priority | Keep 1s/2s/4s backoff, primary first then others | ✓ |
| Try all upstreams first | New fallback order before retry | |

### Unpin mechanism

| Option | Description | Selected |
|--------|-------------|----------|
| Top of menu: 'Auto (hash)' option | Selecting "Auto" returns to hash distribution | ✓ |
| Esc/0 to clear | Keyboard shortcut to clear | |
| No unpin | Stay until changed | |

## App Struct Design

### Struct pattern

| Option | Description | Selected |
|--------|-------------|----------|
| App + constructor injection | Simple struct, no interfaces, direct deps | ✓ |
| App + interfaces | ProxyService interface etc., enables mocking | |

### Callback wiring

| Option | Description | Selected |
|--------|-------------|----------|
| App methods as callbacks | Callbacks struct with func fields, wired by App | ✓ |
| Per-package interfaces | TUI depends on UpstreamManager interface | |

### Lifecycle management

| Option | Description | Selected |
|--------|-------------|----------|
| App manages full lifecycle | NewApp, Run, Shutdown, signal handling | ✓ |
| App as data holder only | main.go orchestrates startup | |

## Migration Strategy

### Approach

| Option | Description | Selected |
|--------|-------------|----------|
| Bottom-up, one file at a time | config → upstream → storage → admin → proxy → tui → main | ✓ |
| Big-bang refactor | Create full target structure, test once | |
| Two-phase: extract then reorganize | Extra step but safer | |

### Verification

| Option | Description | Selected |
|--------|-------------|----------|
| Build + vet each step, smoke test end | go build/vet after each move, manual smoke test | ✓ |
| Automated behavior comparison | HTTP request comparison test | |

## Claude's Discretion

- App struct field names and constructor signatures
- Primary upstream state storage location
- TUI styling for Auto/primary indicators
- Fallback log message format
- go.mod module path

## Deferred Ideas

None — discussion stayed within phase scope.
