# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- Weekly volume retention increased from 4 to 24 weeks (~6 months) on stats page
- Top senders limit increased from 15 to 20 on stats page and MCP get_stats
- Date formatting standardized to ISO 8601 on activity log, settings page (last backup, server time)

### Fixed
- Duplicate `class` attributes silently dropping margins on Add Condition, Add Action, and rules list card
- Theme toggle icon now switches between sun/moon based on active theme
- Export/Import icons swapped to correct directions (download=export, upload=import)
- Undefined CSS variable `--fg` fixed to `--text` in chart toggle
- Inline `<style>` block in dashboard status moved to main stylesheet (prevented DOM accumulation)
- Dashboard action buttons and export/import form now wrap properly on mobile
- Stats cards removed `min-width:300px` to fix horizontal overflow on mobile
- Toggle button styling made consistent (Enable=btn-primary across all sections)
- Seed Contacts button promoted to btn-primary for visual consistency
- Search clear (✕) button now toggles correctly on activity page and native browser ✕ suppressed

### Added
- `aria-pressed` on chart toggle buttons with live updates via JS
- `role="tab"`, `aria-selected`, and `aria-controls` on MCP config tab buttons
- Rule delete now returns full rules panel instead of empty body (fixes orphaned headers and stale priorities)
- Drag reorder preserves search query across reorder requests
- Stats page refresh uses HTMX instead of full page reload
- Toast auto-dismiss targets the most recent toast instead of always the first
- Seed Contacts button shows error toast on failure instead of stuck loading state
- Rule reorder shows spinner indicator during save
- Activity search/filter shows loading spinner
- Rule search shows result count with aria-live announcement
- Activity table sender column uses CSS truncation instead of title attribute
- Docs page now loads shared style.css and mobile.css for consistent mobile UX
- Global HTMX error handling — network and server errors show toast notifications on all pages
- Keyboard-accessible rule reorder with Move Up/Move Down buttons
- Dashboard rules table auto-refreshes every 60 seconds
- Folders refresh warns when configured source folder is missing from IMAP server
- Search clear (✕) button on /rules and /activity search inputs
- Column widths on /rules (Actions) and /activity (Status) widened to prevent text truncation

### Removed
- Dead code: PollingHealthy(), generateAPIKey(), SettingsRepo.GetAll(), unexported SelectFolder

## [0.7.6] - 2026-08-01

### Added
- Security headers middleware (X-Content-Type-Options, X-Frame-Options, Referrer-Policy, Permissions-Policy)
- Session cookie Secure flag when behind TLS proxy, constant-time MCP auth comparison
- Poller reconnect activation — IMAP failures now trigger exponential backoff reconnection
- Matched-but-not-seen diagnostic — warns when rule matches without mark_as_read
- UI accessibility: skip-to-main-content link, aria-label/icons, for/id label associations
- Empty state for folder dropdown when IMAP is disconnected
- Chart.js trending graphs on /stats — line chart (daily volume), donut (actions), horizontal bar (folders), vertical bar (weekly), with per-section List/Chart toggle

### Changed
- MCP tool count updated 22 in docs, dashboard/status described as HTML fragment
- Auth blanket statement clarified for /health exception, HTTPS→HTTP in MCP description
- Logout changed GET→POST with CSRF protection
- MCP auth rate-limited to 100 requests/minute/IP
- Color contrast improved: muted text, toasts, badges (AA compliant)
- Lucide pinned to @0.469.0, orphaned web/static/style.css removed

### Fixed
- TLS connection leak on IMAP login failure — connection now closed on error
- useMonoFont race condition — replaced with atomic.Bool
- IMAP \Deleted flag blocked in set_flag action (iCloud silently rejects)
- Template execution and runtime errors now return generic messages to users
- Activity clear-logs button refreshes list after deletion
- Mobile flex-row elements stack vertically
- fs.Sub error checked at startup; rand.Read error checked in API key generation
- Logout button styled to match nav link appearance

