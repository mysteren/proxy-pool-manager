---
name: proxy-source-fetcher
description: Загрузка списков прокси из URL, локальных файлов и ручного ввода. Парсинг форматов ip:port, protocol://ip:port, JSON (Proxifly). Дедупликация. Используй при реализации Milestone 2.
---

# Proxy Source Fetcher

## Когда использовать

- Добавление источника по URL.
- Загрузка из локального файла.
- Ручной ввод списка.
- Парсинг Proxifly JSON.

## Форматы

### Текстовый (`ip:port`)

```
192.168.1.1:8080
10.0.0.1:1080
```

Протокол по умолчанию — `http`.

### Текстовый (`protocol://ip:port`)

```
http://192.168.1.1:8080
socks5://10.0.0.1:1080
https://203.0.113.5:3128
```

### JSON (Proxifly)

```json
{
  "ip": "192.168.1.1",
  "port": 8080,
  "protocols": ["http", "https"],
  "country": "US",
  "anonymity": "elite"
}
```

Файлы могут быть массивом таких объектов или построчным JSON (JSONL).

## Алгоритм

### 1. Определение типа источника

- URL (начинается с `http://` или `https://`) → HTTP GET.
- Путь к файлу → чтение с диска.
- Многострочный текст → парсинг как есть.

### 2. Загрузка (HTTP)

- `http.Client{Timeout: 15 * time.Second}`.
- Следовать редиректам (по умолчанию).
- Проверять `Content-Type` для определения JSON vs текст.
- Ошибка сети → вернуть ошибку с контекстом.

### 3. Парсинг

- **Текст:**
  - Разбить по строкам.
  - Пропускать пустые строки и комментарии (`#`, `//`).
  - Для каждой строки: если содержит `://` — извлечь протокол и `host:port`,
    иначе — `http` + `host:port`.
  - Валидация: `host` непустой, `port` в диапазоне 1–65535.
- **JSON:**
  - Попробовать `json.Unmarshal` в массив.
  - Если ошибка — попробовать JSONL (построчно).
  - Для каждого объекта взять `ip`, `port`, первый из `protocols`, `country`.

### 4. Дедупликация

- In-memory: `map[string]struct{}` по ключу `host:port`.
- В БД: `INSERT OR IGNORE` с UNIQUE constraint на `(host, port)`.

### 5. Сохранение

- Транзакция на весь батч.
- Prepared statement для вставки.
- Обновить `sources.last_fetched`, `sources.proxy_count`.

### 6. Эмиссия события

После успешной загрузки — `source:fetched` с `{sourceId, count}`.

## Структуры

```go
type ParsedProxy struct {
    Host     string
    Port     int
    Protocol string
    Country  *string
}

type FetchResult struct {
    SourceID int64
    Fetched  int
    Added    int
    Skipped  int
    Errors   []string
}
```

## Методы SourceService

- `FetchFromURL(sourceID int64, url string) (FetchResult, error)`
- `FetchFromFile(sourceID int64, path string) (FetchResult, error)`
- `AddManual(text string) (FetchResult, error)`
- `ParseText(content string) ([]ParsedProxy, error)`
- `ParseJSON(content []byte) ([]ParsedProxy, error)`

## Проверка

- [ ] Proxifly URL: `https://raw.githubusercontent.com/proxifly/free-proxy-list/main/proxies/protocols/http/data.txt` — 100+ прокси в БД.
- [ ] Ручной ввод 5 строк — 5 прокси в БД.
- [ ] Повторная загрузка того же источника — `added = 0`, `skipped = N`.

## Частые ошибки

- **URL с `tree/main` в GitHub** — это HTML-страница, а не raw-файл.
  Использовать `raw.githubusercontent.com`.
- **Прокси с портом 0** — отбрасывать.
- **Дубликаты с разным регистром host** — нормализовать к нижнему регистру.
