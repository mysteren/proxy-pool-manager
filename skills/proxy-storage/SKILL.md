---
name: proxy-storage
description: Работа с SQLite: схема, миграции, CRUD, фильтрация, сортировка, экспорт в TXT/CSV/JSON. Используй везде, где нужны данные прокси.
---

# Proxy Storage

## Когда использовать

- Реализация CRUD над прокси и источниками.
- Фильтрация и сортировка пула.
- Экспорт в файлы.
- Миграции схемы.

## Библиотека

`modernc.org/sqlite` — чистый Go, без CGO. Регистрируется как драйвер `sqlite`.

```go
import (
    "database/sql"
    _ "modernc.org/sqlite"
)

db, err := sql.Open("sqlite", dbPath)
```

Прагмы при открытии:

```sql
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;
PRAGMA busy_timeout = 5000;
```

## Миграции

Директория `internal/db/migrations/`:

- `0001_init.sql` — создание таблиц `proxies`, `sources`, `settings`.
- `0002_...` — последующие изменения.

Таблица `schema_migrations (version INTEGER PRIMARY KEY, applied_at DATETIME)`.

Применение: при старте, в транзакции, последовательно.

## CRUD

### Вставка

```go
// INSERT OR IGNORE для дедупликации
INSERT OR IGNORE INTO proxies (host, port, protocol, country, source_id)
VALUES (?, ?, ?, ?, ?)
```

Возврат: `LastInsertId` — если `0`, значит дубликат.

### Обновление теста

```go
UPDATE proxies
SET latency_ms = ?, download_mbps = ?, last_checked = ?, is_working = ?
WHERE id = ?
```

### Выборка с фильтром

Использовать динамический SQL с `?` placeholder. Пример:

```go
query := "SELECT ... FROM proxies WHERE 1=1"
args := []any{}
if f.OnlyWorking != nil && *f.OnlyWorking {
    query += " AND is_working = 1"
}
if f.Protocol != nil {
    query += " AND protocol = ?"
    args = append(args, *f.Protocol)
}
// ...
```

Сортировка: валидировать `sortBy` по белому списку, иначе SQL-инъекция.

### Удаление

```go
DELETE FROM proxies WHERE id IN (?, ?, ...)
```

## Экспорт

### TXT

```
protocol://host:port\n
```

### CSV

Использовать `encoding/csv`. Заголовки:
`host,port,protocol,latency_ms,download_mbps,country`.

### JSON

`json.MarshalIndent` с массивом объектов. Поля в camelCase.

## Методы StorageService

- `InsertProxies(proxies []ParsedProxy, sourceID *int64) (added int, err error)`
- `UpdateTestResult(r TestResult) error`
- `GetProxies(f ProxyFilter) ([]Proxy, error)`
- `GetProxyByID(id int64) (*Proxy, error)`
- `DeleteProxies(ids []int64) error`
- `ExportToTXT(ids []int64, path string) error`
- `ExportToCSV(ids []int64, path string) error`
- `ExportToJSON(ids []int64, path string) error`
- `GetSetting(key string) (string, error)`
- `SetSetting(key, value string) error`

## Транзакции

- Массовая вставка: одна транзакция.
- Массовое обновление после теста: одна транзакция.
- Использовать `tx.Prepare` для повторяющихся операций.

## Проверка

- [ ] Вставка 1000 прокси < 500 мс.
- [ ] Фильтр «только рабочие» + сортировка по latency < 50 мс.
- [ ] Экспорт 1000 прокси в TXT — валидный файл.
- [ ] Повторная вставка тех же — `added = 0`.

## Частые ошибки

- **SQL injection в sortBy:** всегда валидировать по белому списку.
- **`database is locked`:** использовать WAL и `busy_timeout`.
- **Утечка rows:** `defer rows.Close()`.
