---
name: project-bootstrap
description: Инициализация и очистка Wails v3 проекта с React + TypeScript, Tailwind CSS v4, shadcn/ui, layout без роутера и тёмная/светлая тема. Используй в Milestone 1 и при добавлении UI-инфраструктуры.
---

# Project Bootstrap

## Когда использовать

- На старте проекта (Milestone 1).
- При добавлении Tailwind, shadcn/ui, провайдера темы.
- При настройке Wails-биндингов и событий.

## Принципы

- **Минимум зависимостей.** Не добавлять `react-router`, `@tanstack/react-table`,
  `@tanstack/react-virtual`, `date-fns`, `sonner`. Роутер не нужен (три страницы
  переключаются состоянием), таблица — серверная пагинация, относительное время —
  встроенный `Intl.RelativeTimeFormat`, тост — свой маленький компонент.
- **shadcn/ui** — это не библиотека, а скопированные в репозиторий компоненты.
  Их можно править и заменять.
- **Стек:** `react`, `react-dom`, `tailwindcss` (v4) + `@tailwindcss/vite`,
  зависимости shadcn (`@radix-ui/*`, `class-variance-authority`, `clsx`,
  `tailwind-merge`, `lucide-react`), `zustand`.

## Шаги Milestone 1

### 1. Очистка шаблона

Шаблон `wails3 init -t react-ts` уже создан. Нужно убрать демо:

- `greetservice.go` — удалить.
- `main.go` — убрать `GreetService`, событие `time`, заполнить
  `Name`/`Description`, заголовок окна, убрать демо-горутину.
- Переименовать Go-модуль (`go.mod`: `module changeme` → осмысленное имя),
  удалить демо-биндинги в `frontend/bindings/`, `frontend/src/App.tsx`
  переписать с нуля.
- Заполнить `build/config.yml` (`info.companyName`, `productName`,
  `productIdentifier`, `description`, `copyright`, `version`).

### 2. Tailwind CSS v4

В `frontend/`:

```bash
npm install -D tailwindcss @tailwindcss/vite
```

- В `vite.config.ts` добавить плагин `@tailwindcss/vite`.
- В `src/index.css` — `@import "tailwindcss";`, CSS-переменные светлой темы
  в `:root`, тёмной — в `.dark`.
- v4 не требует `tailwind.config.js` и `postcss.config.js` по умолчанию
  (конфиг — CSS-first через `@theme`).

### 3. shadcn/ui

```bash
npx shadcn@latest init
npx shadcn@latest add button input table checkbox dialog dropdown-menu separator
```

Компоненты окажутся в `src/components/ui/`.

### 4. Layout (без роутера)

- `src/components/layout/Sidebar.tsx` — 240px, пункты «Прокси», «Источники»,
  «Настройки»; переключают `activePage` в Zustand.
- `src/components/layout/AppLayout.tsx` — sidebar + main area.
- `src/stores/uiStore.ts` — `activePage: 'proxies' | 'sources' | 'settings'`.
- `App.tsx` рендерит страницу по `activePage`. Никаких URL и роутера.

### 5. ThemeStore (React)

- `src/stores/themeStore.ts` (Zustand):
  - состояние `mode: 'system' | 'light' | 'dark'` и `resolved: 'light' | 'dark'`;
  - `setMode`, `setResolved`;
  - применение — класс `dark` на `document.documentElement`.
- В `App.tsx` при монтировании — `GetSystemTheme()` из биндингов,
  подписка на событие `theme:changed`.
- В `index.html` — inline-скрипт, выставляющий `dark` до рендера React
  (чтобы не было мерцания).

### 6. Go: ThemeService

- Метод `GetSystemTheme() string` на основе `app.Env.IsDarkMode()`
  (подтверждено в Wails v3 beta.27).
- В `main.go` подписка на `events.Common.ThemeChanged`; при срабатывании —
  `app.Event.Emit("theme:changed", theme)`.
- Кастомные события регистрировать через `application.RegisterEvent[T]`,
  чтобы получить типизированный TS-API.

## Проверка

- [ ] `wails3 dev` запускается, демо-кода нет.
- [ ] Переключение системной темы (GNOME: Настройки → Внешний вид) меняет
      тему приложения без перезапуска.
- [ ] В Настройках есть ручной выбор темы (System / Light / Dark).
- [ ] `wails3 build` проходит.

## Частые ошибки

- **Не работает переключение темы:** проверить, что класс `dark` ставится
  на `<html>`, а не на `<body>`.
- **Событие не приходит:** эмиссия через `app.Event.Emit("theme:changed", theme)`,
  на фронте — `Events.On("theme:changed", ...)`.
- **Проблемы с Tailwind v4:** если shadcn/ui несовместим — временно откатиться
  на v3 (`tailwindcss init -p`, `postcss.config.js`, `darkMode: 'class'`).
