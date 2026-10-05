import { useCallback, useEffect, useMemo, useState } from "react";
import { Copy, Download, Eraser, Play, RefreshCw, Trash2, X } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { formatRelative } from "@/lib/time";
import { formatDownload, formatLatency, latencyVariant, protocolVariant } from "@/lib/proxy";
import { toast } from "@/stores/toastStore";
import { useTestStore } from "@/stores/testStore";
import { ProxyService, TesterService } from "../../bindings/proxy-pool-manager/internal/services";
import { Proxy, ProxyFilter } from "../../bindings/proxy-pool-manager/internal/models";

type Status = "all" | "working" | "broken" | "unchecked";
type ConfirmAction = { kind: "delete" | "clear"; label: string };

const pageSizeOptions = [25, 50, 100, 200];
const latencyOptions = [
  { value: "", label: "Любая" },
  { value: "100", label: "< 100 мс" },
  { value: "500", label: "< 500 мс" },
  { value: "1000", label: "< 1000 мс" },
];
const statusOptions: { value: Status; label: string }[] = [
  { value: "all", label: "Все" },
  { value: "working", label: "Рабочие" },
  { value: "broken", label: "Нерабочие" },
  { value: "unchecked", label: "Без статуса" },
];

export function ProxiesPage() {
  const [rows, setRows] = useState<Proxy[]>([]);
  const [total, setTotal] = useState(0);
  const [workingTotal, setWorkingTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState(50);
  const [sortBy, setSortBy] = useState("host");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("asc");
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [status, setStatus] = useState<Status>("all");
  const [protocol, setProtocol] = useState("");
  const [maxLatency, setMaxLatency] = useState("");
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [confirm, setConfirm] = useState<ConfirmAction | null>(null);
  const [exportFormat, setExportFormat] = useState("txt");

  const running = useTestStore((s) => s.running);
  const progressTotal = useTestStore((s) => s.total);
  const progressCompleted = useTestStore((s) => s.completed);
  const progressCurrent = useTestStore((s) => s.current);
  const refreshToken = useTestStore((s) => s.refreshToken);
  const startTest = useTestStore((s) => s.start);

  useEffect(() => {
    const id = setTimeout(() => setDebouncedSearch(search), 300);
    return () => clearTimeout(id);
  }, [search]);

  useEffect(() => {
    setPage(0);
  }, [debouncedSearch, status, protocol, maxLatency, pageSize]);

  const buildFilter = useCallback(
    (offset: number): ProxyFilter => {
      const filter: ProxyFilter = {
        onlyWorking: null,
        unchecked: null,
        protocol: protocol || null,
        maxLatency: maxLatency ? Number(maxLatency) : null,
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
    [status, protocol, maxLatency, debouncedSearch, sortBy, sortDir, pageSize],
  );

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const filter = buildFilter(page * pageSize);
      const [list, count, workingCount] = await Promise.all([
        ProxyService.GetProxies(filter),
        ProxyService.CountProxies(filter),
        ProxyService.CountProxies({ ...filter, onlyWorking: true, unchecked: null, limit: 0, offset: 0 }),
      ]);
      setRows(list ?? []);
      setTotal(count);
      setWorkingTotal(workingCount);
    } catch (err) {
      toast.error(`Не удалось загрузить пул: ${String(err)}`);
    } finally {
      setLoading(false);
    }
  }, [buildFilter, page, pageSize]);

  useEffect(() => {
    void load();
  }, [load, refreshToken]);

  const pageIds = useMemo(() => rows.map((r) => r.id), [rows]);
  const allOnPageSelected = pageIds.length > 0 && pageIds.every((id) => selected.has(id));
  const pageCount = Math.max(1, Math.ceil(total / pageSize));

  const toggleRow = (id: number) =>
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });

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
      toast.error(String(err));
    }
  };

  const handleTestOne = async (id: number) => {
    try {
      const result = await TesterService.TestProxy(id);
      if (result.isWorking) {
        toast.success(`Работает${result.latencyMs != null ? `, ${result.latencyMs} мс` : ""}`);
      } else {
        toast.error(result.error ? `Не работает: ${result.error}` : "Не работает");
      }
      await load();
    } catch (err) {
      toast.error(String(err));
    }
  };

  const doDelete = async () => {
    const ids = [...selected];
    try {
      await ProxyService.DeleteProxies(ids);
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
          ? await ProxyService.ClearStatusByIDs([...selected])
          : await ProxyService.ClearStatus(buildFilter(0));
      setConfirm(null);
      toast.success(`Статус сброшен: ${count}`);
      await load();
    } catch (err) {
      toast.error(String(err));
    }
  };

  const handleCopy = async () => {
    try {
      const count = await ProxyService.CopyToClipboard([...selected]);
      toast.success(`Скопировано: ${count}`);
    } catch (err) {
      toast.error(String(err));
    }
  };

  const handleExport = async () => {
    try {
      const path = await ProxyService.PickExportPath(exportFormat);
      if (!path) {
        return;
      }
      const count =
        selected.size > 0
          ? await ProxyService.ExportByIDs([...selected], exportFormat, path)
          : await ProxyService.ExportByFilter(buildFilter(0), exportFormat, path);
      toast.success(`Экспортировано: ${count}`);
    } catch (err) {
      toast.error(String(err));
    }
  };

  const sortIndicator = (column: string) =>
    sortBy === column ? (sortDir === "asc" ? " ↑" : " ↓") : "";

  return (
    <div className="flex h-full flex-col">
      <header className="flex items-center justify-between border-b border-border px-6 py-4">
        <div>
          <h1 className="text-lg font-semibold">Прокси</h1>
          <p className="text-sm text-muted-foreground">
            Всего: {total} · рабочих: {workingTotal}
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
          className="h-9 rounded-md border border-input bg-transparent px-2 text-sm"
          value={protocol}
          onChange={(e) => setProtocol(e.target.value)}
        >
          <option value="">Все протоколы</option>
          <option value="http">http</option>
          <option value="socks5">socks5</option>
        </select>
        <select
          className="h-9 rounded-md border border-input bg-transparent px-2 text-sm"
          value={maxLatency}
          onChange={(e) => setMaxLatency(e.target.value)}
        >
          {latencyOptions.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      </div>

      <div className="flex flex-wrap items-center gap-2 border-b border-border px-6 py-3">
        {confirm ? (
          <>
            <span className="text-sm">{confirm.label}</span>
            <Button size="sm" variant={confirm.kind === "delete" ? "destructive" : "default"} onClick={confirm.kind === "delete" ? doDelete : doClear}>
              Да
            </Button>
            <Button size="sm" variant="ghost" onClick={() => setConfirm(null)}>
              Отмена
            </Button>
          </>
        ) : (
          <>
            <Button
              size="sm"
              onClick={() => runBatch(() => TesterService.TestProxies([...selected]), `Проверка запущена: ${selected.size}`)}
              disabled={running || selected.size === 0}
            >
              <Play className="size-3.5" />
              Проверить выбранные
            </Button>
            <Button
              size="sm"
              variant="outline"
              onClick={() => runBatch(() => TesterService.TestUnchecked(), "Проверка прокси без статуса запущена")}
              disabled={running}
            >
              Без статуса
            </Button>
            <Button
              size="sm"
              variant="outline"
              onClick={() => runBatch(() => TesterService.TestNonWorking(), "Проверка нерабочих прокси запущена")}
              disabled={running}
            >
              <RefreshCw className="size-3.5" />
              Нерабочие
            </Button>
            <Button
              size="sm"
              variant="outline"
              onClick={() => runBatch(() => TesterService.TestAll(), "Проверка всего пула запущена")}
              disabled={running}
            >
              Проверить всё
            </Button>
            <Button
              size="sm"
              variant="ghost"
              onClick={() => setConfirm({ kind: "clear", label: selected.size > 0 ? `Сбросить статус у ${selected.size} выбранных?` : "Сбросить статус у всех по фильтру?" })}
              disabled={running}
            >
              <Eraser className="size-3.5" />
              Очистить статус
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

      {running && (
        <div className="flex items-center gap-3 border-b border-border px-6 py-2 text-sm">
          <span className="whitespace-nowrap">
            Проверка: {progressCompleted}
            {progressTotal ? ` / ${progressTotal}` : ""}
          </span>
          <div className="h-1.5 flex-1 overflow-hidden rounded bg-muted">
            <div
              className="h-full rounded bg-primary transition-[width]"
              style={{ width: `${progressTotal ? (progressCompleted / progressTotal) * 100 : 0}%` }}
            />
          </div>
          <span className="w-40 truncate text-xs text-muted-foreground">{progressCurrent}</span>
          <Button size="sm" variant="outline" onClick={() => void TesterService.Cancel()}>
            <X className="size-3.5" />
            Отмена
          </Button>
        </div>
      )}

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
                Host{sortIndicator("host")}
              </th>
              <th className="cursor-pointer px-4 py-2 font-medium" onClick={() => sortByColumn("port")}>
                Порт{sortIndicator("port")}
              </th>
              <th className="cursor-pointer px-4 py-2 font-medium" onClick={() => sortByColumn("protocol")}>
                Протокол{sortIndicator("protocol")}
              </th>
              <th className="cursor-pointer px-4 py-2 font-medium" onClick={() => sortByColumn("latency")}>
                Latency{sortIndicator("latency")}
              </th>
              <th className="px-4 py-2 font-medium">Скорость</th>
              <th className="cursor-pointer px-4 py-2 font-medium" onClick={() => sortByColumn("lastChecked")}>
                Проверен{sortIndicator("lastChecked")}
              </th>
              <th className="w-24 px-4 py-2" />
            </tr>
          </thead>
          <tbody>
            {loading && rows.length === 0 ? (
              <tr>
                <td colSpan={8} className="px-4 py-8 text-center text-muted-foreground">
                  Загрузка…
                </td>
              </tr>
            ) : rows.length === 0 ? (
              <tr>
                <td colSpan={8} className="px-4 py-8 text-center text-muted-foreground">
                  Ничего не найдено.
                </td>
              </tr>
            ) : (
              rows.map((p) => (
                <tr key={p.id} className="border-b border-border/60 hover:bg-accent/40">
                  <td className="px-4 py-2">
                    <input
                      type="checkbox"
                      className="size-4 accent-primary"
                      checked={selected.has(p.id)}
                      onChange={() => toggleRow(p.id)}
                      aria-label={`Выбрать ${p.host}:${p.port}`}
                    />
                  </td>
                  <td className="px-4 py-2 font-mono">{p.host}</td>
                  <td className="px-4 py-2 font-mono">{p.port}</td>
                  <td className="px-4 py-2">
                    <Badge variant={protocolVariant(p.protocol)}>{p.protocol}</Badge>
                  </td>
                  <td className="px-4 py-2">
                    <Badge variant={latencyVariant(p.latencyMs)}>{formatLatency(p.latencyMs)}</Badge>
                  </td>
                  <td className="px-4 py-2 text-muted-foreground">{formatDownload(p.downloadMbps)}</td>
                  <td className="px-4 py-2 text-muted-foreground">{formatRelative(p.lastChecked)}</td>
                  <td className="px-4 py-2 text-right">
                    <Button size="sm" variant="ghost" onClick={() => handleTestOne(p.id)} disabled={running}>
                      Проверить
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
              className="h-8 rounded-md border border-input bg-transparent px-2"
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