### Security
- 14 instances of err.Error() replaced with generic error messages in toasts and HTTP responses
- Session cookies automatically cleaned up every hour
- All forms now include for/id label-input associations

## [0.7.5] - 2026-08-01

### Added
- Debug logging across SMTP, IMAP client, DB migrations, and startup — comprehensive visibility when LOG_LEVEL=debug
- Contacts collection toggle — enable/disable automatic contacts collection from Settings or MCP
- Contacts wipe — delete all collected contacts from Settings or MCP

### Changed
- Top senders increased from 10 to 15 on stats page and MCP `get_stats`

### Fixed
- Dashboard status table uses auto layout — labels compact, values fill width
- Stats link hidden from footer on login/setup pages

## [0.7.4] - 2026-07-31

### Added
- `GET /health` endpoint — public JSON health check with status, uptime, DB/IMAP/poller state, and stats
- MCP `health` tool — same health data available via MCP for agent consumption

### Changed
- AGENTS.md now has mandatory Documentation Updates section; doc-update convention made more prominent
- README: added shields.io badges (Go, Release, License, Docker), fixed operator count (10→13), actions list, and MCP tool count (12→19)
- docs.html: MCP tool count corrected (18→19), added missing health tool, action types, condition fields, date operators, and API endpoints (mcp/toggle, mcp/regenerate, setup, dashboard/status)
- Poller now auto-syncs IMAP folders on each tick — folder renames, deletions, and new folders are detected automatically

## [0.7.3] - 2026-07-31

### Fixed
- MCP tools returning arrays now wrap results in objects: `list_rules`, `list_activity`, `list_folders`, `list_contacts`, `search_contacts`, `backup_rules`

## [0.7.2] - 2026-07-31

### Fixed
- Spinner CSS specificity — `.htmx-indicator { display: none }` now after `.spinner` block
- Activity per_page selector mismatch — handler default (100) now synced with template selection
- Activity page dropdowns right-aligned, search input expanded to fill space
- Rules dropdown sorted alphabetically (case-insensitive)
- Per_page options increased: 100/250/500/1000
- MCP requests blocked by CSRF middleware — `/mcp` path now bypasses CSRF
- MCP config snippets in Settings updated with correct `type` fields (OpenCode, Claude Code, Codex)
- Dashboard rules Edit button right-aligned

## [0.7.1] - 2026-07-31

### Fixed
- Dashboard "IMAP disconnected" warning shown on initial page load before auto-refresh
- HTMX spinner indicators consuming layout space when hidden (opacity → display:none)
- Stats page missing Refresh button — added for convenience
- GitHub release creation step documented as mandatory in AGENTS.md

## [0.6.2] - 2026-07-31

### Added
- Activity From column shows full sender address on hover (title attribute)

### Fixed
- Docs sidebar active nav — removed broken `.docs-sidebar nav a.active` CSS rule and IntersectionObserver JS

### Removed
- Unmatched messages counter from stats page — was a lifetime counter, misleading since unmatched messages eventually get handled

## [0.7.0] - 2026-07-31

### Added
- Body content matching in rules engine (`body` field)
- Arbitrary header matching in rules engine (`header:X-Name` field)
- Date matching operators: older_than, newer_than, before, after
- Forward rule action (IMAP raw fetch + SMTP send with message/rfc822 attachment)
- Delete rule action (auto-detect Trash folder via \Trash flag)
- remove_flag action for rule conditions
- IMAP connection health with exponential backoff reconnect (5s→10s→20s→40s→60s)
- Activity log search, filter by rule/status, and pagination (25/50/100 per page)
- Poller dashboard status card with auto-refresh every 30s (HTMX)
- Mobile responsive CSS (<768px breakpoints)
- HTMX loading spinners on async buttons (Seed Contacts, Run Poll, Backup Now)
- Inline rule deletion via hx-confirm (no separate confirmation page)
- 7 new MCP tools: enable_rule, disable_rule, get_poller_status, import_rules, list_contacts, clear_activity, seed_contacts
- Date field on imap.Message struct (from Envelope.Date)
- Pre-fetch caching of message body/headers for rules evaluation

