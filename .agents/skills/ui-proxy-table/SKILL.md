---
name: ui-proxy-table
description: The proxy pool and MTProto tables — hand-rolled table with server-side pagination, sorting, multi-select, Go-side clipboard copy, re-test, and context menu. Use in Milestones 3–5 when working on frontend/src/pages/Proxies.tsx or MTProto.tsx.
---

# UI Proxy Table

## When to use

- The main "Proxies" and "MTProto" screens.
- Sorting, filters, pagination, selection, copy, export, re-test.

## Key rules

- **No `@tanstack/react-table` / `@tanstack/react-virtual`.** The table is a light
  custom component: sortable headers + server-side pagination (one page in memory).
- Sorting and filtering run **on the backend** via `GetProxies` + `CountProxies`;
  the frontend holds a single page. Virtualization is not needed.
- Clipboard copy goes **through Go** (`ProxyService.CopyToClipboard` →
  `app.Clipboard`), not `navigator.clipboard`, so it works in WebView on GNOME/KDE.
- Columns, badges, toolbar, states and hotkeys are specified in
  `docs/UI_GUIDELINES.md` — implement those, do not invent a parallel set.
- Bindings are imported by relative path (no `@bindings` alias), e.g.
  `../../bindings/proxy-pool-manager/internal/services` from `pages/`.

## Steps

1. State: `page`, `pageSize`, filter, `sortBy`/`sortDir`.
2. Fetch: `GetProxies({ ...filter, limit: pageSize, offset: page * pageSize })`; total
   from `CountProxies(filter)`. Reset `page` to 0 when filter/sort changes.
3. Selection: checkbox column, Shift/Ctrl ranges, header "select page"; keep selected
   IDs **outside** the current page so paging does not reset them.
4. Actions: "Проверить выбранные" → `TestProxies(ids)`; non-working/unchecked/all via
   the matching `TesterService`/`MTProtoService` methods; progress bar + "Отмена".
5. Events: subscribe to `test:progress` to advance only the progress bar, and reload
   the page + count once on `test:completed` (avoid per-tick re-fetch / flicker).

## Pitfalls

- Relative time uses `Intl.RelativeTimeFormat` (no `date-fns`); toasts use the
  project's own `toastStore`, not `sonner`.
- Disable copy/delete when the selection is empty; confirm destructive bulk actions.
- Do not re-read the whole pool on every progress tick.

## Pointers

- Columns, badges, toolbar, hotkeys, states: `docs/UI_GUIDELINES.md`.
- Backend filter/sort/pagination shape: `docs/DATA_MODEL.md`; testing: skill
  `proxy-tester`.
