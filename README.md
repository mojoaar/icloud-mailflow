# iCloud Mailflow

Automated iCloud mail sorting using IMAP rules.

## Features

- **IMAP Rules Engine** — match messages by from/to/cc/subject/attachment, execute actions (move to folder, mark as read, set flags)
- **CardDAV Contacts Import** — import contacts from iCloud address book
- **Email Contact Collection** — automatically extract contacts from processed messages
- **Drag & Drop Rule Reorder** — reorder rules via drag-and-drop on the rules page
- **Activity Log** — see what rules matched and how messages were processed
- **Folder Auto-Create** — source folder is created on iCloud if it doesn't exist
- **Test Connection** — verify IMAP credentials before saving

## Quick Start

```bash
go run ./cmd/mailflow/ -data=./data
```

Open http://127.0.0.1:8080/setup — configure your admin password and iCloud app-specific password.

## Docker

```bash
docker compose up -d
```

## Build Requirements

- Go 1.22+
- No CGO (pure Go SQLite via modernc.org/sqlite)

## Stack

| Component   | Technology                                                |
| ----------- | --------------------------------------------------------- |
| Language    | Go 1.22+                                                  |
| HTTP Router | chi v5                                                    |
| Database    | SQLite (modernc.org/sqlite)                                |
| IMAP        | go-imap v2                                                |
| CardDAV     | go-webdav/carddav                                         |
| Frontend    | HTMX + html/template, no JavaScript framework             |

## Architecture

```
cmd/mailflow/     — entry point
internal/
  config/         — JSON config file
  crypto/         — AES encrypt/decrypt + bcrypt
  db/             — SQLite + migrations + repositories
  imap/           — IMAP client (go-imap v2)
  rules/          — rule evaluation engine
  contacts/       — contact collector from email headers
  carddav/        — iCloud CardDAV contacts importer
  poller/         — periodic email polling
  web/            — chi router, auth, handlers, templates
```

## License

MIT
