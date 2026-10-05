# Архитектура

## Общая схема

```
┌─────────────────────────────────────────────────────┐
│                  Wails Desktop App                  │
│                                                     │
│  ┌────────────────┐         ┌────────────────────┐  │
│  │  React + TS    │◄────────┤   Wails Bindings   │  │
│  │  Frontend      │ events  │  (auto-generated)  │  │
│  └────────────────┘         └──────────┬─────────┘  │
│                                        │            │
│  ┌─────────────────────────────────────▼─────────┐  │
│  │                Go Backend                     │  │
│  │                                               │  │
│  │  ┌──────────────┐  ┌──────────────┐           │  │
│  │  │ SourceService│  │TesterService │           │  │
│  │  └──────┬───────┘  └──────┬───────┘           │  │
│  │         │                 │                   │  │
│  │  ┌──────▼─────────────────▼───────┐           │  │
│  │  │      StorageService (SQLite)   │           │  │
│  │  └────────────────────────────────┘           │  │
│  │                                               │  │
│  │  ┌────────────────────────────────┐           │  │
│  │  │      ThemeService              │           │  │
│  │  └────────────────────────────────┘           │  │
│  └───────────────────────────────────────────────┘  │
│                                                     │
└─────────────────────────────────────────────────────┘
```

## Слои

### Go Backend

- **SourceService** — загрузка источников (HTTP, файлы), парсинг, дедупликация,
  запись в БД через StorageService.
- **TesterService** — тестирование latency и скорости. Пул горутин с семафором,
  `context.WithTimeout`, эмиссия событий прогресса на фронтенд.
- **StorageService** — CRUD над SQLite, фильтрация, сортировка, экспорт.
- **ThemeService** — определение системной темы через `app.Env.IsDarkMode()`,
  подписка на `events.Common.ThemeChanged`, эмиссия события на фронтенд.

### React Frontend

- **Страницы** (переключаются без роутера — поле `activePage` в Zustand):
  - `Proxies` — таблица пула прокси (центральный экран).
  - `Sources` — управление источниками.
  - `Settings` — тема, конкурентность, таймауты, формат копирования.
- **Стейт:** Zustand (простота, минимум бойлерплейта).
- **UI:** Tailwind CSS v4 + shadcn/ui.
- **Таблица:** без внешних table/virtual-библиотек — серверная пагинация,
  сортировка на бэкенде, лёгкая собственная таблица.

### Коммуникация

- **RPC (фронт → бэк):** Wails-биндинги, авто-генерация TS-типов в `frontend/bindings/`.
- **События (бэк → фронт):**
  - `test:progress` — прогресс массового теста.
  - `test:completed` — завершение теста (одиночного/массового).
  - `source:fetched` — источник загружен и распарсен.
  - `theme:changed` — системная тема изменилась.

## Ключевые решения

1. **Вся сеть — в Go.** Фронтенд не делает HTTP-запросов к источникам прокси
   и не тестирует прокси. Он получает уже готовые данные.
2. **SQLite как единственное хранилище.** Файл в
   `~/.local/share/proxy-pool-manager/data.db` (Linux).
3. **Тяжёлые операции — асинхронно с контекстом.** Отмена теста через
   `context.CancelFunc`, сохранённый в TesterService.
4. **Дедупликация на двух уровнях.** При парсинге (in-memory, ключ
   `protocol://host:port`) и при вставке в БД (UNIQUE constraint на
   `host + port + protocol`).
5. **Никаких системных вызовов.** Никаких `iptables`, `gsettings`, `nftables`.
   Приложение — это чистый менеджер пула.
6. **Честная проверка.** Рабочий прокси подтверждается лёгким HTTP-запросом
   через сам прокси, а не только TCP-connect (см. TESTING_STRATEGY.md).
7. **Буфер обмена — через Go** (`app.Clipboard.SetText`), а не
   `navigator.clipboard`: надёжнее в WebView на GNOME/KDE.

## Директории

```
proxy-pool-manager/
├── build/                  # иконки, манифесты для платформ
├── docs/                   # документация
├── skills/                 # скилы для агента
├── frontend/
│   ├── src/
│   │   ├── components/
│   │   │   ├── ui/         # shadcn/ui компоненты
│   │   │   ├── ProxyTable.tsx
│   │   │   ├── SourceManager.tsx
│   │   │   ├── TesterPanel.tsx
│   │   │   └── SettingsPanel.tsx
│   │   ├── pages/
│   │   │   ├── Proxies.tsx
│   │   │   ├── Sources.tsx
│   │   │   └── Settings.tsx
│   │   ├── stores/
│   │   │   ├── proxyStore.ts
│   │   │   ├── sourceStore.ts
│   │   │   └── themeStore.ts
│   │   ├── lib/
│   │   │   └── utils.ts
│   │   ├── App.tsx
│   │   └── main.tsx
│   └── bindings/           # авто-генерация Wails
├── internal/
│   ├── services/
│   │   ├── source.go
│   │   ├── tester.go
│   │   ├── storage.go
│   │   └── theme.go
│   ├── models/
│   │   └── proxy.go
│   ├── parser/
│   │   └── parser.go
│   └── db/
│       ├── db.go
│       └── migrations/
├── main.go
├── go.mod
└── build/config.yml    # конфиг Wails v3 (вместо wails.json)
```

## Зависимости (Go)

- `modernc.org/sqlite` — SQLite без CGO.
- `github.com/adrg/xdg` — кросс-платформенные пути к данным.
- `golang.org/x/sync/semaphore` — ограничение конкурентности.
- `golang.org/x/net/proxy` — подключение через SOCKS5.

(ID источников и прокси — `INTEGER AUTOINCREMENT`, `uuid` не нужен.)

## Зависимости (Frontend)

- `react`, `react-dom`, `typescript`, `vite`.
- `tailwindcss` v4 (+ `@tailwindcss/vite`).
- `shadcn/ui` — компоненты копируются в проект (тянет `@radix-ui/*`,
  `class-variance-authority`, `clsx`, `tailwind-merge`, `lucide-react`).
- `zustand` — стейт и `activePage`.
- Относительное время — встроенный `Intl.RelativeTimeFormat` (без `date-fns`).
- Тосты — маленький собственный компонент (без внешней библиотеки).
