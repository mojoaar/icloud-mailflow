# Task 6 — 7 New MCP Tools

## Changes

- Updated `internal/mcp/server.go`:
  - Added `"log/slog"` and `"github.com/mojoaar/icloud-mailflow/internal/contacts"` imports
  - Changed `New()` signature to accept `*contacts.Collector` and `*db.SettingsRepo`
  - Added 7 new MCP tools: `enable_rule`, `disable_rule`, `get_poller_status`, `import_rules`, `list_contacts`, `clear_activity`, `seed_contacts`
  - Total tool count: 18 (was 11)
- Updated `internal/web/web.go`: `mcp.New()` call passes `collector` and `settingsRepo`
- Updated `internal/mcp/server_test.go`: test `New()` calls updated for new signature

## Verification

```
ok  github.com/mojoaar/icloud-mailflow/cmd/mailflow
ok  github.com/mojoaar/icloud-mailflow/internal/mcp
ok  github.com/mojoaar/icloud-mailflow/internal/... (all packages)
go vet ./... — clean
```
