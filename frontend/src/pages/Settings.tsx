import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { toast } from "@/stores/toastStore";
import { useThemeStore, type ThemeMode } from "@/stores/themeStore";
import {
  SettingsService,
  type Settings,
} from "../../bindings/proxy-pool-manager/internal/services";

const themeOptions: { value: ThemeMode; label: string }[] = [
  { value: "system", label: "Системная" },
  { value: "light", label: "Светлая" },
  { value: "dark", label: "Тёмная" },
];

export function SettingsPage() {
  const mode = useThemeStore((s) => s.mode);
  const setMode = useThemeStore((s) => s.setMode);

  const [settings, setSettings] = useState<Settings | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    SettingsService.Get()
      .then((s) => setSettings(s))
      .catch((err) => toast.error(`Не удалось загрузить настройки: ${String(err)}`));
  }, []);

  const update = (patch: Partial<Settings>) => {
    setSettings((prev) => (prev ? ({ ...prev, ...patch } as Settings) : prev));
  };

  const handleSave = async () => {
    if (!settings) {
      return;
    }
    setSaving(true);
    try {
      const saved = await SettingsService.Update({ ...settings, theme: mode } as Settings);
      setSettings(saved);
      toast.success("Настройки сохранены");
    } catch (err) {
      toast.error(String(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="flex h-full flex-col">
      <header className="border-b border-border px-6 py-4">
        <h1 className="text-lg font-semibold">Настройки</h1>
      </header>

      <div className="flex-1 overflow-auto p-6">
        <div className="flex max-w-xl flex-col gap-6">
          <section>
            <h2 className="text-sm font-medium">Тема</h2>
            <div className="mt-3 inline-flex rounded-md border border-border p-1">
              {themeOptions.map((option) => (
                <button
                  key={option.value}
                  type="button"
                  onClick={() => setMode(option.value)}
                  className={cn(
                    "rounded px-3 py-1.5 text-sm transition-colors",
                    mode === option.value
                      ? "bg-primary text-primary-foreground"
                      : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
                  )}
                >
                  {option.label}
                </button>
              ))}
            </div>
          </section>

          {settings ? (
            <>
              <section className="flex flex-col gap-3">
                <h2 className="text-sm font-medium">Проверка прокси</h2>
                <label className="flex flex-col gap-1 text-sm">
                  Число одновременных проверок
                  <Input
                    type="number"
                    min={1}
                    max={500}
                    value={settings.testConcurrency}
                    onChange={(e) => update({ testConcurrency: Number(e.target.value) })}
                    className="w-40"
                  />
                  <span className="text-xs text-muted-foreground">
                    Меньше — снижает нагрузку на сеть и DNS.
                  </span>
                </label>
                <label className="flex flex-col gap-1 text-sm">
                  Таймаут проверки, мс
                  <Input
                    type="number"
                    min={500}
                    max={60000}
                    step={100}
                    value={settings.latencyTimeoutMs}
                    onChange={(e) => update({ latencyTimeoutMs: Number(e.target.value) })}
                    className="w-40"
                  />
                </label>
                <label className="flex flex-col gap-1 text-sm">
                  Размер файла для теста скорости, байт
                  <Input
                    type="number"
                    min={100000}
                    max={10000000}
                    step={100000}
                    value={settings.speedDownloadBytes}
                    onChange={(e) => update({ speedDownloadBytes: Number(e.target.value) })}
                    className="w-40"
                  />
                </label>
                <label className="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    className="size-4 accent-primary"
                    checked={settings.validateViaHttp}
                    onChange={(e) => update({ validateViaHttp: e.target.checked })}
                  />
                  Подтверждать работоспособность HTTP-запросом через прокси
                </label>
                <label className="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    className="size-4 accent-primary"
                    checked={settings.speedTest}
                    onChange={(e) => update({ speedTest: e.target.checked })}
                  />
                  Измерять скорость при проверке
                </label>
                <label className="flex flex-col gap-1 text-sm">
                  URL для проверки
                  <Input
                    value={settings.httpValidationUrl}
                    onChange={(e) => update({ httpValidationUrl: e.target.value })}
                  />
                </label>
              </section>

              <section className="flex flex-col gap-3">
                <h2 className="text-sm font-medium">Копирование</h2>
                <label className="flex flex-col gap-1 text-sm">
                  Формат
                  <select
                    className="h-9 w-56 rounded-md border border-input bg-background px-2 text-sm text-foreground"
                    value={settings.copyFormat}
                    onChange={(e) => update({ copyFormat: e.target.value })}
                  >
                    <option value="uri">protocol://host:port</option>
                    <option value="hostport">host:port</option>
                  </select>
                </label>
              </section>

              <div>
                <Button onClick={handleSave} disabled={saving}>
                  Сохранить
                </Button>
              </div>
            </>
          ) : (
            <p className="text-sm text-muted-foreground">Загрузка…</p>
          )}
        </div>
      </div>
    </div>
  );
}
