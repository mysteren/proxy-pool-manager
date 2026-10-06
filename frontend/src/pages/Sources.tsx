import { useCallback, useEffect, useState } from "react";
import { FileText, Link2, PencilLine, RefreshCw, Trash2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import { formatRelative } from "@/lib/time";
import { toast } from "@/stores/toastStore";
import { SourceService, type FetchResult } from "../../bindings/proxy-pool-manager/internal/services";
import type { Source } from "../../bindings/proxy-pool-manager/internal/models";

type Tab = "url" | "file" | "manual";

const tabs: { value: Tab; label: string; icon: typeof Link2 }[] = [
  { value: "url", label: "URL", icon: Link2 },
  { value: "file", label: "Файл", icon: FileText },
  { value: "manual", label: "Вручную", icon: PencilLine },
];

export function SourcesPage() {
  const [sources, setSources] = useState<Source[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [tab, setTab] = useState<Tab>("url");
  const [name, setName] = useState("");
  const [url, setUrl] = useState("");
  const [filePath, setFilePath] = useState("");
  const [manual, setManual] = useState("");
  const [confirmDeleteId, setConfirmDeleteId] = useState<number | null>(null);

  const load = useCallback(async () => {
    try {
      const list = await SourceService.ListSources();
      setSources(list ?? []);
    } catch (err) {
      toast.error(`Не удалось загрузить источники: ${String(err)}`);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const reportResult = (result: FetchResult) => {
    if (result.fetched === 0 && result.mtprotoAdded > 0) {
      toast.success(`Добавлено Telegram-прокси: ${result.mtprotoAdded}`);
      return;
    }
    const mt = result.mtprotoAdded > 0 ? `, Telegram-прокси: ${result.mtprotoAdded}` : "";
    if (result.fetched > 0 && result.added === 0 && result.mtprotoAdded === 0) {
      toast.info(`Все ${result.fetched} прокси уже в пуле`);
      return;
    }
    toast.success(
      `Добавлено ${result.added} из ${result.fetched} (уже в пуле: ${result.skipped})${mt}`,
    );
  };

  const run = useCallback(
    async (action: () => Promise<FetchResult>) => {
      setBusy(true);
      try {
        const result = await action();
        reportResult(result);
        await load();
        return true;
      } catch (err) {
        toast.error(String(err));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [load],
  );

  const handleAddURL = async () => {
    if (!url.trim()) {
      toast.error("Укажите URL");
      return;
    }
    if (await run(() => SourceService.AddFromURL(name.trim(), url.trim()))) {
      setName("");
      setUrl("");
    }
  };

  const handlePickFile = async () => {
    try {
      const path = await SourceService.PickProxyFile();
      if (path) {
        setFilePath(path);
      }
    } catch (err) {
      toast.error(String(err));
    }
  };

  const handleAddFile = async () => {
    if (!filePath.trim()) {
      toast.error("Выберите файл");
      return;
    }
    if (await run(() => SourceService.AddFromFile(name.trim(), filePath.trim()))) {
      setName("");
      setFilePath("");
    }
  };

  const handleAddManual = async () => {
    if (!manual.trim()) {
      toast.error("Введите список прокси");
      return;
    }
    if (await run(() => SourceService.AddManual(manual))) {
      setManual("");
    }
  };

  const handleRefresh = (id: number) => {
    void run(() => SourceService.RefreshSource(id));
  };

  const handleDelete = async (id: number) => {
    try {
      await SourceService.DeleteSource(id);
      setConfirmDeleteId(null);
      toast.success("Источник удалён");
      await load();
    } catch (err) {
      toast.error(String(err));
    }
  };

  return (
    <div className="flex h-full flex-col">
      <header className="border-b border-border px-6 py-4">
        <h1 className="text-lg font-semibold">Источники</h1>
        <p className="text-sm text-muted-foreground">
          Добавляйте списки прокси по URL, из файла или вручную.
        </p>
      </header>

      <div className="flex-1 overflow-auto p-6">
        <div className="mx-auto flex max-w-3xl flex-col gap-6">
          <div className="flex gap-1 rounded-md border border-border p-1">
            {tabs.map(({ value, label, icon: Icon }) => (
              <button
                key={value}
                type="button"
                onClick={() => setTab(value)}
                className={cn(
                  "flex flex-1 items-center justify-center gap-2 rounded px-3 py-1.5 text-sm transition-colors",
                  tab === value
                    ? "bg-primary text-primary-foreground"
                    : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
                )}
              >
                <Icon className="size-4" />
                {label}
              </button>
            ))}
          </div>

          {tab === "url" && (
            <div className="flex flex-col gap-3">
              <Input
                placeholder="Название (необязательно)"
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
              <Input
                placeholder="https://raw.githubusercontent.com/..."
                value={url}
                onChange={(e) => setUrl(e.target.value)}
              />
              <Button onClick={handleAddURL} disabled={busy} className="self-start">
                Добавить
              </Button>
            </div>
          )}

          {tab === "file" && (
            <div className="flex flex-col gap-3">
              <Input
                placeholder="Название (необязательно)"
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
              <div className="flex gap-2">
                <Input
                  placeholder="Путь к файлу (.txt или .json)"
                  value={filePath}
                  onChange={(e) => setFilePath(e.target.value)}
                />
                <Button variant="outline" onClick={handlePickFile} disabled={busy}>
                  Выбрать…
                </Button>
              </div>
              <Button onClick={handleAddFile} disabled={busy} className="self-start">
                Добавить
              </Button>
            </div>
          )}

          {tab === "manual" && (
            <div className="flex flex-col gap-3">
              <Textarea
                placeholder={"192.168.1.1:8080\nsocks5://10.0.0.1:1080"}
                value={manual}
                onChange={(e) => setManual(e.target.value)}
              />
              <Button onClick={handleAddManual} disabled={busy} className="self-start">
                Добавить в пул
              </Button>
            </div>
          )}

          <section className="flex flex-col gap-3">
            <h2 className="text-sm font-medium">
              Источники{sources.length > 0 ? ` (${sources.length})` : ""}
            </h2>

            {loading ? (
              <p className="text-sm text-muted-foreground">Загрузка…</p>
            ) : sources.length === 0 ? (
              <p className="text-sm text-muted-foreground">Источников пока нет.</p>
            ) : (
              <div className="flex flex-col gap-3">
                {sources.map((source) => (
                  <Card key={source.id}>
                    <CardHeader>
                      <CardTitle>{source.name}</CardTitle>
                      <p className="truncate text-xs text-muted-foreground">
                        {source.url ?? source.filePath ?? "—"}
                      </p>
                    </CardHeader>
                    <CardContent className="flex items-center justify-between gap-4">
                      <div className="min-w-0 text-xs text-muted-foreground">
                        <div>{source.proxyCount} прокси</div>
                        <div>Обновлён: {formatRelative(source.lastFetched)}</div>
                      </div>
                      {confirmDeleteId === source.id ? (
                        <div className="flex items-center gap-2">
                          <Button size="sm" variant="destructive" onClick={() => handleDelete(source.id)}>
                            Удалить
                          </Button>
                          <Button size="sm" variant="ghost" onClick={() => setConfirmDeleteId(null)}>
                            Отмена
                          </Button>
                        </div>
                      ) : (
                        <div className="flex items-center gap-2">
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => handleRefresh(source.id)}
                            disabled={busy}
                          >
                            <RefreshCw className="size-3.5" />
                            Обновить
                          </Button>
                          <Button
                            size="icon"
                            variant="ghost"
                            aria-label="Удалить источник"
                            onClick={() => setConfirmDeleteId(source.id)}
                          >
                            <Trash2 className="size-4" />
                          </Button>
                        </div>
                      )}
                    </CardContent>
                  </Card>
                ))}
              </div>
            )}
          </section>
        </div>
      </div>
    </div>
  );
}
