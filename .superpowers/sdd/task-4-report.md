# Task 4 Report — Forward, Delete, Remove Flag Actions

## Status: Complete

## Files changed

### `internal/smtp/smtp.go`
- Added `SendRaw(to, from, password string, raw []byte) error` — sends raw MIME bytes via SMTP

### `internal/poller/poller.go`
- Added imports: `bytes`, `mime`, `strings`
- Added `trashFolder string` field to `Poller` struct
- Added 3 new switch cases in `executeActions()`:
  - **forward** — fetches raw MIME via `FetchRawMessage`, builds forward MIME, sends via `smtp.SendRaw`
  - **delete** — auto-detects Trash folder via `getTrashFolder()`, moves message there
  - **remove_flag** — normalizes flag name (e.g. `seen` → `\Seen`), calls `RemoveFlags`
- Added `getTrashFolder()` — caches and returns Trash folder, checking `\Trash` flag then "Deleted Messages" fallback
- Added `buildForwardMIME()` — constructs multipart/mixed forward MIME with message/rfc822 attachment
- Added `setLastError(err)` placeholder for Task 5

### `internal/poller/poller_test.go`
- Added `folders []imap.Folder` field to `trackedMock`
- Updated `ListFolders()` to return `m.folders`
- Added `TestExecuteActionsDelete` — verifies mock returns Trash folder with `\Trash` flag and `MoveMessage` is called with that folder

## Verification

```
go build ./cmd/mailflow/    → BUILD OK
go test ./...               → all passing (10 packages)
go vet ./...                → clean
```
