# Task 5: IMAP Health Monitoring

## Changes

### `internal/poller/poller.go`
- Added fields: `mu`, `lastError`, `consecutiveFailures`, `backoff`, `imapConnect`, `lastTickDuration`
- Added `PollerStatus` struct with JSON tags
- Added `Status()` method returning full poller state
- Added `PollingHealthy()` method (checks `consecutiveFailures == 0`)
- Replaced placeholder `setLastError` with real implementation (stores error, increments failures, triggers reconnect after 2+ failures)
- Added `clearLastError()` to reset error state on successful ticks
- Added `reconnect()` with exponential backoff (5s initial, doubling up to 60s cap)
- Updated `process()` with tick timing via `defer` and `clearLastError()` on clean exit
- Updated `NewPoller()` signature with `imapEmail string` and `connectFn func() (imap.Client, error)`

### `cmd/mailflow/main.go`
- Updated `NewPoller` call with `imapEmail` and IMAP connect factory

### `internal/poller/poller_test.go`
- Updated all 16 `NewPoller` call sites with `""` and `nil` for new params

## Verification

```
go build ./cmd/mailflow/   ✓
go test ./...              ✓ (all 11 packages pass)
go vet ./...               ✓
```
