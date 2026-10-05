---
name: project-bootstrap
description: Инициализация Wails v3 проекта с React + TypeScript, настройка Tailwind CSS и shadcn/ui, базовая тёмная/светлая тема. Используй в начале проекта или при добавлении UI-инфраструктуры.
---

# Project Bootstrap

## Когда использовать

- На старте проекта.
- При добавлении Tailwind, shadcn/ui, провайдера темы.
- При настройке Wails-биндингов и событий.

## Шаги

### 1. Инициализация Wails

```bash
wails3 init -n proxy-pool-manager -t react-ts
cd proxy-pool-manager
wails3 dev  # проверка, что всё запускается
```

### 2. Tailwind CSS

Установить в `frontend/`:

```bash
npm install -D tailwindcss postcss autoprefixer
npx tailwindcss init -p
```

Настроить `tailwind.config.js`:

- `content: ['./index.html', './src/**/*.{ts,tsx}']`
- `darkMode: 'class'`
- Расширить `theme` цветами через CSS-переменные (см. shadcn/ui).

Добавить в `src/index.css`:

- Директивы `@tailwind base; @tailwind components; @tailwind utilities;`
- CSS-переменные для светлой и тёмной темы (`.dark { ... }`).

### 3. shadcn/ui

```bash
npx shadcn@latest init
npx shadcn@latest add button input table checkbox dialog dropdown-menu toast sonner
```

Компоненты окажутся в `src/components/ui/`.

### 4. ThemeProvider (React)

- Создать `src/stores/themeStore.ts` на Zustand:
  - Состояние: `theme: 'system' | 'light' | 'dark'`.
  - Экшены: `setTheme`, `applyTheme`.
  - `applyTheme` управляет классом `dark` на `document.documentElement`.

- В `App.tsx`:
  - При монтировании — вызвать `GetSystemTheme()` из Wails-биндингов.
  - Подписаться на событие `theme:changed`.
  - При изменении — обновить store.

### 5. Go: ThemeService

- Метод `GetSystemTheme() string` — возвращает `light` или `dark`
  на основе `app.Env.IsDarkMode()`.
- Подписка на `events.Common.ThemeChanged` в `main.go`:
  - При срабатывании — эмитить `theme:changed` на фронтенд.

### 6. Layout

- Sidebar слева (240px): «Прокси», «Источники», «Настройки».
- Main area: рендер текущей страницы.
- Использовать `react-router-dom` (HashRouter — рекомендуется для Wails).

## Проверка

- [ ] `wails3 dev` запускается.
- [ ] Переключение системной темы (GNOME: Настройки → Внешний вид) меняет
      тему приложения без перезапуска.
- [ ] В Настройках есть ручной выбор темы.
- [ ] `wails3 build` проходит.

## Частые ошибки

- **Не работает переключение темы:** проверить, что класс `dark` ставится
  на `<html>`, а не на `<body>`.
- **Событие не приходит:** проверить, что эмиссия идёт через
  `app.Event.Emit("theme:changed", theme)` и на фронте слушается
  `window.runtime.EventsOn("theme:changed", ...)`.
