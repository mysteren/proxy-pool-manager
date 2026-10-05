---
name: proxy-storage
description: Работа с SQLite: схема, миграции, CRUD, фильтрация, серверная пагинация, экспорт в TXT/CSV/JSON. Используй везде, где нужны данные прокси.
---

# Proxy Storage

## Когда использовать

- Реализация CRUD над прокси и источниками.
- Фильтрация, сортировка и пагинация пула.
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

Путь к БД — кросс-платформенно через `github.com/adrg/xdg`:

```go
import "github.com/adrg/xdg"

dir := filepath.Join(xdg.DataHome, "proxy-pool-manager")
dbPath := filepath.Join(dir, "data.db")
```

`os.UserConfigDir()` **не использовать** — на Linux это `~/.config`, другой путь.

Прагмы при открытии:

```sql
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;
PRAGMA busy_timeout = 5000;
```

Запись сериализовать: `db.SetMaxOpenConns(1)` для пишущего соединения
(или писать из одного воркера) — иначе при массовом обновлении результатов
теста возможны `database is locked`.

## Миграции

Директория `internal/db/migrations/`, файлы встраиваются в бинарник:

```go
//go:embed migrations/*.sql
var migrationsFS embed.FS
```

- `0001_init.sql` — создание таблиц `proxies`, `sources`, `settings`.
- `0002_...` — последующие изменения.

Таблица `schema_migrations (version INTEGER PRIMARY KEY, applied_at DATETIME)`.
Применение: при старте, в транзакции, последовательно.

## CRUD

### Вставка

```go
// INSERT OR IGNORE для дедупликации по (host, port, protocol)
res, err := stmt.Exec(host, port, protocol, country, sourceID)
n, _ := res.RowsAffected()
// n == 0 → дубликат. НЕ использовать LastInsertId == 0:
// после игнорируемой вставки он может вернуть id предыдущей строки.
```

Dedup-ключ — `protocol://host:port`. Схема: `UNIQUE(host, port, protocol)`,
поэтому `http://1.2.3.4:8080` и `socks5://1.2.3.4:8080` — разные строки.

### Обновление теста

```sql
UPDATE proxies
SET latency_ms = ?, download_mbps = ?, last_checked = ?, is_working = ?
WHERE id = ?
```

При ошибке проверки: `latency_ms = NULL`, `download_mbps = NULL`,
`is_working = 0` (старое значение перезаписывается).

### Выборка с фильтром (серверная пагинация)

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
Пагинация — `LIMIT ? OFFSET ?`. Общее число строк определять отдельным
запросом `CountProxies(f)` (тот же WHERE, но `SELECT COUNT(*)`), а не
вычислять по длине страницы.

### Удаление

```sql
DELETE FROM proxies WHERE id IN (?, ?, ...)
```

## Источники

- Удаление источника: `source_id` у прокси обнуляется
  (`ON DELETE SET NULL`) — прокси **остаются в пуле**. Не каскадировать.
- `proxy_count` в БД **не хранится**: `Source.ProxyCount` считается
  через `COUNT(*)` по `source_id` (иначе счётчик расходится при удалении прокси).

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

Экспорт вызывается после диалога сохранения файла (`app.Dialog.SaveFile()`).

## Методы StorageService

- `InsertProxies(proxies []ParsedProxy, sourceID *int64) (added int, err error)`
- `UpdateTestResult(r TestResult) error`
- `GetProxies(f ProxyFilter) ([]Proxy, error)`
- `CountProxies(f ProxyFilter) (int, error)`
- `GetProxyByID(id int64) (*Proxy, error)`
- `DeleteProxies(ids []int64) error`
- `ListSources() ([]Source, error)`
- `DeleteSource(id int64) error`
- `ExportToTXT(ids []int64, path string) error`
- `ExportToCSV(ids []int64, path string) error`
- `ExportToJSON(ids []int64, path string) error`
- `GetSetting(key string) (string, error)` — при отсутствии ключа вернуть
  значение по умолчанию из `DATA_MODEL.md`.
- `SetSetting(key, value string) error`

## Транзакции

- Массовая вставка: одна транзакция.
- Массовое обновление после проверки: одна транзакция.
- Использовать `tx.Prepare` для повторяющихся операций.

## Проверка

- [ ] Вставка 1000 прокси < 500 мс.
- [ ] Фильтр «только рабочие» + сортировка по latency + пагинация < 50 мс.
- [ ] Экспорт 1000 прокси в TXT — валидный файл.
- [ ] Повторная вставка тех же — `added = 0`.
- [ ] `http://h:p` и `socks5://h:p` с одинаковым `h:p` сохраняются как две строки.

## Частые ошибки

- **SQL injection в sortBy:** всегда валидировать по белому списку.
- **`database is locked`:** WAL + `busy_timeout`, сериализовать запись.
- **Утечка rows:** `defer rows.Close()`.
- **`LastInsertId` для детекта дубликата:** ненадёжно — использовать `RowsAffected()`.
