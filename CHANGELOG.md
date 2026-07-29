# Changelog

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
