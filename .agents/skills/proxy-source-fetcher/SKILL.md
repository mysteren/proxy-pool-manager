---
name: proxy-source-fetcher
description: Load proxy lists from a URL, a local file, or manual input, and parse them (SOCKS5 lines, Proxifly JSON, Telegram/MTProto links) with deduplication. Use when implementing or debugging proxy source ingestion (Milestone 2+).
---

# Proxy Source Fetcher

## When to use

- Adding a source (URL / file / manual input) or refreshing one.
- Parsing txt/json/tg input, protocol normalization, dedup, ingest into SQLite.

## Key rules

- Entry points are `SourceService` methods in `internal/services/source.go`:
  `AddFromURL`, `AddFromFile`, `AddManual`, `RefreshSource`, `ListSources`,
  `DeleteSource`; the file picker is `PickProxyFile` (Go dialog, not HTML input).
- Parsing is pure and lives in `internal/parser/` (`ParseText`, `ParseJSON`,
  `ParseMTProtoText`). Keep it free of I/O and DB so it stays unit-testable.
- **Accepted formats and protocol handling are defined in `docs/REQUIREMENTS.md`
  ("Форматы данных") and enforced by `internal/parser/parser.go`.** The app is
  SOCKS5-only (`socks5h` → `socks5`, scheme-less `host:port` → `socks5`;
  `http`/`https`/`socks4`/unknown are skipped). Change the parser and the docs
  together; do not restate the rule in several places.
- Telegram/MTProto links (`tg://…`, `t.me/…`, `host:port:secret`) go to the
  separate MTProto pool (`internal/parser/mtproto.go`), not the SOCKS5 pool.
- Dedup twice: in-memory key `protocol://host:port` during parse, and
  `UNIQUE(host, port, protocol)` + `INSERT OR IGNORE` in the DB.
- After a successful fetch, emit `source:fetched` with the fetch result.

## Steps (ingest)

1. Resolve the source type: URL (`http(s)://`) → `fetchURL`; file path → read disk;
   otherwise treat as raw text.
2. HTTP: `http.Client{Timeout: 15s}`, follow redirects, branch on `Content-Type`
   (JSON vs text); a non-2xx is an error.
3. Parse (text or JSON), normalizing protocol and lowercasing `host`.
4. Insert per batch in one transaction via `StorageService`; update
   `sources.last_fetched`.
5. Emit `source:fetched`; return counts fetched / added / skipped.

## Pitfalls

- GitHub page links (`.../blob/...`) are auto-rewritten to `raw.githubusercontent.com`;
  `raw.githubusercontent.com` is preferred for lists (page/`tree` URLs are HTML).
- Drop out-of-range or zero ports; lowercase `host` before dedup.
- Detect duplicates with `RowsAffected() == 0`, never `LastInsertId == 0`.
- Do not send MTProto lines into the SOCKS5 pool (or vice versa).

## Pointers

- Input formats, protocols, FR-01…FR-05: `docs/REQUIREMENTS.md`.
- Schema and unique constraints: `docs/DATA_MODEL.md`.
- Persistence: skill `proxy-storage`.
