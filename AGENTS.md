# AGENTS.md

Repository: github.com/mojoaar/icloud-mailflow

## Build
```bash
go build ./cmd/mailflow/
```

## Stack
- Go 1.25+, chi v5, modernc.org/sqlite (no CGO), html/template
- HTMX frontend, no JavaScript framework
- IMAP via github.com/emersion/go-imap/v2 (not v1 - deprecated)

## Testing
```bash
go test ./...
```

## Lint
```bash
go vet ./...
```

## Run
```bash
go run ./cmd/mailflow/ -data=./data
```

## Docker
```bash
docker compose up -d
```

## Architecture
- `cmd/mailflow/` - entry point
- `internal/config/` - JSON config file
- `internal/db/` - SQLite + migrations + repos
- `internal/imap/` - IMAP client (go-imap v2), Message types
- `internal/mcp/` - MCP server (go-mcp) with 12 tools, API key auth middleware
- `internal/rules/` - rule evaluation engine
- `internal/contacts/` - email contact collector
- `internal/carddav/` - iCloud CardDAV contacts importer
- `internal/poller/` - periodic email polling
- `internal/smtp/` - SMTP MIME multipart email sender
- `internal/crypto/` - AES encrypt/decrypt + bcrypt hashing
- `internal/web/` - chi router, auth, handlers, embedded templates

## Conventions
- No comments unless essential
- Follow existing patterns for new files
- Use the same Go standard library where possible
- No deprecated packages (no go-imap v1)
- Action execution MUST preserve declared order — never reorder `mark_as_read` before `move_to_folder`. iCloud requires `\Seen` flag on a message before allowing MOVE/delete operations. Two-pass execution (moves first, flags second) breaks this. Always execute actions sequentially in declared order.
- When adding a new feature or package, always update documentation: README.md features list + architecture tree, docs.html reference + API endpoints, and AGENTS.md architecture section.

## iCloud IMAP Constraints

iCloud's IMAP implementation has non-standard behavior that must be accounted for:

- **`\Seen` required before MOVE**: iCloud blocks MOVE operations on unread messages. Always apply `mark_as_read`/`\Seen` BEFORE any `move_to_folder` action. Single-pass execution preserves declared action order — never reorder moves before flags.
- **`STORE \Deleted` is blocked**: iCloud silently rejects `STORE +Flags \Deleted`. Do not use COPY+STORE+EXPUNGE. Use `UID MOVE` with `\Seen` applied first.
- **Query result cap**: All IMAP queries (`UIDSearch`, `UID FETCH`) return at most ~1 result per call regardless of criteria or range. Process messages in a loop — call `SearchMessages(source, 1)` repeatedly until the folder is empty or batch limit is reached.
- **Seen/unseen**: `UIDSearch` with empty criteria returns only unseen messages by default. `UID FETCH 1:*` returns all messages. Use `UIDSearch` for automated polling (natural deduplication via `\Seen`). Never use `UID FETCH` with wildcard range (`1:*`) — it hangs on iCloud after an initial successful response on a fresh connection.

## Release
1. Bump version in `cmd/mailflow/main.go` (`const version = "X.Y.Z"`)
2. Update `CHANGELOG.md` with new version header and changes
3. Commit: `git add -A && git commit -m "chore: release vX.Y.Z"`
4. Tag: `git tag vX.Y.Z`
5. Push: `git push origin main && git push --tags`
