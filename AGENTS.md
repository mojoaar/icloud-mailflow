# AGENTS.md

Repository: github.com/mojoaar/icloud-mailflow

## Build
```bash
go build ./cmd/mailflow/
```

## Stack
- Go 1.22+, chi v5, modernc.org/sqlite (no CGO), html/template
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
- `internal/rules/` - rule evaluation engine
- `internal/contacts/` - email contact collector
- `internal/poller/` - periodic email polling
- `internal/crypto/` - AES encrypt/decrypt + bcrypt hashing
- `internal/web/` - chi router, auth, handlers, embedded templates

## Conventions
- No comments unless essential
- Follow existing patterns for new files
- Use the same Go standard library where possible
- No deprecated packages (no go-imap v1)

## Release
1. Bump version in `cmd/mailflow/main.go` (`const version = "X.Y.Z"`)
2. Update `CHANGELOG.md` with new version header and changes
3. Commit: `git add -A && git commit -m "chore: release vX.Y.Z"`
4. Tag: `git tag vX.Y.Z`
5. Push: `git push origin main && git push --tags`
