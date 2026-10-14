---
name: ui-source-manager
description: The Sources page — add a source by URL, file picker, or manual input; list sources with last-updated time and proxy counts; refresh and delete. Use in Milestone 2 when working on frontend/src/pages/Sources.tsx.
---

# UI Source Manager

## When to use

- The "Источники" page, its add form and the source list.

## Key rules

- Add tabs: `URL` / `Файл` / `Вручную`, laid out and labelled per
  `docs/UI_GUIDELINES.md` ("SourceManager"). Do not add modals for these simple
  operations.
- All work goes through `SourceService`: `AddFromURL`, `AddFromFile`, `AddManual`,
  `RefreshSource`, `ListSources`, `DeleteSource`. The file dialog is the Go binding
  `PickProxyFile` (`app.Dialog.OpenFile`) — **not** an HTML `<input type="file">`.
- URL input must start with `http://` or `https://`.
- After a successful add/refresh, `SourceService` emits `source:fetched`; the page
  reloads the source list (and the pool page reacts on its own).

## Steps

1. URL tab: input + optional name (default to the URL host) + "Добавить".
2. File tab: "Выбрать файл" → `PickProxyFile()`, show the chosen path, then add.
3. Manual tab: textarea with example placeholders + "Добавить в пул".
4. List: card per source with name (or URL), relative last-fetched time, proxy count,
   and "Обновить" / "Удалить" actions.
5. Delete asks for confirmation; working proxies stay in the pool
   (`source_id = NULL`) — see `docs/DATA_MODEL.md` for the exact cascade.

## Pitfalls

- Relative time via `Intl.RelativeTimeFormat` (no `date-fns`).
- Avoid duplicate sources: dedupe by URL/path (`FindSourceByURL` /
  `FindSourceByFilePath`) and offer "Обновить" instead of adding a second copy.
- Show progress for large files rather than freezing the UI.

## Pointers

- Layout, tabs, cards: `docs/UI_GUIDELINES.md`.
- FR-01…FR-05 and accepted formats: `docs/REQUIREMENTS.md`.
- Source deletion semantics: `docs/DATA_MODEL.md`; fetching: skill
  `proxy-source-fetcher`.
