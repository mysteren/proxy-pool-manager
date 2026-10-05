# Модель данных

## Схема SQLite

Файл БД: `~/.local/share/proxy-pool-manager/data.db` (Linux).
Путь определяется кросс-платформенно через `github.com/adrg/xdg`
(`xdg.DataHome()`): macOS — `~/Library/Application Support/proxy-pool-manager/data.db`,
Windows — `%LOCALAPPDATA%\proxy-pool-manager\data.db`.
`os.UserConfigDir()` НЕ использовать — на Linux это `~/.config` (другой путь).

### Таблица `proxies`

```sql
CREATE TABLE proxies (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    host            TEXT    NOT NULL,
    port            INTEGER NOT NULL,
    protocol        TEXT    NOT NULL DEFAULT 'http',
    country         TEXT,
    latency_ms      INTEGER,
    download_mbps   REAL,
    last_checked    DATETIME,
    is_working      BOOLEAN NOT NULL DEFAULT 0,
    source_id       INTEGER REFERENCES sources(id) ON DELETE SET NULL,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(host, port, protocol)
);

CREATE INDEX idx_proxies_is_working ON proxies(is_working);
CREATE INDEX idx_proxies_latency    ON proxies(latency_ms);
CREATE INDEX idx_proxies_protocol   ON proxies(protocol);
CREATE INDEX idx_proxies_source     ON proxies(source_id);
```

### Таблица `sources`

```sql
CREATE TABLE sources (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    name          TEXT    NOT NULL,
    url           TEXT,
    file_path     TEXT,
    last_fetched  DATETIME,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (url IS NOT NULL OR file_path IS NOT NULL)
);
```

Удаление источника: `source_id` у прокси обнуляется (`ON DELETE SET NULL`),
прокси остаются в пуле — теряется только привязка к источнику.

`proxy_count` в БД не хранится: количество прокси источника вычисляется
через `COUNT(*)` по `source_id` (иначе счётчик расходится при удалении прокси).

### Таблица `settings`

```sql
CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
```

Начальные настройки:

| key                       | default                              |
|---------------------------|--------------------------------------|
| `theme`                   | `system`                             |
| `latency_timeout_ms`      | `5000`                               |
| `speed_download_bytes`    | `1000000`                            |
| `test_concurrency`        | `50`                                 |
| `copy_format`             | `uri`                                |
| `validate_via_http`       | `true`                               |
| `http_validation_url`     | `https://api.ipify.org?format=json`  |

## Go-модели

### `Proxy`

Поля, соответствующие строке таблицы `proxies`. Теги JSON — в camelCase
для передачи на фронтенд.

```go
type Proxy struct {
    ID           int64      `json:"id"`
    Host         string     `json:"host"`
    Port         int        `json:"port"`
    Protocol     string     `json:"protocol"`
    Country      *string    `json:"country,omitempty"`
    LatencyMs    *int       `json:"latencyMs,omitempty"`
    DownloadMbps *float64   `json:"downloadMbps,omitempty"`
    LastChecked  *time.Time `json:"lastChecked,omitempty"`
    IsWorking    bool       `json:"isWorking"`
    SourceID     *int64     `json:"sourceId,omitempty"`
}
```

### `Source`

```go
// ProxyCount не хранится в БД, а вычисляется через COUNT(*) по source_id.
type Source struct {
    ID          int64      `json:"id"`
    Name        string     `json:"name"`
    URL         *string    `json:"url,omitempty"`
    FilePath    *string    `json:"filePath,omitempty"`
    LastFetched *time.Time `json:"lastFetched,omitempty"`
    ProxyCount  int        `json:"proxyCount"`
}
```

### `ProxyFilter`

```go
type ProxyFilter struct {
    OnlyWorking *bool    `json:"onlyWorking,omitempty"`
    Protocol    *string  `json:"protocol,omitempty"`
    MaxLatency  *int     `json:"maxLatency,omitempty"`
    Country     *string  `json:"country,omitempty"`
    Search      *string  `json:"search,omitempty"` // поиск по host
    SortBy      string   `json:"sortBy"`           // latency|host|lastChecked
    SortDir     string   `json:"sortDir"`          // asc|desc
    // Пагинация серверная; общее число строк — отдельным запросом CountProxies.
    Limit       int      `json:"limit"`
    Offset      int      `json:"offset"`
}
```

## Форматы экспорта

### TXT

```
http://192.168.1.1:8080
socks5://10.0.0.1:1080
https://203.0.113.5:3128
```

### CSV

```
host,port,protocol,latency_ms,download_mbps,country
192.168.1.1,8080,http,120,5.4,US
10.0.0.1,1080,socks5,85,,DE
```

### JSON

```json
[
  {
    "host": "192.168.1.1",
    "port": 8080,
    "protocol": "http",
    "latencyMs": 120,
    "downloadMbps": 5.4,
    "country": "US"
  }
]
```

## Миграции

Миграции хранятся в `internal/db/migrations/` в виде пронумерованных
SQL-файлов: `0001_init.sql`, `0002_add_country.sql` и т.д.
Файлы встраиваются в бинарник через `//go:embed migrations/*.sql`.
Применение — при старте приложения, через таблицу `schema_migrations`.
