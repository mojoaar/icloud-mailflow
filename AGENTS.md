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
- MCP via github.com/mark3labs/mcp-go

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
- `internal/imap/` - IMAP client (go-imap v2), Message types with Date field, body/header/raw fetch
- `internal/mcp/` - MCP server (go-mcp) with 18 tools, API key auth middleware
- `internal/rules/` - rule evaluation engine with body/header/date matching
- `internal/contacts/` - email contact collector
- `internal/carddav/` - iCloud CardDAV contacts importer
- `internal/poller/` - periodic email polling with forward/delete/remove_flag + IMAP health
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

## Versioning
- Semantic versioning per https://semver.org (MAJOR.MINOR.PATCH)
- Version stored in `cmd/mailflow/main.go` (`const version`) and `VERSION` file — keep both in sync
- MAJOR: breaking changes (API, removed features, DB schema breaks)
- MINOR: new features (new package, new endpoint, new capability)
- PATCH: bug fixes, performance, refactoring without behavior changes
- Agent determines bump level from changelog entries

## Changelog
- Follows https://keepachangelog.com/en/1.0.0/ format
- `[Unreleased]` section at top to accumulate changes before release
- Version headers: `## [X.Y.Z] - YYYY-MM-DD`
- Standard section types only: Added, Changed, Deprecated, Removed, Fixed, Security
- Release dates in ISO 8601 format (`YYYY-MM-DD`)

## iCloud IMAP Constraints

iCloud's IMAP implementation has non-standard behavior that must be accounted for:

- **`\Seen` required before MOVE**: iCloud blocks MOVE operations on unread messages. Always apply `mark_as_read`/`\Seen` BEFORE any `move_to_folder` action. Single-pass execution preserves declared action order — never reorder moves before flags.
- **`STORE \Deleted` is blocked**: iCloud silently rejects `STORE +Flags \Deleted`. Do not use COPY+STORE+EXPUNGE. Use `UID MOVE` with `\Seen` applied first.
- **Query result cap**: All IMAP queries (`UIDSearch`, `UID FETCH`) return at most ~1 result per call regardless of criteria or range. Process messages in a loop — call `SearchMessages(source, 1)` repeatedly until the folder is empty or batch limit is reached.
- **Seen/unseen**: `UIDSearch` with empty criteria returns only unseen messages by default. `UID FETCH 1:*` returns all messages. Use `UIDSearch` for automated polling (natural deduplication via `\Seen`). Never use `UID FETCH` with wildcard range (`1:*`) — it hangs on iCloud after an initial successful response on a fresh connection.
- **Unmatched message deadlock**: Since `UIDSearch` with empty criteria returns only unseen messages, an unmatched unread message blocks the pipeline — it stays unseen and gets returned by every subsequent call. Use `UID >= N` range criteria (the `minUID` parameter on `SearchMessages`) to skip past already-examined UIDs. After each unmatched message, set `minUID = unmatchedUID + 1` so the next `UIDSearch` only returns messages beyond the blocker.

## Release
1. Determine bump level (MAJOR/MINOR/PATCH) from changelog entries
2. Bump version in `cmd/mailflow/main.go` and `VERSION` (keep in sync)
3. Move `[Unreleased]` entries to new version section with date in `CHANGELOG.md`
4. Add comparison link to bottom of `CHANGELOG.md`: `[X.Y.Z]: https://github.com/mojoaar/icloud-mailflow/compare/vPREVIOUS...vX.Y.Z`
5. Commit: `git add -A && git commit -m "chore: release vX.Y.Z"`
6. Tag: `git tag vX.Y.Z`
7. Push: `git push origin main && git push --tags`
8. Create GitHub release: `gh release create vX.Y.Z --title "vX.Y.Z — <brief description>" --notes-file <(extract_changelog_section) --latest`
