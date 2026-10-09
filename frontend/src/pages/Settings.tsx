import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import { Hint } from "@/components/ui/hint";
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

  const handleClearGeoCache = async () => {
    try {
      const count = await SettingsService.ClearGeoCache();
      toast.success(`Гео-кэш очищен: ${count}`);
    } catch (err) {
      toast.error(String(err));
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
                  <Hint content={<><b>Число одновременных проверок.</b> Сколько прокси проверять параллельно. Меньше — ниже нагрузка на сеть и DNS, но обход идёт дольше.</>}>
                    Число одновременных проверок
                  </Hint>
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
                  <Hint content={<><b>Таймаут проверки.</b> Сколько ждать ответа от прокси. Больший таймаут повышает шанс поймать медленные прокси, но удлиняет обход.</>}>
                    Таймаут проверки, мс
                  </Hint>
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
                  <Hint content={<><b>Размер файла для теста скорости.</b> Сколько байт скачивать через прокси. Больше — точнее результат, но дольше проверка.</>}>
                    Размер файла для теста скорости, байт
                  </Hint>
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
                  <Hint content={<><b>HTTP-подтверждение.</b> Прокси считается рабочим не только по TCP-соединению, но и по реальному HTTP-ответу через него. Честнее, но медленнее.</>}>
                    Подтверждать работоспособность HTTP-запросом через прокси
                  </Hint>
                </label>
                <label className="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    className="size-4 accent-primary"
                    checked={settings.speedTest}
                    onChange={(e) => update({ speedTest: e.target.checked })}
                  />
                  <Hint content={<><b>Измерять скорость.</b> Замер скачивания при каждой проверке. Замедляет обход; нулевая скорость считается нерабочим прокси.</>}>
                    Измерять скорость при проверке
                  </Hint>
                </label>
                <label className="flex flex-col gap-1 text-sm">
                  <Hint content={<><b>Замеров скорости.</b> Сколько раз мерить скорость: 1 — быстро; 2 — берётся максимум; 3 — медиана (устойчивее к выбросам). При 2–3 размер файла делится между замерами, поэтому трафик почти не растёт, но проверка дольше.</>}>
                    Замеров скорости
                  </Hint>
                  <select
                    className="h-9 w-56 rounded-md border border-input bg-background px-2 text-sm text-foreground"
                    value={settings.speedSamples}
                    onChange={(e) => update({ speedSamples: Number(e.target.value) })}
                  >
                    <option value={1}>1 — один замер</option>
                    <option value={2}>2 — максимум</option>
                    <option value={3}>3 — медиана</option>
                  </select>
                </label>
                <label className="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    className="size-4 accent-primary"
                    checked={settings.testRandomOrder}
                    onChange={(e) => update({ testRandomOrder: e.target.checked })}
                  />
                  Проверять в случайном порядке
                  <span className="text-xs text-muted-foreground">
                    — помогает быстро найти рабочие в большом пуле
                  </span>
                </label>
                <label className="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    className="size-4 accent-primary"
                    checked={settings.geoConsensus}
                    onChange={(e) => update({ geoConsensus: e.target.checked })}
                  />
                  Гео и IP по нескольким источникам (консенсус)
                  <span className="text-xs text-muted-foreground">
                    — страна/город/координаты определяются большинством голосов
                  </span>
                </label>
                <label className="flex flex-col gap-1 text-sm">
                  <Hint content={<><b>URL для проверки.</b> Адрес, через который подтверждается работа прокси и берётся exit-IP/гео. При включённом консенсусе опрашиваются ещё и встроенные источники.</>}>
                    URL для проверки
                  </Hint>
                  <Input
                    value={settings.httpValidationUrl}
                    onChange={(e) => update({ httpValidationUrl: e.target.value })}
                  />
                </label>
              </section>

              <section className="flex flex-col gap-3">
                <h2 className="text-sm font-medium">Гео</h2>
                <label className="flex flex-col gap-1 text-sm">
                  <Hint content={<><b>Срок хранения гео-кэша.</b> Гео узла (страна/город/координаты) меняется редко, поэтому результаты по IP кэшируются и переиспользуются. 0 — без срока.</>}>
                    Срок хранения гео-кэша, дней
                  </Hint>
                  <Input
                    type="number"
                    min={0}
                    max={365}
                    value={settings.geoCacheTtlDays}
                    onChange={(e) => update({ geoCacheTtlDays: Number(e.target.value) })}
                    className="w-40"
                  />
                </label>
                <div>
                  <Button variant="outline" size="sm" onClick={handleClearGeoCache}>
                    Сбросить гео-кэш
                  </Button>
                </div>
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
