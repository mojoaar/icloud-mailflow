# Task 1 Report: IMAP Client — FetchMessageHeader, FetchMessageBody, FetchRawMessage + Date

## Status: DONE

## Commits

- `9e463c9 feat: add FetchMessageHeader, FetchMessageBody, FetchRawMessage to IMAP client + Date field on Message`

## Files Changed

| File | Change |
|------|--------|
| `internal/imap/client.go` | Added 3 interface methods + implementations, `Date time.Time` field to `Message`, populated in `convertMessage()` |
| `internal/poller/poller_test.go` | Added `rawMessages`, `messageBodies`, `messageHeaders` fields + 3 method implementations on `trackedMock` |
| `internal/web/web_test.go` | Added 3 stub methods returning empty/zero values on `mockIMAPClient` |
| `internal/contacts/collector_test.go` | Added 3 stub methods on `mockClient` (unplanned — discovered during verification) |

## Test Results

```
ok  github.com/mojoaar/icloud-mailflow/cmd/mailflow  0.413s
ok  github.com/mojoaar/icloud-mailflow/internal/carddav  (cached)
ok  github.com/mojoaar/icloud-mailflow/internal/config   (cached)
ok  github.com/mojoaar/icloud-mailflow/internal/contacts 0.306s
ok  github.com/mojoaar/icloud-mailflow/internal/crypto   (cached)
ok  github.com/mojoaar/icloud-mailflow/internal/db       (cached)
ok  github.com/mojoaar/icloud-mailflow/internal/imap     (cached)
ok  github.com/mojoaar/icloud-mailflow/internal/mcp      (cached)
ok  github.com/mojoaar/icloud-mailflow/internal/poller   (cached)
ok  github.com/mojoaar/icloud-mailflow/internal/rules    (cached)
ok  github.com/mojoaar/icloud-mailflow/internal/smtp     (cached)
ok  github.com/mojoaar/icloud-mailflow/internal/web      (cached)
```

`go build ./cmd/mailflow/`, `go test ./...`, and `go vet ./...` all pass clean.

## Concerns

1. **Plan code used wrong go-imap v2 API field names.** The plan specified `FetchOptions.Body` (should be `BodySection`), `FetchItemBodySection.Header` (should be `HeaderFields` with `Specifier: PartSpecifierHeader`), and iterated `raw[0].Body` (should use `raw[0].FindBodySection(&fetchItem)`). The actual implementations use the correct v2.0.0-beta.8 API. This will need to be reflected in the plan for subsequent tasks (Tasks 3, 4, 5, 6, 7) that reference these new methods.
