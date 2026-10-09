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
    protocol        TEXT    NOT NULL DEFAULT 'socks5',
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

На больших пулах (десятки–сотни тысяч строк) выборка и сортировка заметно
ускоряются составными индексами с `protocol` в начале (миграция `0005`):

- счётчики: `(protocol, is_working)`, `(protocol, last_checked)` — покрывающие,
  `COUNT(*)` по фильтру статуса идёт без полного скана;
- сортировка: `(protocol, (latency_ms IS NULL), latency_ms, id)` и аналогичный
  по `download_mbps`, плюс `(protocol, host, id)`, `(protocol, port, id)` —
  без временного b-tree на каждый запрос.

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

Удаление источника: нерабочие/непроверенные прокси источника удаляются,
а проверенные рабочие — остаются в общем пуле (`source_id = NULL`),
чтобы не терять рабочие прокси. То же для `mtproto_proxies`.

`proxy_count` в БД не хранится: количество прокси источника вычисляется
через `COUNT(*)` по `source_id` в обеих таблицах (`proxies` + `mtproto_proxies`).

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
| `http_validation_url`     | `https://speed.cloudflare.com/meta`  |
| `speed_test_enabled`      | `true`                               |
| `test_random_order`       | `true`                               |
| `geo_consensus`           | `true`                               |

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
socks5://10.0.0.1:1080
socks5://10.0.0.2:1080
```

### CSV

```
host,port,protocol,latency_ms,download_mbps,country
10.0.0.1,1080,socks5,85,,DE
10.0.0.2,1080,socks5,120,5.4,US
```

### JSON

```json
[
  {
    "host": "10.0.0.1",
    "port": 1080,
    "protocol": "socks5",
    "latencyMs": 120,
    "downloadMbps": 5.4,
    "country": "US"
  }
]
```

### Таблица `mtproto_proxies`

Telegram-прокси (MTProto / tg-socks). Заполняется автоматически при загрузке
источника (строки `tg://…`, `t.me/…`, `https://t.me/…`, `host:port:secret`).

Ключевые поля: `host, port, secret, tg_type` (`mtproto`|`socks`), `is_working`,
`last_checked`, `source_id` (как у `proxies`). Уникальность — `(host, port, secret)`.

Метрики качества (после проверки):

- `ping_ms` — средний RTT успешных рукопожатий;
- `jitter_ms` — разброс пинга (стабильность);
- `successes` / `attempts` — надёжность (по 3 попыткам);
- `score = successes/attempts*1000 − ping_ms − jitter_ms` (больше — лучше).

В UI сортировка по `score` (по умолчанию), а также по пингу/джиттеру/надёжности.

Гео (миграция `0006`): `country`, `city`, `latitude`, `longitude`, `server_ip`.
Через MTProto HTTP-запрос провести нельзя, поэтому гео определяется **по IP
сервера**: host резолвится в IP, затем IP спрашивается у нескольких гео-сервисов
(`ipwho.is`, `ip-api`, `ipinfo`) с консенсусом. Запросы ограничены по частоте
(бесплатные лимиты), выполняются по кнопке «Определить гео» в фоне с прогрессом.

## Миграции

Миграции хранятся в `internal/db/migrations/` в виде пронумерованных
SQL-файлов: `0001_init.sql`, `0002_add_country.sql` и т.д.
Файлы встраиваются в бинарник через `//go:embed migrations/*.sql`.
Применение — при старте приложения, через таблицу `schema_migrations`.
