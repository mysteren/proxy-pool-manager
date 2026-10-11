// Флаги берём как SVG-URL из пакета flag-icons, но только набор 4x3 (без
// квадратных 1x1) — так бандл примерно вдвое легче, чем при импорте всего CSS
// пакета. Эмодзи-флаги не используем: в WebView2 на Windows их нет.
const modules = import.meta.glob("/node_modules/flag-icons/flags/4x3/*.svg", {
  query: "?url",
  import: "default",
  eager: true,
}) as Record<string, string>;

const flags: Record<string, string> = {};
for (const [path, url] of Object.entries(modules)) {
  const name = path.slice(path.lastIndexOf("/") + 1, -".svg".length);
  flags[name] = url;
}

/** flagUrl возвращает URL SVG-флага по коду страны (US, DE, …) или undefined. */
export function flagUrl(code: string | null | undefined): string | undefined {
  if (!code || !/^[a-zA-Z]{2}$/.test(code)) {
    return undefined;
  }
  return flags[code.toLowerCase()];
}
