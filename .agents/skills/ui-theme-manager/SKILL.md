---
name: ui-theme-manager
description: Light/dark theme in React + Wails — auto-detect the system theme, manual System/Light/Dark switching, and syncing on system change events. Use when working on the layout, theme store, or Settings theme control.
---

# UI Theme Manager

## When to use

- Wiring the theme provider, the Settings theme control, or debugging theme flips.

## Key rules

- Go side (`ThemeService` in `internal/services/theme.go`) is only the source of
  truth for **system** mode: `GetSystemTheme()` returns `"dark"`/`"light"` and
  `ServiceStartup` re-emits `events.Common.ThemeChanged` as `theme:changed`.
- Frontend state is the zustand `themeStore` (`mode: "system" | "light" | "dark"`,
  `resolved: "light" | "dark"`); the manual choice persists in `localStorage`.
- Apply the theme by toggling the `dark` class on `<html>` — never `<body>`.
- On startup, seed `resolved` from `GetSystemTheme()`; an inline script in
  `index.html` sets the class before React mounts to avoid a flash.
- Colors come only from shadcn/ui CSS variables (`--background`, `--foreground`,
  `--primary`, …) — see `docs/UI_GUIDELINES.md` ("Темы").

## Steps

1. `App.tsx` on mount: `ThemeService.GetSystemTheme()` → `setSystemTheme(theme)`.
2. Subscribe to `theme:changed` via `Events.On`; return the unsubscribe from
   `useEffect`.
3. `setMode("light"|"dark")` applies `resolved` immediately; `setMode("system")`
   defers to the next Go report.
4. `setSystemTheme` must ignore updates when `mode !== "system"` (manual choice wins).
5. `ThemeSelector` in Settings: three options System / Light / Dark calling `setMode`.

## Pitfalls

- Subscribing in `useEffect` (empty/no missing deps) so no events are missed or
  leaked; always call the returned unsubscribe.
- Flash on launch: the inline script must run before React renders.
- Register `theme:changed` as a typed event (`application.RegisterEvent[string]`) or
  the generated TS API will not expose it.

## Pointers

- Themes and Settings UI: `docs/UI_GUIDELINES.md`.
- Bootstrap/theme wiring: `docs/ARCHITECTURE.md`; skill `project-bootstrap`.
