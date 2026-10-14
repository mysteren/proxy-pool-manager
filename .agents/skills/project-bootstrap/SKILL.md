---
name: project-bootstrap
description: Initialize or extend the Wails v3 + React + TypeScript scaffolding — Tailwind CSS v4, shadcn/ui, a router-less layout, and light/dark theme wiring. Use in Milestone 1 or whenever adding UI infrastructure, bindings, or events.
---

# Project Bootstrap

## When to use

- Milestone 1 (bootstrap) and any addition of UI infrastructure.
- Setting up Tailwind v4, shadcn/ui, the layout, or the theme provider.
- Wiring Wails bindings and events.

## Key rules

- **Minimum dependencies.** Do not add `react-router`, `@tanstack/react-table`,
  `@tanstack/react-virtual`, `date-fns`, or `sonner` — see `project-conventions`.
- **shadcn/ui is copied code**, not a library: components land in
  `frontend/src/components/ui` and may be edited.
- **Tailwind v4 is CSS-first**: `@import "tailwindcss";` in `src/index.css`, light
  variables in `:root`, dark in `.dark`; no `tailwind.config.js` /
  `postcss.config.js`.

## Steps

1. **Clean the template** (`wails3 init -t react-ts`): remove the demo
   (`greetservice.go`, `GreetService`, the `time` event), rename the Go module off
   `changeme`, rewrite `frontend/src/App.tsx`, fill `build/config.yml`.
2. **Tailwind v4**: `npm i -D tailwindcss @tailwindcss/vite`; register
   `@tailwindcss/vite` in `vite.config.ts`; set up `src/index.css`.
3. **shadcn/ui**: `npx shadcn@latest init` then `npx shadcn@latest add button input
   table checkbox dialog dropdown-menu separator` (add `badge`, `hint`, … as needed).
4. **Layout without a router**: `components/layout/Sidebar.tsx` + `AppLayout.tsx`;
   page switching via `uiStore.activePage`; `App.tsx` renders the active page.
5. **Theme**: Go `ThemeService` + zustand `themeStore` + an inline `<html>`-class
   script to avoid flash — see skill `ui-theme-manager`.

## Pitfalls

- Theme flips only when the `dark` class is on `<html>` (not `<body>`).
- Cross-process wiring is `GetSystemTheme()` + the `theme:changed` event; register
  typed events with `application.RegisterEvent[T]` in `main.go` or the TS API is
  missing the event.
- Do not leave template demo code or the `changeme` module name behind.

## Pointers

- Milestone 1 checklist: `docs/ROADMAP.md`.
- Layers, deps, directory layout: `docs/ARCHITECTURE.md`.
- Theme details: `docs/UI_GUIDELINES.md` + skill `ui-theme-manager`.
