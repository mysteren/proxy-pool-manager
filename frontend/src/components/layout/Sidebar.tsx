import { ListTree, Send, Rss, Settings as SettingsIcon, Moon, Sun } from "lucide-react";
import { cn } from "@/lib/utils";
import { useUiStore, type Page } from "@/stores/uiStore";
import { useThemeStore } from "@/stores/themeStore";

interface NavItem {
  page: Page;
  label: string;
  icon: typeof ListTree;
}

const items: NavItem[] = [
  { page: "proxies", label: "Прокси", icon: ListTree },
  { page: "mtproto", label: "MTProto", icon: Send },
  { page: "sources", label: "Источники", icon: Rss },
  { page: "settings", label: "Настройки", icon: SettingsIcon },
];

export function Sidebar() {
  const activePage = useUiStore((s) => s.activePage);
  const setActivePage = useUiStore((s) => s.setActivePage);
  const resolved = useThemeStore((s) => s.resolved);
  const setMode = useThemeStore((s) => s.setMode);

  return (
    <aside className="flex w-60 shrink-0 flex-col justify-between border-r border-sidebar-border bg-sidebar text-sidebar-foreground">
      <div className="flex flex-col gap-1 p-3">
        <div className="px-2 py-3 text-sm font-semibold tracking-tight">
          Proxy Pool Manager
        </div>
        <nav className="flex flex-col gap-1">
          {items.map(({ page, label, icon: Icon }) => (
            <button
              key={page}
              type="button"
              onClick={() => setActivePage(page)}
              className={cn(
                "flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors",
                "hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
                activePage === page &&
                  "bg-sidebar-primary text-sidebar-primary-foreground hover:bg-sidebar-primary hover:text-sidebar-primary-foreground",
              )}
            >
              <Icon className="size-4" />
              {label}
            </button>
          ))}
        </nav>
      </div>
      <div className="p-3">
        <button
          type="button"
          onClick={() => setMode(resolved === "dark" ? "light" : "dark")}
          className="flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
        >
          {resolved === "dark" ? (
            <Sun className="size-4" />
          ) : (
            <Moon className="size-4" />
          )}
          {resolved === "dark" ? "Светлая тема" : "Тёмная тема"}
        </button>
      </div>
    </aside>
  );
}
