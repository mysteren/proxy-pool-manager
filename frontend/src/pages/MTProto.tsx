import { useCallback, useEffect, useMemo, useState } from "react";
import { Copy, Download, Eraser, Globe, Loader2, Play, RefreshCw, Trash2 } from "lucide-react";

import { GeoProgressBar } from "@/components/GeoProgressBar";
import { TestProgressBar } from "@/components/TestProgressBar";
import { Badge } from "@/components/ui/badge";
import { Hint } from "@/components/ui/hint";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { formatRelative } from "@/lib/time";
import { formatLatency, latencyVariant } from "@/lib/proxy";
import { countryFlag, countryName, formatDistance, haversineKm } from "@/lib/geo";
import { toast } from "@/stores/toastStore";
import { useTestStore } from "@/stores/testStore";
import { useGeoStore } from "@/stores/geoStore";
import { MTProtoService, TesterService, type MyLocation } from "../../bindings/proxy-pool-manager/internal/services";
import { MTProtoFilter, MTProtoProxy } from "../../bindings/proxy-pool-manager/internal/models";

type Status = "all" | "working" | "broken" | "unchecked";
type ConfirmAction = {
  kind: "delete" | "clear" | "deleteBroken" | "deleteUnchecked" | "deleteAll";
  label: string;
};

const pageSizeOptions = [25, 50, 100, 200];
const statusOptions: { value: Status; label: string }[] = [
  { value: "all", label: "Все" },
  { value: "working", label: "Рабочие" },
  { value: "broken", label: "Нерабочие" },
  { value: "unchecked", label: "Без статуса" },
];

