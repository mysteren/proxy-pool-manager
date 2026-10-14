# Proxy Pool Manager

Десктопное приложение для сбора, проверки и управления пулом рабочих прокси.

Приложение загружает списки прокси из внешних источников (URL) или из ручного
ввода, проверяет их на работоспособность и скорость, сохраняет рабочие прокси
в локальный пул. Пул можно фильтровать, сортировать, копировать в буфер обмена
и экспортировать в файлы для использования в браузерах, `yt-dlp`, `curl`
и других приложениях.

**Приложение НЕ управляет системным прокси.** Пользователь сам применяет
скопированные прокси в своих инструментах.

## Скачать

Готовые сборки — в [Releases](https://github.com/mysteren/proxy-pool-manager/releases):

- **Linux** — `.deb`, `.rpm`, `.pkg.tar.zst` (Arch) и `.AppImage` (использует системные GTK4 + WebKitGTK 6.0 — Debian 13+, Ubuntu 24.04+, Fedora 39+, Arch/Manjaro, openSUSE Tumbleweed);
- **Windows** — установщик NSIS (`.exe`), портативный `.exe` и `.zip` (запуск без установки);
- **macOS** — `.dmg` (universal: Intel + Apple Silicon).

## Стек

- **Wails v3** — десктопный фреймворк (Go + WebView)
- **Go 1.25+** — бэкенд, сетевые операции, SQLite
- **React 18 + TypeScript** — фронтенд
- **Vite** — сборка фронтенда
- **Tailwind CSS v4 + shadcn/ui** — UI, темы
- **SQLite** (`modernc.org/sqlite`, без CGO) — локальное хранилище пула

## Быстрый старт

Требования: Go 1.25+, Node.js 24+, Wails v3 Beta (`wails3 version`).

```bash
wails3 dev     # запуск в режиме разработки
wails3 build   # сборка релиза
```

## Разработка и проверки

Агент и разработчик работают по правилам [`AGENTS.md`](AGENTS.md) и переносимому стандарту
[`PROJECT-BLUEPRINT.md`](PROJECT-BLUEPRINT.md). Офлайн-проверки:

```sh
# Go (main.go встраивает frontend/dist — нужна заглушка):
mkdir -p frontend/dist && touch frontend/dist/index.html
go test ./internal/... && go vet ./internal/...
golangci-lint run

# Фронтенд:
cd frontend && npm run typecheck && npm run lint && npm test
```

То же через Taskfile: `task check` (lint + typecheck + tests).

## Документация

Вся документация — в [`docs/`](docs/README.md). Скилы для ИИ-агента — в
[`.agents/skills/`](.agents/skills/README.md).

- [REQUIREMENTS.md](docs/REQUIREMENTS.md) — функциональные и нефункциональные требования
- [ARCHITECTURE.md](docs/ARCHITECTURE.md) — архитектура приложения
- [DATA_MODEL.md](docs/DATA_MODEL.md) — модель данных и схема БД
- [CONVENTIONS.md](docs/CONVENTIONS.md) — соглашения кода, инструменты и команды
- [UI_GUIDELINES.md](docs/UI_GUIDELINES.md) — правила интерфейса
- [TESTING_STRATEGY.md](docs/TESTING_STRATEGY.md) — стратегия тестирования
- [ROADMAP.md](docs/ROADMAP.md) — этапы разработки
- [AGENT_WORKFLOW.md](docs/AGENT_WORKFLOW.md) — работа с агентом через Zed

## Статус

Активная разработка. Первая цель — стабильная версия для Linux (GNOME/KDE),
затем macOS и Windows.

## Лицензия

MIT
