# Соглашения (конвенции кода)

> **Аудитория:** разработчик (как писать и менять код) и ИИ-агент.
> Короткие always-on правила — [`../AGENTS.md`](../AGENTS.md); переносимый стандарт —
> [`../PROJECT-BLUEPRINT.md`](../PROJECT-BLUEPRINT.md); EN-чеклист — скил
> [`project-conventions`](../.agents/skills/project-conventions/SKILL.md).

## Слои и границы

- Весь доступ к сети, парсингу, тестам прокси и SQLite — **в Go**. Фронтенд не ходит
  в сеть и не читает БД; он вызывает Wails-биндинги из `frontend/bindings/`.
- Биндинги **генерируются** — не редактировать вручную; из компонентов вызывать через
  слой-обёртку (`frontend/src/lib`/`stores`), а не напрямую в JSX.
- Данные через границу идут типами `internal/models`. Не плодить параллельные типы.
- Импорты во фронтенде — «вниз»: `pages → components → stores/lib`. Псевдоним `@` → `frontend/src`;
  у биндингов нет алиаса — импорт относительным путём.
- Никаких системных вызовов (`gsettings`, `iptables`, `nftables`) и системного прокси —
  приложение только управляет пулом.

Слои и поток данных — в [`ARCHITECTURE.md`](ARCHITECTURE.md).

## Go

- Обрабатывать ошибки явно (не `_ = err`), оборачивать с контекстом `fmt.Errorf("…: %w", err)`.
- `context.Context` — первым аргументом; долгие операции отменяемы (`DialContext`, `ctx` в HTTP).
  Не использовать `time.Sleep` для ожидания.
- Массовые записи — одной транзакцией с prepared statements.
- Дедупликация при вставке — `INSERT OR IGNORE`, факт дубликата — по `RowsAffected() == 0`
  (не `LastInsertId == 0`).
- Пути — через `github.com/adrg/xdg` (`xdg.DataHome()`), не `os.UserConfigDir()`, без хардкода.
- Всегда `defer rows.Close()`; запись сериализована (`SetMaxOpenConns(1)`, WAL, `busy_timeout`).
- `sortBy` валидировать по белому списку (защита от SQL-инъекции).
- Логирование — `log/slog`; на верхнем уровне — понятный текст без трейсбека.
- Комментарии — по-русски и про «почему»; идентификаторы — английские.

## TypeScript / React

- Функциональные компоненты + хуки; `tsconfig` в режиме `strict`, `any` запрещён (`unknown` + сужение).
- Состояние — в zustand-сторах (`frontend/src/stores/*Store.ts`); переключение страниц —
  `uiStore.activePage` (роутера нет).
- UI — Tailwind CSS v4 + shadcn/ui (копии компонентов в `frontend/src/components/ui`).
  Цвета — только через CSS-переменные (`--background`, `--primary`, …), без хардкода.
- Без тяжёлых зависимостей: таблица — своя с серверной пагинацией (без virtual-библиотек),
  относительное время — `Intl.RelativeTimeFormat` (без `date-fns`), тосты — свой `toastStore`.
- Буфер обмена — через Go (`app.Clipboard`), не `navigator.clipboard`.
- `import type` для типов; union-типы вместо `enum`; производные значения — вычислением, а
  не дублирующим состоянием; `useEffect` возвращает cleanup (отписка от событий/таймеров).

## Именование

- Go: экспорт `PascalCase`, файлы `snake_case.go`, конструкторы `NewXxxService`; JSON-теги camelCase.
- TS: компоненты `PascalCase.tsx`; хуки/сторы `camelCase` (`use…`, `…Store`).
- Ключи настроек: `snake_case` в таблице `settings`, camelCase на фронтенде.
- События: `domain:action`. Объявляются в `main.go` через `application.RegisterEvent[T]`,
  зеркалятся на фронте подпиской `Events.On(name, …)`.
  Существующие: `theme:changed`, `source:fetched`, `test:progress`, `test:completed`,
  `geo:progress`, `geo:completed`.

## Линтеры, формат и тесты

Инструменты и команды (всё офлайн). Полный прогон — `task check`.

| Что | Инструмент | Команда |
|-----|-----------|---------|
| Go: линт | `golangci-lint` (`.golangci.yml`) | `golangci-lint run` |
| Go: формат | `gofmt`/`goimports` | `golangci-lint fmt` |
| Go: тесты | `go test` | `go test ./internal/...` |
| Фронт: линт/формат | `biome` (`frontend/biome.json`) | `npm run lint` / `npm run format` |
| Фронт: типы | `tsc` | `npm run typecheck` |
| Фронт: тесты | `vitest` | `npm test` |

Задачи: `task lint`, `task typecheck`, `task test`, `task check`. В CI
(`.github/workflows/ci.yml`) выполняются те же проверки.

⚠️ `go test ./...`, `go vet ./...` и `golangci-lint run` компилируют `main.go`, который
встраивает `frontend/dist` (`//go:embed`). Он в git не хранится — перед прогоном нужна
заглушка: `mkdir -p frontend/dist && touch frontend/dist/index.html` (или собранный фронт).

Тесты — рядом с кодом: Go — `*_test.go`; фронт — `*.test.ts` (чистые функции, `vitest`).

**Осознанные отступления (почему):**

- **biome — только линт, без автопереформатирования.** Проверка — `biome lint .`
  (`npm run lint`); форматирование (`npm run format` = `biome format --write .`) применяется
  вручную. Существующий код не переформатируем массово, чтобы не было огромных диффов
  (biome агрессивно разворачивает компактный JSX). Новый код пишем в текущем стиле
  (`.editorconfig`: 2 пробела, LF).
- В `biome` отключены `noUnusedVariables`/`noUnusedImports` — за неиспользуемое отвечает
  `tsc` (`noUnusedLocals`); отключён ассист сортировки импортов (исторический порядок).
  Правила `useExhaustiveDependencies`, `useIterableCallbackReturn`, `noShadowRestrictedNames`
  и a11y (`noLabelWithoutControl`, `noStaticElementInteractions`) понижены до `warn`:
  они подсвечивают пре-существующие места, но не валят проверку — правка отдельной задачей.
- В `golangci-lint` исключены идиоматичные `errcheck` на `Close` (`os.File`, `sql.DB`,
  `sql.Rows`, …) и `fmt.Fprint*`; отключена стилевая рекомендация `staticcheck` QF1001;
  платформенные заготовки `build/` не анализируются.

## Рамки (чтобы не было «оверинжиниринга»)

- Маленькие правки под задачу; не рефакторить «попутно» и не менять файлы вне scope.
- Переиспользовать существующие слои и паттерны, а не добавлять новые абстракции «на будущее».
- Один факт — одно место: меняешь факт в коде/`docs` → обнови затронутый скил (и наоборот).
