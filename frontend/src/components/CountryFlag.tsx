import { cn } from "@/lib/utils";
import { flagUrl } from "@/lib/flags";

// CountryFlag рисует флаг страны SVG-картинкой из пакета flag-icons, а не эмодзи:
// эмодзи-флаги не отображаются в WebView2 на Windows (в шрифте Segoe UI Emoji нет
// региональных флагов), поэтому на Windows вместо флага были буквы. SVG-флаги
// рендерятся одинаково на Windows, Linux и macOS.
export function CountryFlag({ code, className }: { code?: string | null; className?: string }) {
  const url = flagUrl(code);
  if (!url) {
    return null;
  }
  return (
    <img
      src={url}
      alt=""
      aria-hidden="true"
      className={cn("inline-block h-3 w-4 rounded-[2px] object-cover align-[-1px]", className)}
    />
  );
}
