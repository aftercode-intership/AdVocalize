# WebSocket Handler Review & Improvement Plan

## Information Gathered

- **Current file**: `backend/internal/handlers/websocket_handler.go`
- **Reference version**: Provided by user (cleaner, from a review/rewrite)
- **Key difference**: The reference version fixes a critical context leak and simplifies send helpers.

## Critical Issues Found in Current File

1. **Context Leak (Resource Leak)**
   - `newContextWithTimeout()` discards the `cancel` function returned by `context.WithTimeout`
   - This leaks a goroutine that waits for the full 90s timeout to expire
   - Under load, goroutine/memory usage will accumulate

2. **Inconsistent Message Sending Patterns**
   - Current file mixes `h.sendError()`, `h.sendTyping()`, and direct `ws.WriteJSON()`
   - Reference version uses a single generic `sendJSON()` helper — cleaner and DRY

3. **Overly Verbose Comments**
   - Some comments explain obvious behavior or outweigh the code itself

4. **Timeout Duration (90s)**
   - Very long for a real-time chat; 25s (reference) is more reasonable

## Plan

| Step | File | Change |
|------|------|--------|
| 1 | `backend/internal/handlers/websocket_handler.go` | Remove `newContextWithTimeout()` helper and replace with inline `context.WithTimeout` that properly calls `cancel()` |
| 2 | `backend/internal/handlers/websocket_handler.go` | Replace `sendError()` and `sendTyping()` with single generic `sendJSON()` helper |
| 3 | `backend/internal/handlers/websocket_handler.go` | Consolidate `ws.WriteJSON(response)` usage to use the new `sendJSON()` helper |
| 4 | `backend/internal/handlers/websocket_handler.go` | Reduce verbose comments where code is self-explanatory |
| 5 | `backend/internal/handlers/websocket_handler.go` | Change service timeout from 90s to 25s |

## Dependent Files
- None — this is a self-contained handler file.

## Status
- [x] Step 1-5: Applied improvements to websocket_handler.go

## Follow-up Steps
- [x] Run `go build ./...` — **PASSED** (no errors)

