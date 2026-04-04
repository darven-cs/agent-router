# Quick Task 260404-qrg: TUI选择其他上游模型后仍使用Zhipu

## Summary

**Issue:** TUI selects other upstream model but requests still use Zhipu

**Root Cause:** `OnUpstreamModelSelected` callback only updated `sharedUpstreams` but NOT `lb` (LoadBalancer). The proxy uses `lb.SelectNext()` which reads from its own copy, so changes weren't reflected in actual requests.

**Fix:** Added `lb.UpdateUpstream(u)` to `OnUpstreamModelSelected` callback in `main.go`, mirroring the pattern already correctly used in `OnUpstreamToggled`.

## Files Modified

- `main.go` - Added `lb.UpdateUpstream(u)` to `OnUpstreamModelSelected` callback

## Commit

`521604b` - fix(tui): update LoadBalancer when upstream model is selected