### Changed
- Evaluate() and Match() now accept imap.Client parameter for body/header fetching
- activityHandler accepts rulesRepo for rule filter dropdown
- Poller.NewPoller() accepts imapEmail and imapConnect factory
- MCP New() accepts collector and settingsRepo parameters
- Dashboard status card auto-refreshes every 30s via HTMX

### Removed
- rules_delete.html template (replaced by inline hx-confirm)
- GET /rules/{id}/delete route
- rulesDeleteConfirmHandler

## [0.6.1] - 2026-07-31

### Added
- **Persistent stats** — stats now stored in a dedicated `stats` table, independent of the activity log. Clearing activity logs no longer resets statistics.
- 4 new stat categories: **unmatched messages**, **error/success rates**, **messages by folder** (distribution), **weekly volume**
- Stats backfill on migration — existing activity log data is populated into the new stats table on first run
- `StatsRepo.IncrementStat()` for atomic upsert increments (no race conditions)

### Changed
- Stats page redesigned with 3 new cards: unmatched/error summary row, folder distribution bars, weekly volume table
- Activity log confirm dialog updated — no longer warns about resetting stats
- Docs updated with new stats categories and persistence explanation

## [0.6.0] - 2026-07-31

### Added
- **MCP (Model Context Protocol) server** — remote access for AI agents (Claude Code, OpenCode, Codex) with 12 tools
- `internal/mcp/` package — `New(*sql.DB, imap.Client, *poller.Poller) *StreamableHTTPServer` with Bearer token auth middleware
- MCP Access settings card with enable/disable toggle, API key generation/regeneration, copy button, and platform config snippets
- 12 MCP tools: `list_rules`, `get_rule`, `create_rule`, `update_rule`, `delete_rule`, `check_email`, `list_activity`, `get_stats`, `run_poll`, `backup_rules`, `list_folders`, `search_contacts`
- MCP endpoint docs in the built-in documentation page (Settings Reference + API Reference + tools table + platform config)
- Dependency: `github.com/mark3labs/mcp-go@v0.57.0`

## [0.5.1] - 2026-07-31

### Added
- **Activity page refresh button** — reloads activity content via HTMX without a full page reload
- **Scheduled rules backup via email** — exports rules as JSON attachment and emails to a configurable recipient (defaults to self) via iCloud SMTP
- Backup settings card with enabled toggle, frequency select (daily/weekly/monthly, default weekly), recipient input
- "Backup Now" button for manual backups outside the schedule
- `internal/smtp/` package — MIME multipart email sender using `net/smtp.SendMail`
- **Rules search by condition value** — search now matches email addresses and domains in rule conditions, not just name/description
- Documentation convention in AGENTS.md — new features/packages must update README, docs, and AGENTS.md

### Changed
- Backup schedule checked after each poll tick — persists `last_backup` timestamp to DB across restarts

### Fixed
- **Unmatched message pipeline deadlock** — unmatched messages no longer block subsequent messages with matching rules. Poller now uses `UID >= N` search range to skip past already-seen unmatched messages instead of bailing early.

## [0.5.0] - 2026-07-30

### Added
- `LOG_LEVEL=debug` environment variable for verbose logging in Docker
- `slog.Debug` traces for every condition evaluation, poller state, rule matching
- Client-side search filter for rules list

### Changed
- ~50 inline styles consolidated into 5 CSS utility classes
- Dead code removed, license updated (2026), funding link added
- Screenshots added to README
- HTMX 1.9.10→2.0.10, highlight.js 11.9.0→11.11.2
- 21 new tests — coverage `db` 60%→73%, `web` 27%→45%
- Foreign keys enabled on SQLite — `ON DELETE CASCADE` now works
- 3 database indexes added (`condition_groups.rule_id`, `conditions.group_id`, `actions.rule_id`)
- Poller mutex replaced with atomic counter — `LastTick()` no longer blocks
- Regex compiled once on load instead of per-evaluation

