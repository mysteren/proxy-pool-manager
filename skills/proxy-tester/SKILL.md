---
name: proxy-tester
description: Проверка прокси на latency (TCP-connect) и работоспособность (HTTP-запрос через прокси), тест скорости. Конкурентность с семафором, отмена через context, эмиссия прогресса. Используй в Milestone 3 и 5.
---

# Proxy Tester

## Когда использовать

- Реализация TCP-connect (latency).
- Реализация подтверждения работоспособности через HTTP.
- Реализация speed-теста.
- Массовая проверка пула, повторная проверка выбранных/нерабочих.

## Принцип

**Рабочий прокси ≠ открытый порт.** TCP-connect может «пройти» для
постороннего сервиса. Поэтому рабочим прокси считается только тот, через
который успешно прошёл лёгкий HTTP-запрос. Честность важнее скорости.

Поддерживаются протоколы `http` и `socks5` (`https` нормализуется в `http`,
`socks4` отбрасывается при парсинге).

## Latency (TCP-connect)

### Алгоритм

1. Взять `host:port` из прокси.
2. `net.Dialer{Timeout: timeout}.DialContext(ctx, "tcp", addr)`.
   Использовать именно `DialContext` (а не `net.DialTimeout`), чтобы отмена
   массовой проверки прерывала висящие подключения.
3. Замерить время до успешного connect → `latencyMs`.
4. При ошибке/таймауте → `is_working = false`, `latency_ms = nil`.

### Таймаут

По умолчанию 5 секунд. Настраивается в `settings.latency_timeout_ms`.

## Подтверждение работоспособности (HTTP через прокси)

Выполняется после успешного TCP-connect.

1. Если `settings.validate_via_http = false` — пропустить (менее надёжно).
2. Создать HTTP-клиент с транспортом через прокси:
   - `http` — `http.Transport{Proxy: http.ProxyURL(&url.URL{Scheme: "http", Host: addr})}`.
   - `socks5` — `golang.org/x/net/proxy.SOCKS5("tcp", addr, nil, forwardDialer)`
     + кастомный `DialContext`.
3. GET на `settings.http_validation_url` (по умолчанию
   `https://api.ipify.org?format=json`) с `ctx` и таймаутом
   `latency_timeout_ms`.
4. Успешный ответ (2xx, непустое тело) → `is_working = true`.
5. Ошибка/таймаут → `is_working = false`, `latency_ms = nil`.
6. Обновить запись в БД, эмитить `test:progress`.

Цель — HTTPS, чтобы проверить и туннель CONNECT.

## Скорость (download)

Запускается **только для прокси, прошедших подтверждение работоспособности**.

1. GET на `https://speed.cloudflare.com/__down?bytes=N`
   (`settings.speed_download_bytes`) через прокси.
2. Читать тело, замерять время, считать
   `mbps = float64(bytes) * 8 / elapsed.Seconds() / 1e6`.
3. Таймаут 30 секунд. При ошибке → `download_mbps = nil`.

## Конкурентность

- Число одновременных проверок — настройка `test_concurrency` (по умолчанию 50).
  Пользователь снижает её, чтобы уменьшить нагрузку на сеть/DNS.
- Реализация: `golang.org/x/sync/semaphore` или пул воркеров.
- Скорость (download) — отдельный меньший лимит (по умолчанию 10).

## Отмена

- `ctx, cancel := context.WithCancel(parent)` на каждый массовый тест.
- `cancel` хранить в `TesterService` **под мьютексом** — доступ из разных
  горутин (запуск теста, кнопка «Отмена», завершение).
- В цикле проверять `ctx.Err()`; `DialContext` и HTTP-запросы принимают `ctx`.
- Все горутины обязаны делать `sem.Release(1)` и `wg.Done()` в `defer`.
- При отмене — событие `test:completed{cancelled: true}`.

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

- `TestLatency(proxyID int64) (TestResult, error)` — TCP + HTTP-подтверждение.
- `TestLatencyBatch(proxyIDs []int64) error`
- `TestSpeed(proxyID int64) (TestResult, error)`
- `TestSpeedBatch(proxyIDs []int64) error`
- `Cancel()` — вызывает сохранённый `cancel` под мьютексом.

## События

Регистрировать через `application.RegisterEvent[T]` (типизированный TS-API).

- `test:progress` — `{total, completed, current}` — throttle ~100 мс.
- `test:completed` — `{cancelled bool, tested int, working int}`.

## Проверка

- [ ] Нерабочий прокси (открытый порт без прокси) помечается нерабочим.
- [ ] Проверка 100 прокси: рабочие имеют latency 50–2000 мс.
- [ ] Отмена на середине: горутины завершены, событие с `cancelled: true`.
- [ ] Speed-тест 10 рабочих: Mbps в диапазоне 0.1–100.

## Частые ошибки

- **Утечка горутин:** `sem.Release(1)` и `wg.Done()` в `defer`.
- **Deadlock:** не блокировать отправку в канал без читателя.
- **`net.DialTimeout` вместо `DialContext`:** отмена не сработает.
- **SOCKS5:** `x/net/proxy` требует `DialContext`-обёртку.
- **Ложные «рабочие»:** не пропускать HTTP-подтверждение.
