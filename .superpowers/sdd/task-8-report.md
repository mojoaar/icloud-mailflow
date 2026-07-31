# Task 8 Report: Replace rule deletion confirmation page with inline hx-confirm

## Changes Made

1. **`internal/web/rules.go`** — Removed `rulesDeleteConfirmHandler` entirely. Updated `rulesDeleteHandler` to:
   - Use `strconv.Atoi` for ID parsing (simpler than `ParseInt`)
   - Return `toast` partial for HTMX requests on error (invalid ID or delete failure)
   - Return HTTP 200 OK for successful HTMX delete (row will be swapped out by `hx-target="closest tr"`)
   - Fall back to redirect for non-HTMX requests

2. **`internal/web/templates/rules_list.html`** — Replaced `<a href="/rules/{{.ID}}/delete">` link with an HTMX button:
   - `hx-delete="/rules/{{.ID}}"` for the actual delete
   - `hx-confirm` for browser-native confirm dialog with rule name
   - `hx-target="closest tr"` + `hx-swap="outerHTML"` to remove the row from the DOM

3. **`internal/web/web.go`** — Removed `r.Get("/rules/{id}/delete", rulesDeleteConfirmHandler(rulesRepo))` route.

4. **Deleted** `internal/web/templates/rules_delete.html` — No longer needed.

## Verification

```
go build ./cmd/mailflow/  # PASS
go test ./...             # PASS (all packages)
go vet ./...              # PASS
```
