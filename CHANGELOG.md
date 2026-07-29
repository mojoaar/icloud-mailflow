# Changelog

## [0.3.1] — UI & Docker

### Added
- AND/OR condition toggle on rule forms (ALL vs ANY matching)
- Docker image auto-build via GitHub Actions on tag push (ghcr.io)
- Clear README with iCloud mail rule setup instructions and how-it-works flow

### Changed
- Activity log table layout fixed at 1200px with proper column widths
- Navigation links hidden when logged out (only brand shows)
- Dockerfile updated to Go 1.23

## [0.3.0] — Security & Hardening

### Security
- IMAP password encrypted at rest with AES-256-GCM (random key stored in config.json)
- CSRF protection on all non-HTMX POST forms (login, setup, settings, rules)
- Session cookie set to SameSite=Strict
- Rate limiting on login (5 attempts/minute/IP)
- Config validation on load (port, poll interval)

### Fixed
- Build-time version injection via ldflags (`-X main.version=...`)
- Activity log auto-cleanup on startup (keeps last 1000 entries)

### Changed
- `renderPage` now sets CSRF token cookie and embeds token in template data
- All form templates include CSRF hidden field
- `setupPage` accepts `*config.Config` for password encryption
- `settingsSaveIMAP` encrypts password before storing

## [0.2.0] — Bug Fixes & Polish

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

## [0.1.0] — Initial Release

### Core
- IMAP client via go-imap v2 (connect, search, fetch, move, set flags, create folder, list folders)
- Periodic email polling from a configurable source folder
- Rule engine with 10 condition operators (equals, contains, starts_with, ends_with, matches_regex, exists, etc.)
- Rule actions: move to folder, mark as read, set flag
- Catch-all fallback rule for unmatched messages

### Contacts
- Email contact collection from message headers (From/To/Cc)
- Seed contacts from all synced IMAP folders (newest messages first)
- iCloud CardDAV address book import (go-webdav/carddav)

### Web UI
- HTMX-powered single-page app with chi v5 router
- Dashboard with status, contacts count, rules overview
- Rules list with drag-and-drop reordering
- Activity log showing all processed messages and rule matches
- Settings page with IMAP test connection, folder refresh, CardDAV import
- Setup flow for first-time configuration
- Session-based authentication with bcrypt password hashing

### Infrastructure
- SQLite database with 9 tables (modernc.org/sqlite, no CGO)
- AES-256 encrypted credential storage
- Docker Compose support
- 160+ tests across all packages
- Folder auto-creation, source folder dropdown with autocomplete