export function MTProtoPage() {
  const [rows, setRows] = useState<MTProtoProxy[]>([]);
  const [total, setTotal] = useState(0);
  const [workingTotal, setWorkingTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState(50);
  const [sortBy, setSortBy] = useState("score");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [status, setStatus] = useState<Status>("all");
  const [noSource, setNoSource] = useState(false);
  const [typeFilter, setTypeFilter] = useState("");
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [anchorIndex, setAnchorIndex] = useState<number | null>(null);
  const [checking, setChecking] = useState<Set<number>>(new Set());
  const [confirm, setConfirm] = useState<ConfirmAction | null>(null);
  const [exportFormat, setExportFormat] = useState("txt");
  const [myLocation, setMyLocation] = useState<MyLocation | null>(null);

  const running = useTestStore((s) => s.running);
  const refreshToken = useTestStore((s) => s.refreshToken);
  const startTest = useTestStore((s) => s.start);
  const resetTest = useTestStore((s) => s.reset);

  const geoRunning = useGeoStore((s) => s.running);
  const startGeo = useGeoStore((s) => s.start);
  const resetGeo = useGeoStore((s) => s.reset);

  useEffect(() => {
    TesterService.GetMyLocation()
      .then((loc) => setMyLocation(loc))
      .catch(() => setMyLocation(null));
  }, []);

  useEffect(() => {
    const id = setTimeout(() => setDebouncedSearch(search), 300);
    return () => clearTimeout(id);
  }, [search]);

  useEffect(() => {
    setPage(0);
  }, [debouncedSearch, status, noSource, typeFilter, pageSize]);

  const buildFilter = useCallback(
    (offset: number): MTProtoFilter => {
      const filter: MTProtoFilter = {
        onlyWorking: null,
        unchecked: null,
        noSource: noSource ? true : null,
        type: typeFilter || null,
        search: debouncedSearch || null,
        sortBy,
        sortDir,
        limit: pageSize,
        offset,
      };
      if (status === "working") {
        filter.onlyWorking = true;
      } else if (status === "broken") {
        filter.onlyWorking = false;
        filter.unchecked = false;
      } else if (status === "unchecked") {
        filter.unchecked = true;
      }
      return filter;
    },
    [status, noSource, typeFilter, debouncedSearch, sortBy, sortDir, pageSize],
  );

  const load = useCallback(
    async (silent = false, withCounts = true) => {
      if (!silent) setLoading(true);
      try {
        const filter = buildFilter(page * pageSize);
        const list = await MTProtoService.GetProxies(filter);
        setRows(list ?? []);
        // Счётчики — самые дорогие запросы (полный COUNT(*)), поэтому во время
        // фонового обновления их пересчитываем реже.
        if (withCounts) {
          const [count, workingCount] = await Promise.all([
            MTProtoService.Count(filter),
            MTProtoService.Count({ ...filter, onlyWorking: true, unchecked: null, limit: 0, offset: 0 }),
          ]);
          setTotal(count);
          setWorkingTotal(workingCount);
        }
      } catch (err) {
        if (!silent) toast.error(`Не удалось загрузить MTProto: ${String(err)}`);
      } finally {
        if (!silent) setLoading(false);
      }
    },
    [buildFilter, page, pageSize],
  );

  useEffect(() => {
    void load();
  }, [load, refreshToken]);

  // Статусы сохраняются в БД по мере проверки каждого прокси, а не в конце
  // прогона. Поэтому во время массовой проверки периодически подтягиваем только
  // текущую страницу (один запрос с LIMIT), а тяжёлые COUNT(*) — раз в несколько
  // секунд, чтобы не перегружать единственное соединение с БД.
  useEffect(() => {
    if (!running) return;
    let tick = 0;
    const id = setInterval(() => {
      tick += 1;
      void load(true, tick % 5 === 0);
    }, 1500);
    return () => clearInterval(id);
  }, [running, load]);

  const pageIds = useMemo(() => rows.map((r) => r.id), [rows]);
  const allOnPageSelected = pageIds.length > 0 && pageIds.every((id) => selected.has(id));
  const pageCount = Math.max(1, Math.ceil(total / pageSize));

  const handleRowClick = (index: number, event: { shiftKey: boolean; ctrlKey: boolean; metaKey: boolean }) => {
    const id = rows[index]?.id;
    if (id == null) {
      return;
    }
    if (event.shiftKey && anchorIndex != null) {
      const [from, to] = anchorIndex <= index ? [anchorIndex, index] : [index, anchorIndex];
      const rangeIds = rows.slice(from, to + 1).map((r) => r.id);
      setSelected((prev) => {
        const next = event.ctrlKey || event.metaKey ? new Set(prev) : new Set<number>();
        rangeIds.forEach((rid) => next.add(rid));
        return next;
      });
      return;
    }
    if (event.ctrlKey || event.metaKey) {
      setSelected((prev) => {
        const next = new Set(prev);
        if (next.has(id)) {
          next.delete(id);
        } else {
          next.add(id);
        }
        return next;
      });
    } else {
      setSelected(new Set([id]));
    }
    setAnchorIndex(index);
  };

  const toggleAllOnPage = () =>
    setSelected((prev) => {
      const next = new Set(prev);
      if (allOnPageSelected) {
        pageIds.forEach((id) => next.delete(id));
      } else {
        pageIds.forEach((id) => next.add(id));
      }
      return next;
    });

  const sortByColumn = (column: string) => {
    if (sortBy === column) {
      setSortDir((dir) => (dir === "asc" ? "desc" : "asc"));
    } else {
      setSortBy(column);
      setSortDir("asc");
    }
  };

  const runBatch = async (action: () => Promise<unknown>, message: string) => {
    startTest();
    try {
      await action();
      toast.info(message);
    } catch (err) {
      resetTest();
      toast.error(String(err));
    }
  };

  const startGeoLookup = async () => {
    startGeo();
    try {
      await MTProtoService.LookupGeo();
      toast.info("Определение гео запущено");
    } catch (err) {
      resetGeo();
      toast.error(String(err));
    }
  };

  const handleTestOne = async (id: number) => {
    setChecking((prev) => new Set(prev).add(id));
    try {
      await MTProtoService.TestProxies([id]);
    } catch (err) {
      toast.error(String(err));
    } finally {
      await load();
      setChecking((prev) => {
        const next = new Set(prev);
        next.delete(id);
        return next;
      });
    }
  };

  const handleCopy = async () => {
    try {
      const count = await MTProtoService.CopyToClipboard([...selected]);
      toast.success(`Скопировано: ${count}`);
    } catch (err) {
      toast.error(String(err));
    }
  };

  const handleExport = async () => {
    try {
      const path = await MTProtoService.PickExportPath(exportFormat);
      if (!path) {
        return;
      }
      const count =
        selected.size > 0
          ? await MTProtoService.ExportByIDs([...selected], exportFormat, path)
          : await MTProtoService.ExportByFilter(buildFilter(0), exportFormat, path);
      toast.success(`Экспортировано: ${count}`);
    } catch (err) {
      toast.error(String(err));
    }
  };

  const doDelete = async () => {
    const ids = [...selected];
    try {
      await MTProtoService.Delete(ids);
      setSelected(new Set());
      setConfirm(null);
      toast.success(`Удалено: ${ids.length}`);
      await load();
    } catch (err) {
      toast.error(String(err));
    }
  };

  const doClear = async () => {
    try {
      const count =
        selected.size > 0
          ? await MTProtoService.ClearStatusByIDs([...selected])
          : await MTProtoService.ClearStatus(buildFilter(0));
      setConfirm(null);
      toast.success(`Статус сброшен: ${count}`);
      await load();
    } catch (err) {
      toast.error(String(err));
    }
  };

  const deleteFilter = (patch: Partial<MTProtoFilter>): MTProtoFilter => ({
    onlyWorking: null,
    unchecked: null,
    noSource: null,
    type: null,
    search: null,
    sortBy: "id",
    sortDir: "asc",
    limit: 0,
    offset: 0,
    ...patch,
  });

  const deleteByFilter = async (filter: MTProtoFilter, message: string) => {
    try {
      const count = await MTProtoService.DeleteByFilter(filter);
      setConfirm(null);
      setSelected(new Set());
      toast.success(`${message}: ${count}`);
      await load();
    } catch (err) {
      toast.error(String(err));
    }
  };

  const doConfirm = async () => {
    if (!confirm) {
      return;
    }
    switch (confirm.kind) {
      case "delete":
        await doDelete();
        break;
      case "clear":
        await doClear();
        break;
      case "deleteBroken":
        await deleteByFilter(deleteFilter({ onlyWorking: false, unchecked: false }), "Удалено нерабочих");
        break;
      case "deleteUnchecked":
        await deleteByFilter(deleteFilter({ unchecked: true }), "Удалено непроверенных");
        break;
      case "deleteAll":
        await deleteByFilter(deleteFilter({}), "Удалено всего");
        break;
    }
  };

  const distanceFor = (p: MTProtoProxy): number | null => {
    if (!myLocation?.latitude || !myLocation?.longitude || p.latitude == null || p.longitude == null) {
      return null;
    }
    return haversineKm(myLocation.latitude, myLocation.longitude, p.latitude, p.longitude);
  };

  const sortIndicator = (column: string) =>
    sortBy === column ? (sortDir === "asc" ? " ↑" : " ↓") : "";

  return (
    <div className="flex h-full flex-col">
      <header className="flex items-center justify-between border-b border-border px-6 py-4">
        <div>
          <h1 className="text-lg font-semibold">MTProto</h1>
          <p className="text-sm text-muted-foreground">
            Telegram-прокси · всего: {total} · рабочих: {workingTotal}
          </p>
        </div>
      </header>

      <div className="flex flex-wrap items-center gap-3 border-b border-border px-6 py-3">
        <Input
          placeholder="Поиск по host"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="w-48"
        />
        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            className="size-4 accent-primary"
            checked={noSource}
            onChange={(e) => setNoSource(e.target.checked)}
          />
          Без источника
        </label>
        <div className="flex rounded-md border border-border p-0.5">
          {statusOptions.map((option) => (
            <button
              key={option.value}
              type="button"
              onClick={() => setStatus(option.value)}
              className={cn(
                "rounded px-2.5 py-1 text-sm transition-colors",
                status === option.value
                  ? "bg-primary text-primary-foreground"
                  : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
              )}
            >
              {option.label}
            </button>
          ))}
        </div>
        <select
          className="h-9 rounded-md border border-input bg-background px-2 text-sm text-foreground"
          value={typeFilter}
          onChange={(e) => setTypeFilter(e.target.value)}
        >
          <option value="">Все типы</option>
          <option value="mtproto">mtproto</option>
          <option value="socks">socks</option>
        </select>
      </div>

      <div className="flex flex-wrap items-center gap-2 border-b border-border px-6 py-3">
        {confirm ? (
          <>
            <span className="text-sm">{confirm.label}</span>
            <Button size="sm" variant={confirm.kind === "clear" ? "default" : "destructive"} onClick={doConfirm}>
              Да
            </Button>
            <Button size="sm" variant="ghost" onClick={() => setConfirm(null)}>
              Отмена
            </Button>
          </>
        ) : (
          <>
            <Hint plain focusable={false} content={<><b>Проверить выбранные.</b> Проверить только выделенные Telegram-прокси.</>}>
              <Button
                size="sm"
                onClick={() => runBatch(() => MTProtoService.TestProxies([...selected]), `Проверка запущена: ${selected.size}`)}
                disabled={running || selected.size === 0}
              >
                <Play className="size-3.5" />
                Проверить выбранные
              </Button>
            </Hint>
            <Hint plain focusable={false} content={<><b>Без статуса.</b> Проверить Telegram-прокси, которые ещё ни разу не проверялись.</>}>
              <Button
                size="sm"
                variant="outline"
                onClick={() => runBatch(() => MTProtoService.TestUnchecked(), "Проверка без статуса запущена")}
                disabled={running}
              >
                Без статуса
              </Button>
            </Hint>
            <Hint plain focusable={false} content={<><b>Нерабочие.</b> Перепроверить прокси, помеченные нерабочими.</>}>
              <Button
                size="sm"
                variant="outline"
                onClick={() => runBatch(() => MTProtoService.TestNonWorking(), "Проверка нерабочих запущена")}
                disabled={running}
              >
                <RefreshCw className="size-3.5" />
                Нерабочие
              </Button>
            </Hint>
            <Hint plain focusable={false} content={<><b>Проверить всё.</b> Проверить все Telegram-прокси. Порядок зависит от настройки «Проверять в случайном порядке».</>}>
              <Button
                size="sm"
                variant="outline"
                onClick={() => runBatch(() => MTProtoService.TestAll(), "Проверка всех запущена")}
                disabled={running}
              >
                Проверить всё
              </Button>
            </Hint>
            <Hint plain focusable={false} content={<><b>Определить гео.</b> Определить страну/город и координаты прокси по IP сервера (через MTProto HTTP-запрос не провести). Идёт в фоне с ограничением частоты запросов к гео-сервисам.</>}>
              <Button
                size="sm"
                variant="outline"
                onClick={startGeoLookup}
                disabled={running || geoRunning}
              >
                <Globe className="size-3.5" />
                Определить гео
              </Button>
            </Hint>
            <Button
              size="sm"
              variant="ghost"
              onClick={() => setConfirm({ kind: "clear", label: selected.size > 0 ? `Сбросить статус у ${selected.size} выбранных?` : "Сбросить статус у всех по фильтру?" })}
            >
              <Eraser className="size-3.5" />
              Очистить статус
            </Button>
            <Button
              size="sm"
              variant="ghost"
              onClick={() => setConfirm({ kind: "deleteUnchecked", label: "Удалить все непроверенные Telegram-прокси?" })}
            >
              <Trash2 className="size-3.5" />
              Непроверенные
            </Button>
            <Button
              size="sm"
              variant="ghost"
              onClick={() => setConfirm({ kind: "deleteBroken", label: "Удалить все нерабочие Telegram-прокси?" })}
            >
              <Trash2 className="size-3.5" />
              Нерабочие
            </Button>
            <Button
              size="sm"
              variant="ghost"
              onClick={() => setConfirm({ kind: "deleteAll", label: "Удалить ВСЕ Telegram-прокси? Источники останутся." })}
            >
              <Trash2 className="size-3.5" />
              Всё
            </Button>

            <div className="ml-auto flex items-center gap-2">
              <Button size="sm" variant="outline" onClick={handleCopy} disabled={selected.size === 0}>
                <Copy className="size-3.5" />
                Копировать
              </Button>
              <select
                className="h-9 rounded-md border border-input bg-transparent px-2 text-sm"
                value={exportFormat}
                onChange={(e) => setExportFormat(e.target.value)}
                aria-label="Формат экспорта"
              >
                <option value="txt">TXT</option>
                <option value="csv">CSV</option>
                <option value="json">JSON</option>
              </select>
              <Button size="sm" variant="outline" onClick={handleExport}>
                <Download className="size-3.5" />
                Экспорт
              </Button>
              <Button
                size="sm"
                variant="outline"
                onClick={() => setConfirm({ kind: "delete", label: `Удалить ${selected.size} выбранных?` })}
                disabled={selected.size === 0}
              >
                <Trash2 className="size-3.5" />
                Удалить
              </Button>
            </div>
          </>
        )}
      </div>

      <TestProgressBar onCancel={() => void MTProtoService.Cancel()} />
      <GeoProgressBar onCancel={() => void MTProtoService.CancelGeo()} />

      <div className="flex-1 overflow-auto">
        <table className="w-full border-collapse text-sm">
          <thead className="sticky top-0 z-10 bg-card">
            <tr className="border-b border-border text-left text-muted-foreground">
              <th className="w-10 px-4 py-2">
                <input
                  type="checkbox"
                  className="size-4 accent-primary"
                  checked={allOnPageSelected}
                  onChange={toggleAllOnPage}
                  aria-label="Выбрать все на странице"
                />
              </th>
              <th className="cursor-pointer px-4 py-2 font-medium" onClick={() => sortByColumn("host")}>
                <Hint content={<><b>Host</b> — адрес MTProto-прокси (IP или домен).</>}>Host</Hint>
                {sortIndicator("host")}
              </th>
              <th className="cursor-pointer px-4 py-2 font-medium" onClick={() => sortByColumn("port")}>
                <Hint content={<><b>Порт</b> — TCP-порт прокси, обычно 443.</>}>Порт</Hint>
                {sortIndicator("port")}
              </th>
              <th className="cursor-pointer px-4 py-2 font-medium" onClick={() => sortByColumn("country")}>
                <Hint content={<><b>Страна</b> — страна сервера прокси, определённая по его IP (через MTProto HTTP-запрос не провести).</>}>Страна</Hint>
                {sortIndicator("country")}
              </th>
              <th className="cursor-pointer px-4 py-2 font-medium" onClick={() => sortByColumn("city")}>
                <Hint content={<><b>Город</b> — город сервера прокси по его IP, если гео-источник его сообщил.</>}>Город</Hint>
                {sortIndicator("city")}
              </th>
              <th className="px-4 py-2 font-medium">
                <Hint content={<><b>Расстояние</b> — расстояние по прямой от вас до сервера прокси (по координатам обоих). Приблизительное. Появляется после «Определить гео».</>}>Расстояние</Hint>
              </th>
              <th className="px-4 py-2 font-medium">
                <Hint content={<><b>Тип</b> — MTProto — собственный прокси Telegram (маскировка FakeTLS/obfuscated); SOCKS — SOCKS5-прокси, поддерживающий MTProto.</>}>Тип</Hint>
              </th>
              <th className="px-4 py-2 font-medium">
                <Hint content={<><b>Секрет</b> — секретный ключ прокси (hex или base64). Входит в ссылку tg://proxy и нужен Telegram для подключения.</>}>Секрет</Hint>
              </th>
              <th className="cursor-pointer px-4 py-2 font-medium" onClick={() => sortByColumn("score")}>
                <Hint content={<><b>Оценка</b> — итоговый балл: надёжность (доля успешных рукопожатий) за вычетом пинга и джиттера. Чем больше — тем лучше; по нему сортировка по умолчанию.</>}>Оценка</Hint>
                {sortIndicator("score")}
              </th>
              <th className="cursor-pointer px-4 py-2 font-medium" onClick={() => sortByColumn("ping")}>
                <Hint content={<><b>Пинг</b> — средняя задержка рукопожатия за несколько попыток, в мс.</>}>Пинг</Hint>
                {sortIndicator("ping")}
              </th>
              <th className="cursor-pointer px-4 py-2 font-medium" onClick={() => sortByColumn("jitter")}>
                <Hint content={<><b>Джиттер</b> — разброс задержки между попытками, в мс. Чем меньше — тем стабильнее соединение.</>}>Джиттер</Hint>
                {sortIndicator("jitter")}
              </th>
              <th className="cursor-pointer px-4 py-2 font-medium" onClick={() => sortByColumn("success")}>
                <Hint content={<><b>Успех</b> — доля успешных рукопожатий: успешно / всего попыток.</>}>Успех</Hint>
                {sortIndicator("success")}
              </th>
              <th className="px-4 py-2 font-medium">
                <Hint content={<><b>Метод</b> — как прокси ответил: faketls (маскировка рукопожатия под TLS) или obfuscated2.</>}>Метод</Hint>
              </th>
              <th className="cursor-pointer px-4 py-2 font-medium" onClick={() => sortByColumn("lastChecked")}>
                <Hint content={<><b>Проверен</b> — когда прокси последний раз проверялся.</>}>Проверен</Hint>
                {sortIndicator("lastChecked")}
              </th>
              <th className="w-24 px-4 py-2" />
            </tr>
          </thead>
          <tbody>
            {loading && rows.length === 0 ? (
              <tr>
                <td colSpan={15} className="px-4 py-8 text-center text-muted-foreground">
                  Загрузка…
                </td>
              </tr>
            ) : rows.length === 0 ? (
              <tr>
                <td colSpan={15} className="px-4 py-8 text-center text-muted-foreground">
                  Telegram-прокси не найдены. Добавьте источник на странице «Источники».
                </td>
              </tr>
            ) : (
              rows.map((p, idx) => (
                <tr
                  key={p.id}
                  onClick={(e) => handleRowClick(idx, e)}
                  className={cn(
                    "cursor-pointer border-b border-border/60 select-none hover:bg-accent/40",
                    selected.has(p.id) && "bg-primary/10",
                    checking.has(p.id) && "bg-accent/50",
                  )}
                >
                  <td className="px-4 py-2" />
                  <td className="px-4 py-2 font-mono">{p.host}</td>
                  <td className="px-4 py-2 font-mono">{p.port}</td>
                  <td
                    className="px-4 py-2"
                    title={p.serverIp ? `${p.serverIp} · ${countryName(p.country)}` : countryName(p.country)}
                  >
                    {countryFlag(p.country)} {p.country ?? "—"}
                  </td>
                  <td className="px-4 py-2 text-muted-foreground">{p.city ?? "—"}</td>
                  <td className="px-4 py-2 text-muted-foreground">{formatDistance(distanceFor(p))}</td>
                  <td className="px-4 py-2">
                    <Badge variant={p.type === "socks" ? "socks5" : "telegram"}>{p.type}</Badge>
                  </td>
                  <td className="max-w-40 truncate px-4 py-2 font-mono text-xs text-muted-foreground" title={p.secret}>
                    {p.secret ? (p.secret.length > 16 ? `${p.secret.slice(0, 16)}…` : p.secret) : "—"}
                  </td>
                  <td className="px-4 py-2 font-mono">{p.score != null ? Math.round(p.score) : "—"}</td>
                  <td className="px-4 py-2">
                    <Badge variant={latencyVariant(p.pingMs)}>{formatLatency(p.pingMs)}</Badge>
                  </td>
                  <td className="px-4 py-2 text-muted-foreground">{p.jitterMs != null ? `${p.jitterMs} мс` : "—"}</td>
                  <td className="px-4 py-2 text-muted-foreground">
                    {p.attempts > 0 ? `${p.successes}/${p.attempts}` : "—"}
                  </td>
                  <td className="px-4 py-2 text-muted-foreground">{p.method || "—"}</td>
                  <td className="px-4 py-2 text-muted-foreground">{formatRelative(p.lastChecked)}</td>
                  <td className="px-4 py-2 text-right">
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={(e) => {
                        e.stopPropagation();
                        handleTestOne(p.id);
                      }}
                      disabled={running || checking.has(p.id)}
                    >
                      {checking.has(p.id) ? (
                        <>
                          <Loader2 className="size-3.5 animate-spin" />
                          Проверка…
                        </>
                      ) : (
                        "Проверить"
                      )}
                    </Button>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      <div className="flex items-center justify-between border-t border-border px-6 py-3 text-sm">
        <div className="text-muted-foreground">
          Страница {page + 1} из {pageCount}
        </div>
        <div className="flex items-center gap-3">
          <label className="flex items-center gap-2">
            На странице:
            <select
              className="h-8 rounded-md border border-input bg-background px-2 text-foreground"
              value={pageSize}
              onChange={(e) => setPageSize(Number(e.target.value))}
            >
              {pageSizeOptions.map((size) => (
                <option key={size} value={size}>
                  {size}
                </option>
              ))}
            </select>
          </label>
          <Button size="sm" variant="outline" onClick={() => setPage((p) => p - 1)} disabled={page <= 0}>
            Назад
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() => setPage((p) => p + 1)}
            disabled={page + 1 >= pageCount}
          >
            Вперёд
          </Button>
        </div>
      </div>
    </div>
  );
}