### Fixed
- **Skip-list for unmatched messages** — when a message has no matching rule, it's added to a skip list and filtered from subsequent search results. Other messages in the folder continue processing instead of being blocked.
- **Single-pass action execution** — `mark_as_read` now executes BEFORE `move_to_folder` in declared order. iCloud requires `\Seen` flag before allowing MOVE operations.
- **Poll loop** — calls `SearchMessages(source, 1)` repeatedly until folder is empty or batch limit reached. iCloud caps all IMAP queries to ~1 result per call.
- **iCloud IMAP constraints** documented in AGENTS.md — `\Seen` before MOVE, no `STORE \Deleted`, query result caps, UIDSearch seen/unseen behavior.

## [0.4.4] - 2026-07-30

Superseded by 0.5.0.

## [0.4.3] - 2026-07-30

### Added
- Client-side search filter for rules list (name + description, instant)
- Demo database now works correctly (fixed bcrypt hash, RFC 3339 timestamps, correct cookie instructions)

### Changed
- Foreign keys enabled on SQLite — `ON DELETE CASCADE` now works, fixing silent orphaned rows
- 3 database indexes added (`condition_groups.rule_id`, `conditions.group_id`, `actions.rule_id`)
- N+1 query pattern eliminated in `RulesRepo.List()` — 3N+1 queries reduced to 4 total
- Poller mutex replaced with atomic counter — `LastTick()` no longer blocks on IMAP round-trips
- Regex compiled once on load instead of per-evaluation in `matches_regex` conditions
- IMAP messages fetched in a single batch instead of one at a time
- ~50 inline styles consolidated into 5 CSS utility classes (`.muted`, `.flex-between`, `.flex-row`, `.mt-*`)
- Timezone list moved from hardcoded HTML to Go data-driven range loop
- Dead code removed: `csrfCookie()`, `App.ImapClient`, `AdminPass`, unused `.docs-layout` CSS
- Shared test helper `db.NewTestDB(t)` replaces 3 duplicate implementations
- 25 new tests added — coverage `db` 60%→73%, `web` 27%→45%
- HTMX 1.9.10→2.0.10, highlight.js 11.9.0→11.11.2

### Fixed
- iCloud `UID SEARCH` replaced with `UID FETCH 1:*` — fixes 1-email-per-poll-tick bug caused by iCloud search result capping
- Confirm dialog now warns about stats reset when clearing activity logs
- `rand.Read()` errors checked in config and web packages
- `cfg.Save()` error handled on encryption key generation
- `EnsureCatchAll()` wrapped in a transaction — no more partial cleanup on interruption
- `Delete()` now relies on foreign key cascade (simplified from manual child-row deletion)

## [0.4.2] - 2026-07-30

### Added
- Light/dark theme toggle with OS preference detection (moon icon in nav)
- Demo data script (`scripts/demo.sh`) for screenshots and testing

### Fixed
- Dynamic host in docs URLs (shows actual server host, not internal bind)
- How It Works card missing closing `</div>`
- nil guard on CreateFolder when IMAP not connected
- Demo script: added table migrations and clearer login instructions
- Ignored demo WAL files in git

## [0.4.1] - 2026-07-30

### Added
- Stats dashboard (`/stats`) — rule hit counts (bar chart), top senders, actions breakdown, daily volume
- Messages processed count on Dashboard status table
- Documentation page (`/docs`) with full usage guide and API reference
- API reference covers all 21 endpoints with curl examples and syntax highlighting (highlight.js)
- Configurable log retention in Settings (default 1000 entries)
- Server metrics in Settings → Server (time, uptime, memory)
- Clear activity logs button with confirmation
- Link to docs in navigation
- Link to stats in footer

