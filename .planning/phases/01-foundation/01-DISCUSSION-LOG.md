# Phase 1: Foundation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the analysis.

**Date:** 2026-04-03
**Phase:** 01-foundation
**Mode:** discuss (auto)
**Areas discussed:** Project structure, HTTP handler architecture, Config management, TUI architecture, Upstream routing, Error response format

## Assumptions Presented

### Project Structure
| Assumption | Confidence | Evidence |
|------------|-----------|----------|
| Single main.go file for Phase 1 | Confident | Greenfield project — simplest approach to start |
| config.yaml alongside binary | Confident | Standard Go pattern, aligns with CLAUDE.md |

### HTTP Handler Architecture
| Assumption | Confidence | Evidence |
|------------|-----------|----------|
| Standard net/http with context.Context | Confident | CLAUDE.md specifies net/http native |
| POST /v1/messages exact path | Confident | CORE-01 requires Claude SDK compatible endpoint |
| Pass-through request/response | Confident | CORE-03/CORE-04 require full compatibility |

### Config Management
| Assumption | Confidence | Evidence |
|------------|-----------|----------|
| config.yaml via os.ExpandEnv | Confident | CLAUDE.md §Hot Config explicitly specifies this |
| 3 pre-configured upstreams | Confident | PROJECT.md describes Zhipu, Aicodee, Minimax trio |

### TUI Architecture
| Assumption | Confidence | Evidence |
|------------|-----------|----------|
| tea.NewGoroutine for concurrent model | Confident | CLAUDE.md stack specifies bubbletea; this is the standard pattern |
| TUI as main entry point | Confident | bubbletea is the main loop, HTTP runs in background |

### Upstream Routing
| Assumption | Confidence | Evidence |
|------------|-----------|----------|
| Modulo hash: hash(request_id) % len(upstreams) | Confident | LB-01/LB-02/LB-03 require modulo hash |
| Hash input: x-request-id or client IP | Confident | LB-02 specifies request ID or client IP |

### Error Response Format
| Assumption | Confidence | Evidence |
|------------|-----------|----------|
| Standard Claude error format | Confident | Must maintain SDK compatibility |
| Error code 1001 for upstream failure | Confident | FAIL-04 specifies code 1001 |

## Auto-Resolved

All assumptions were Confident — proceeding with all as-written.

## Notes

This is a greenfield project. No prior phases, no existing code, no ADRs or specs to reference beyond ROADMAP.md, REQUIREMENTS.md, and CLAUDE.md.

Auto mode selected all gray areas and used recommended defaults throughout.
