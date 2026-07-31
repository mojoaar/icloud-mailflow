# Task 7 Report: Dashboard Status Endpoint + Activity Log Enhancements

## Changes

### A. `internal/web/render.go` — Template FuncMap
- Added `subtract` and `add` template functions for pagination math
- Extracted `templateFuncs` variable for reuse

### B. `internal/web/dashboard.go` — dashboardStatusHandler
- Added `dashboardStatusHandler` that renders `dashboard_status` partial with live poller data
- Returns `PollingHealthy`, `LastTick`, `LastDuration`, `Processing`, `NextPoll` from poller status
- Includes rule count, folder count, contact count, processed count

### C. `internal/web/web.go` — Routes
- Added `GET /dashboard/status` route pointing to `dashboardStatusHandler`
- Updated activity route to pass `rulesRepo` for rule dropdown

### D. `internal/web/activity.go` — Rewritten with query params
- Parses `q`, `rule`, `status`, `per_page`, `page` query params
- Uses `ListFiltered` for paginated, filtered, searchable results
- Computes `totalPages`, passes `RuleNames` list to template
- Supports HTMX partial rendering via HX-Request header check

### E. `internal/web/templates/dashboard.html` — Split + auto-refresh
- Status card replaced with HTMX div: `hx-get="/dashboard/status" hx-trigger="every 30s" hx-swap="innerHTML"`
- Extracted status card content into `{{define "dashboard_status"}}` partial
- Partial includes poll health warning, last poll time/duration, spinner during processing

### F. `internal/web/templates/activity.html` — Search/filter/pagination
- Search input with `keyup changed delay:300ms` debounced HTMX
- Rule dropdown (populated from database, excluding `_catch_all`)
- Status dropdown (All/Success/Error)
- Per-page selector (25/50/100)
- Pagination with Prev/Next buttons using `subtract`/`add` template funcs

### G. `internal/web/activity_test.go` — Updated signatures
- Added `rulesRepo` parameter to match new `activityHandler` signature

## Verification
```
go build ./cmd/mailflow/ && go test ./... && go vet ./...
```
All ok.