### Changed
- Global link color now visible on dark background
- Docs page: linear layout with indented sub-content
- Code blocks in dark sub-containers for visual separation
- Export/Import on same line with left/right alignment
- Refresh button renamed to "Refresh folders"
- Removed icons from cramped Edit/Del buttons

## [0.4.0] - 2026-07-30

### Added
- Lucide icons throughout the UI (nav, buttons, drag handle)
- JetBrains Mono font with toggle in Settings → Regional
- Favicon (SVG + PNG + Apple touch icon)
- Footer with author, GitHub, MIT license, version, copyright
- Brand gradient text header
- Polling enable/disable toggle on Settings
- Dashboard polling status + next poll time display
- Contacts count in Settings → Server section
- Configurable poll batch size (messages per poll)
- Known Issues section in README (iCloud web session conflict)

### Changed
- Default condition operator: OR (was AND)
- Default poll interval: 300s (was 60s)
- Listen address shows actual host (not internal bind)
- Export/Import buttons on same line with left/right alignment
- Refresh button renamed to "Refresh folders"
- Admin password placeholder: "Leave blank to keep current"

### Fixed
- Forced scrollbar prevents layout bounce
- Seed contacts nil-guard (prevents crash when IMAP not connected)
- Static CSS embedded via `//go:embed` + `fs.Sub` (works in Docker)
- Action value input adapts to action type (hidden for mark_as_read/mark_as_unread)
- Condition/action rows have remove (×) buttons
- Docker build: `golang:alpine` + `GOTOOLCHAIN=auto`

## [0.3.2] - 2026-07-30

### Changed
- CSRF disabled on login/setup pages (pre-auth, no session)
- Login rate limiting removed CSRF dependency

### Fixed
- Root path `/` redirects to `/dashboard` (was 404)
- CSS broken in Docker — embedded static files with `fs.Sub`
- `SeedFromFolder` nil-guard prevents panic when IMAP not connected
- Docker build with `golang:alpine` + `GOTOOLCHAIN=auto` (no dependency downgrades)
- Default listen address `0.0.0.0:8080` for Docker compatibility

## [0.3.1] - 2026-07-29

### Added
- AND/OR condition toggle on rule forms (ALL vs ANY matching)
- Docker image auto-build via GitHub Actions on tag push (ghcr.io)
- Clear README with iCloud mail rule setup instructions and how-it-works flow

### Changed
- Activity log table layout fixed at 1200px with proper column widths
- Navigation links hidden when logged out (only brand shows)
- Dockerfile updated to Go 1.23

## [0.3.0] - 2026-07-29

### Added
- AES-256-GCM encrypted IMAP password storage (random key in config.json)
- CSRF protection on all non-HTMX POST forms (login, setup, settings, rules)
- Rate limiting on login (5 attempts/minute/IP)
- Config validation on load (port, poll interval)

### Changed
- Session cookie set to SameSite=Strict
- `renderPage` now sets CSRF token cookie and embeds token in template data
- All form templates include CSRF hidden field
- `setupPage` accepts `*config.Config` for password encryption
- `settingsSaveIMAP` encrypts password before storing

### Fixed
- Build-time version injection via ldflags (`-X main.version=...`)
- Activity log auto-cleanup on startup (keeps last 1000 entries)

## [0.2.0] - 2026-07-29

### Added
- `mark_as_unread` action (removes `\Seen` flag via IMAP `StoreFlagsDel`)
- Rules export/import as JSON from Settings
- Timezone setting with local time display in activity log
- Contact autocomplete (datalist) on rule condition value inputs
- Folder autocomplete (datalist) on rule action value inputs
- Seed Contacts scans all synced folders, shows contact count
- Remove button (×) on condition and action rows in rule editor
- Refresh Folders button on settings polling section
- Activity page in nav, version + repo link in settings footer
- IMAP connection test button on settings

