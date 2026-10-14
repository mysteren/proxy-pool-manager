---
name: proxy-storage
description: Work with SQLite — schema, migrations, CRUD, filtering, sorting, server-side pagination, and export to TXT/CSV/JSON. Use wherever proxy/source/MTProto data is read or written.
---

# Proxy Storage

## When to use

- CRUD over proxies, MTProto proxies and sources.
- Filtering, sorting and pagination of the pool.
- Schema migrations and export to files.

## Key rules

- Driver is `modernc.org/sqlite` (pure Go, no CGO), opened as `sql.Open("sqlite", path)`.
  The DB file lives under `xdg.DataHome()` — never `os.UserConfigDir()`, never
  hardcoded.
- Open with pragmas `journal_mode = WAL`, `foreign_keys = ON`,
  `busy_timeout = 5000`; serialize writes (`SetMaxOpenConns(1)`) to avoid
  `database is locked`.
- Migrations are numbered SQL files in `internal/db/migrations/`, embedded with
  `//go:embed` and applied at startup via `schema_migrations` (see
  `internal/db/db.go`).
- **Schema, unique constraints, `settings` keys/defaults and export formats live in
  `docs/DATA_MODEL.md`.** Read it before touching tables; update it with the code.
- Entry points: `StorageService` holds the SQL; `ProxyService` and `MTProtoService`
  are the binding facades the frontend calls (`GetProxies`/`CountProxies`,
  `DeleteByFilter`, `ClearStatus`, `CopyToClipboard`, `PickExportPath`,
  `ExportByIDs`/`ExportByFilter`).
- Pagination is server-side: `GetProxies` returns one page (`LIMIT ? OFFSET ?`) and
  the total comes from a separate `CountProxies` with the same `WHERE`.
- Batch insert/update uses one transaction and prepared statements.

## Pitfalls

- Dedup detection: `INSERT OR IGNORE` + `RowsAffected() == 0`; `LastInsertId == 0`
  is unreliable for ignored inserts.
- Sort column (`SortBy`) must be validated against a whitelist — otherwise SQL
  injection.
- Always `defer rows.Close()`; scope all queries with `?` placeholders.
- Deleting a source keeps working proxies in the pool (`source_id = NULL`); the exact
  cascade rules for non-working/unchecked rows are in `docs/DATA_MODEL.md` — do not
  hardcode a different behaviour.
- `proxy_count` is never stored; compute it with `COUNT(*)` by `source_id`.

## Pointers

- Schema, settings, export formats, MTProto/geo tables: `docs/DATA_MODEL.md`.
- Storage layers and dedup decision: `docs/ARCHITECTURE.md`.
- Ingestion path: skill `proxy-source-fetcher`; testing: skill `proxy-tester`.
