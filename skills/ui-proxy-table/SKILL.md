---
name: ui-proxy-table
description: Таблица пула прокси с виртуализацией, сортировкой, множественным выбором, копированием в буфер, повторным тестом и контекстным меню. Используй в Milestone 3–4.
---

# UI Proxy Table

## Когда использовать

- Реализация основного экрана «Прокси».
- Добавление сортировки, фильтров, выбора.
- Копирование в буфер, экспорт.

## Библиотеки

- `@tanstack/react-table` — логика таблицы.
- `@tanstack/react-virtual` — виртуализация строк.
- `lucide-react` — иконки.
- `sonner` — тосты.
- shadcn/ui: `Table`, `Checkbox`, `DropdownMenu`, `Button`, `Input`, `Badge`.

## Колонки

| Ключ          | Отображение                                                   |
|---------------|---------------------------------------------------------------|
| `select`      | Checkbox                                                      |
| `host`        | Текст, `font-mono`                                            |
| `port`        | Число                                                         |
| `protocol`    | Badge с цветом по протоколу                                   |
| `latency`     | Badge: зелёный < 150, жёлтый 150–500, красный > 500, серый `—`|
| `download`    | `X.X Mbps` или `—`                                            |
| `lastChecked` | Относительное время (`date-fns/formatDistanceToNow`)          |
| `actions`     | DropdownMenu (Копировать, Тестировать, Удалить)               |

## Виртуализация

Использовать `useVirtualizer` из TanStack Virtual:

```ts
const rowVirtualizer = useVirtualizer({
  count: rows.length,
  getScrollElement: () => tableContainerRef.current,
  estimateSize: () => 40,
  overscan: 10,
})
```

Рендерить только `rowVirtualizer.getVirtualItems()`.

## Выбор строк

- Состояние `rowSelection` из TanStack Table.
- Shift-клик — диапазон, Ctrl/Cmd-клик — точечно.
- Шапка: checkbox «выбрать все видимые».

## Копирование в буфер

```ts
const text = selectedProxies
  .map(p => formatProxy(p, copyFormat))
  .join('\n')
await navigator.clipboard.writeText(text)
toast.success(`Скопировано: ${selectedProxies.length}`)
```

`copyFormat` из настроек: `uri` (`protocol://host:port`) или `host:port`.

## Повторный тест

- Кнопка «Тестировать выбранные» — вызывает `TestLatencyBatch(ids)`.
- Кнопка «Тестировать нерабочие» — фильтрует по `isWorking = false`.
- Прогресс-бар над таблицей (виден при активном тесте).
- Кнопка «Отмена» — вызывает `Cancel()`.

## Прогресс-бар

- Подписка на `test:progress`.
- Обновлять состояние через Zustand или локально.
- По завершении `test:completed` — перезагрузить список.

## Контекстное меню

Правая кнопка на строке:

- Копировать (`Ctrl+C`).
- Тестировать (`Ctrl+R`).
- Удалить (`Delete`).

## Горячие клавиши

Регистрировать через `useEffect` + `addEventListener('keydown')`:

- `Ctrl/Cmd + R` — тестировать выбранные.
- `Ctrl/Cmd + Shift + R` — тестировать нерабочие.
- `Ctrl/Cmd + C` — копировать.
- `Ctrl/Cmd + A` — выделить все видимые.
- `Delete` — удалить выбранные (с подтверждением, если > 10).

## Фильтры (Toolbar)

- Поле поиска по host (debounce 300 мс).
- Checkbox «Только рабочие».
- Dropdown «Протокол»: все / http / https / socks4 / socks5.
- Dropdown «Макс. latency»: все / <100 / <500 / <1000.

## Проверка

- [ ] Таблица с 10 000 строк: скролл без фризов.
- [ ] Сортировка по latency: 1 клик — возрастание, 2 — убывание.
- [ ] Копирование 100 выбранных: 100 строк в буфере.
- [ ] Отмена теста: прогресс-бар исчезает, состояние не сломано.

## Частые ошибки

- **Мерцание при обновлении:** использовать `keepPreviousData` в React Query
  (если используется) или мемоизировать строки.
- **Лаги при 10k строк:** не рендерить все — только виртуализированные.
- **Копирование пустого списка:** блокировать кнопку, если выбор пуст.
