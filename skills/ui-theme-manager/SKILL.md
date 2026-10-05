---
name: ui-theme-manager
description: Светлая и тёмная тема в React + Wails. Автоопределение системной темы, ручное переключение, подписка на события. Используй при настройке layout и компонентов.
---

# UI Theme Manager

## Когда использовать

- Настройка ThemeProvider.
- Добавление переключателя темы в Настройки.
- Отладка переключения темы.

## Go: ThemeService

### Метод

```go
func (s *ThemeService) GetSystemTheme() string {
    if s.app.Env.IsDarkMode() {
        return "dark"
    }
    return "light"
}
```

### Подписка на системное событие

В `main.go` или в `ThemeService.Startup`:

```go
app.Event.OnApplicationEvent(events.Common.ThemeChanged, func(e *application.AppEvent) {
    theme := "light"
    if app.Env.IsDarkMode() {
        theme = "dark"
    }
    app.Event.Emit("theme:changed", theme)
})
```

## Frontend: ThemeStore (Zustand)

```ts
type Theme = 'system' | 'light' | 'dark'

interface ThemeStore {
  mode: Theme          // выбор пользователя
  resolved: 'light' | 'dark'  // фактическая тема
  setMode: (m: Theme) => void
  setResolved: (r: 'light' | 'dark') => void
}
```

### Логика

- При старте: `mode = 'system'`, `resolved = await GetSystemTheme()`.
- `setMode('system')` — подписаться на `theme:changed`.
- `setMode('light' | 'dark')` — отписаться, установить `resolved` вручную.
- При изменении `resolved` — управлять классом `dark` на `<html>`.

## Применение темы

```ts
useEffect(() => {
  document.documentElement.classList.toggle('dark', resolved === 'dark')
}, [resolved])
```

## CSS-переменные

В `src/index.css` определить переменные для светлой темы в `:root`
и для тёмной — в `.dark`. Использовать палитру shadcn/ui.

## UI в Настройках

Компонент `ThemeSelector`:

- Три кнопки или `RadioGroup`: System / Light / Dark.
- Активная кнопка — подсвечена.
- При смене — вызвать `setMode`.

## Проверка

- [ ] Запуск: тема соответствует системной.
- [ ] GNOME → тёмная тема → приложение темнеет без перезапуска.
- [ ] Ручной выбор Light → приложение светлое, системное событие игнорируется.
- [ ] Возврат на System → снова синхронизация с системой.
- [ ] Класс `dark` стоит на `<html>`, не на `<body>`.

## Частые ошибки

- **Событие приходит до монтирования:** подписаться в `useEffect` с пустым
  массивом зависимостей.
- **Мерцание при запуске:** выставить класс `dark` до рендера React (inline
  script в `index.html`).
- **Не отписывается от события:** сохранить unsubscribe-функцию и вызывать
  её при смене режима.
