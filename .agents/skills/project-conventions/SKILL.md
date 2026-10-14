---
name: project-conventions
description: Enforceable conventions for ANY change in proxy-pool-manager (Wails v3 + Go + React 18 / TS strict + Tailwind v4 + zustand). Use before writing or reviewing code — layering, TS/React rules, Go rules, naming, events, and the before-finishing checklist.
---

# Project Conventions

Applies to **every** code change. Read `AGENTS.md`, `PROJECT-BLUEPRINT.md`, and the
relevant `docs/*.md` first — this file is the short checklist, not a replacement.

## Layering

- The frontend never touches the network or SQLite directly; it calls Go through the
  generated Wails bindings. All HTTP, parsing and testing happen in Go.
- Go services (`internal/services/`): `SourceService` (fetch/parse),
  `TesterService` (latency/HTTP/speed/geo), `StorageService` (SQLite CRUD),
  `ProxyService` + `MTProtoService` (binding facades), `SettingsService`,
  `ThemeService`.
- Data crosses the boundary as `internal/models` types (`Proxy`, `Source`,
  `ProxyFilter`, `MTProtoProxy`, …). Reuse them; do not invent parallel types.
- No system calls (`gsettings`, `iptables`, `nftables`) and no system-proxy
  management — the app only manages a pool.

## TypeScript / React

- React 18 function components + hooks only. `tsconfig` is `strict`: no `any`
  (use `unknown` + narrowing), avoid non-null `!` unless truly safe.
- State lives in zustand stores (`frontend/src/stores/*Store.ts`); page switching
  uses `uiStore.activePage` — there is **no router**.
- UI = Tailwind CSS v4 + shadcn/ui copied into `frontend/src/components/ui`. Use CSS
  variables (`--background`, `--primary`, …), never hardcoded colors.
- No heavy deps: no table/virtual library (tables are hand-rolled with server-side
  pagination), no `date-fns` (use `Intl.RelativeTimeFormat`), no `sonner` (use the
  project's own `toastStore` / `Toaster`).
- Import through the `@` alias (`@` → `frontend/src`). Wails bindings have **no**
  `@bindings` alias — import them by relative path, e.g.
  `../../bindings/proxy-pool-manager/internal/services` from `pages/`.
- Clipboard goes through Go (a service method using `app.Clipboard`), not
  `navigator.clipboard`.

## Go

- Handle errors explicitly (never `_ = err`); wrap with context
  (`fmt.Errorf("...: %w", err)`).
- Long operations take `ctx context.Context`, run off the main thread, and support
  cancellation; never use `time.Sleep` to wait.
- Batch DB writes run in one transaction with prepared statements.
- Dedup on insert: `INSERT OR IGNORE`, detect via `RowsAffected() == 0` — never via
  `LastInsertId == 0`.
- Paths come from `github.com/adrg/xdg` (`xdg.DataHome()`), never
  `os.UserConfigDir()`, never hardcoded.
- Always `defer rows.Close()`; keep the DB writer serialized
  (`SetMaxOpenConns(1)`, WAL + `busy_timeout`) to avoid `database is locked`.
- Validate SQL `sortBy` against a whitelist (SQL-injection guard).

## Naming

- Go: exported `PascalCase`, files `snake_case.go` (`source.go`, `storage_geo.go`),
  constructors `NewXxxService`. JSON tags are camelCase.
- TS: components `PascalCase.tsx`; hooks/stores `camelCase` (`use…`, `…Store`).
- Settings keys are `snake_case` in the `settings` table, camelCase on the frontend
  (see `Settings` in `internal/services/settings.go`).

## Events (Go → React)

- Declare typed events in `main.go` with `application.RegisterEvent[T]("domain:action")`.
- Emit with `app.Event.Emit(name, payload)`; subscribe with `Events.On(name, …)` and
  always return the unsubscribe function from `useEffect`.
- Existing events: `theme:changed`, `source:fetched`, `test:progress`,
  `test:completed`, `geo:progress`, `geo:completed`.

## Before finishing

- [ ] Change matches the requirement/milestone in scope; no unrelated edits.
- [ ] `go test ./internal/...` and `go vet ./internal/...` pass (add a
      `frontend/dist` stub first — `main.go` embeds it via `//go:embed`).
- [ ] Frontend change: `npm run typecheck` / `npm test` / `npm run lint` pass in `frontend/`.
- [ ] No new heavy dependency, no system calls, no hardcoded paths.
- [ ] Errors handled, no debug leftovers, no swallowed errors.

## Pointers

- Repo rules: `AGENTS.md`; overview: `PROJECT-BLUEPRINT.md`.
- `docs/CONVENTIONS.md` (RU source of truth for these rules + tooling/commands),
  `docs/ARCHITECTURE.md` (layers, key decisions), `docs/DATA_MODEL.md` (schema,
  settings, export), `docs/TESTING_STRATEGY.md` (proxy checking + tests),
  `docs/UI_GUIDELINES.md` (layout, components, hotkeys), `docs/REQUIREMENTS.md`
  (FR/NFR), `docs/ROADMAP.md` (milestones), `docs/AGENT_WORKFLOW.md` (workflow).
