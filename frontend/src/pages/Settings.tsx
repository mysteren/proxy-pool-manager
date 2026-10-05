import { cn } from "@/lib/utils";
import { useThemeStore, type ThemeMode } from "@/stores/themeStore";

const themeOptions: { value: ThemeMode; label: string }[] = [
  { value: "system", label: "Системная" },
  { value: "light", label: "Светлая" },
  { value: "dark", label: "Тёмная" },
];

export function SettingsPage() {
  const mode = useThemeStore((s) => s.mode);
  const setMode = useThemeStore((s) => s.setMode);

  return (
    <div className="flex h-full flex-col">
      <header className="border-b border-border px-6 py-4">
        <h1 className="text-lg font-semibold">Настройки</h1>
      </header>
      <div className="flex-1 overflow-auto p-6">
        <section className="max-w-xl">
          <h2 className="text-sm font-medium">Тема</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            Системная — следует за оформлением ОС. Остальные настройки появятся
            на этапе Milestone 5.
          </p>
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
      </div>
    </div>
  );
}