### Fixed
- `mark_as_read` now correctly applies to destination folder after move (uses `SelectMailbox` repositioning)
- Rule conditions on `from`/`to`/`cc` now match against email addresses only (not `Name <Email>` format)
- Catch-all rule no longer requires `has_attachment` condition — matches all unmatched messages
- New rules auto-assign correct priority (before catch-all, not after)
- Drag-and-drop reorder preserves catch-all priority and doesn't duplicate UI elements
- `parseActions`/`parseConditions` now reset before re-parsing form (removing actions actually works)
- Duplicate activity log entries prevented (mutex on poller process, catch-all deduplication)
- Folder list sorted case-insensitively (eBoks now in correct position)
- `_method` override middleware for PUT/DELETE forms (rule editing works)
- Seed Contacts button shows immediate "Scanning..." feedback
- Toast notifications fixed at bottom-right with auto-dismiss and click-to-dismiss

## [0.1.0] - 2026-07-29

### Added
- IMAP client via go-imap v2 (connect, search, fetch, move, set flags, create folder, list folders)
- Periodic email polling from a configurable source folder
- Rule engine with 10 condition operators (equals, contains, starts_with, ends_with, matches_regex, exists, etc.)
- Rule actions: move to folder, mark as read, set flag
- Catch-all fallback rule for unmatched messages
- Email contact collection from message headers (From/To/Cc)
- Seed contacts from all synced IMAP folders (newest messages first)
- iCloud CardDAV address book import (go-webdav/carddav)
- HTMX-powered single-page app with chi v5 router
- Dashboard with status, contacts count, rules overview
- Rules list with drag-and-drop reordering
- Activity log showing all processed messages and rule matches
- Settings page with IMAP test connection, folder refresh, CardDAV import
- Setup flow for first-time configuration
- Session-based authentication with bcrypt password hashing
- SQLite database with 9 tables (modernc.org/sqlite, no CGO)
- AES-256 encrypted credential storage
- Docker Compose support
- 160+ tests across all packages
- Folder auto-creation, source folder dropdown with autocomplete

[0.7.6]: https://github.com/mojoaar/icloud-mailflow/compare/v0.7.5...v0.7.6
[0.7.5]: https://github.com/mojoaar/icloud-mailflow/compare/v0.7.4...v0.7.5
[0.7.4]: https://github.com/mojoaar/icloud-mailflow/compare/v0.7.3...v0.7.4
[0.7.3]: https://github.com/mojoaar/icloud-mailflow/compare/v0.7.2...v0.7.3
[0.7.2]: https://github.com/mojoaar/icloud-mailflow/compare/v0.7.1...v0.7.2
[0.7.1]: https://github.com/mojoaar/icloud-mailflow/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/mojoaar/icloud-mailflow/compare/v0.6.2...v0.7.0
[0.6.2]: https://github.com/mojoaar/icloud-mailflow/compare/v0.6.1...v0.6.2
[0.6.1]: https://github.com/mojoaar/icloud-mailflow/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/mojoaar/icloud-mailflow/compare/v0.5.1...v0.6.0
[0.5.1]: https://github.com/mojoaar/icloud-mailflow/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/mojoaar/icloud-mailflow/compare/v0.4.4...v0.5.0
[0.4.4]: https://github.com/mojoaar/icloud-mailflow/compare/v0.4.3...v0.4.4
[0.4.3]: https://github.com/mojoaar/icloud-mailflow/compare/v0.4.2...v0.4.3
[0.4.2]: https://github.com/mojoaar/icloud-mailflow/compare/v0.4.1...v0.4.2
[0.4.1]: https://github.com/mojoaar/icloud-mailflow/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/mojoaar/icloud-mailflow/compare/v0.3.2...v0.4.0
[0.3.2]: https://github.com/mojoaar/icloud-mailflow/compare/v0.3.1...v0.3.2
[0.3.1]: https://github.com/mojoaar/icloud-mailflow/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/mojoaar/icloud-mailflow/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/mojoaar/icloud-mailflow/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/mojoaar/icloud-mailflow/releases/tag/v0.1.0
