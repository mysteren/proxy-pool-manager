---
name: proxy-tester
description: Тестирование прокси на latency (TCP-connect) и скорость (download через прокси). Конкурентное выполнение с семафором, отмена через context, эмиссия прогресса. Используй в Milestone 3 и 5.
---

# Proxy Tester

## Когда использовать

- Реализация latency-теста.
- Реализация speed-теста.
- Массовое тестирование пула.
- Повторный тест выбранных/нерабочих.

## Latency (TCP-connect)

### Алгоритм

1. Взять `host:port` из прокси.
2. `net.DialTimeout("tcp", addr, timeout)`.
3. Замерить время до успешного connect.
4. При успехе: `latencyMs = int(elapsed.Milliseconds())`, `isWorking = true`.
5. При ошибке: `latencyMs = nil`, `isWorking = false`.
6. Обновить запись в БД.
7. Эмитить `test:progress` с текущим счётчиком.

### Таймаут

По умолчанию 5 секунд. Настраивается в `settings.latency_timeout_ms`.

### Конкурентность

- Семафор на `test_concurrency` (default 50).
- Батчи по 50, между батчами пауза 1 сек.

### Отмена

- Создавать `ctx, cancel := context.WithCancel(parent)` на массовый тест.
- Сохранять `cancel` в TesterService.
- В цикле проверять `ctx.Err()`.
- При отмене — эмитить `test:completed{cancelled: true}`.

## Speed (download)

### Алгоритм

1. Пропускать прокси без успешного latency-теста.
2. Создать HTTP-клиент с транспортом через прокси:
   - `http://` / `https://` — `http.Transport{Proxy: http.ProxyURL(...)}`.
   - `socks5://` — `golang.org/x/net/proxy.SOCKS5` + кастомный DialContext.
3. GET на `https://speed.cloudflare.com/__down?bytes=N`.
4. Читать тело, замерять время.
5. `mbps = float64(bytes) * 8 / elapsed.Seconds() / 1e6`.
6. Обновить `download_mbps` в БД.

### Таймаут

30 секунд. При превышении — `download_mbps = nil`.

### Конкурентность

Семафор на 10 (speed-тесты тяжелее latency).

## Структуры

```go
type TestResult struct {
    ProxyID      int64
    LatencyMs    *int
    DownloadMbps *float64
    IsWorking    bool
    Error        string
}

type TestProgress struct {
    Total     int
    Completed int
    Current   string // host:port
}
```

## Методы TesterService

- `TestLatency(proxyID int64) (TestResult, error)`
- `TestLatencyBatch(proxyIDs []int64) error`
- `TestSpeed(proxyID int64) (TestResult, error)`
- `TestSpeedBatch(proxyIDs []int64) error`
- `Cancel()` — вызывает сохранённый `cancel`.

## События

- `test:progress` — `{total, completed, current}` — каждые 100 мс (throttle).
- `test:completed` — `{cancelled bool, tested int, working int}`.

## Проверка

- [ ] Тест 100 прокси из Proxifly: ~20–50 рабочих, latency 50–2000 мс.
- [ ] Отмена на середине: горутины завершены, событие с `cancelled: true`.
- [ ] Speed-тест 10 рабочих: Mbps в диапазоне 0.1–100.

## Частые ошибки

- **Утечка горутин:** не забывать `sem.Release(1)` и `wg.Done()` в `defer`.
- **Deadlock:** не блокировать отправку в канал без читателя.
- **SOCKS5:** `x/net/proxy` требует `context` — использовать `DialContext` через обёртку.
