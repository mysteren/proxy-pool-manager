import { useEffect } from "react";
import { Events } from "@wailsio/runtime";

import { AppLayout } from "@/components/layout/AppLayout";
import { ProxiesPage } from "@/pages/Proxies";
import { SourcesPage } from "@/pages/Sources";
import { SettingsPage } from "@/pages/Settings";
import { useThemeStore } from "@/stores/themeStore";
import { useUiStore } from "@/stores/uiStore";
import { ThemeService } from "../bindings/proxy-pool-manager/internal/services";

export default function App() {
  const activePage = useUiStore((s) => s.activePage);
  const setSystemTheme = useThemeStore((s) => s.setSystemTheme);

  useEffect(() => {
    // Начальная синхронизация с системной темой.
    ThemeService.GetSystemTheme()
      .then((theme) => setSystemTheme(theme === "dark" ? "dark" : "light"))
      .catch(console.error);

    // Go шлёт "theme:changed" при смене системного оформления.
    const off = Events.On("theme:changed", (event) => {
      setSystemTheme(event.data === "dark" ? "dark" : "light");
    });

    return () => off();
  }, [setSystemTheme]);

  return (
    <AppLayout>
      {activePage === "proxies" && <ProxiesPage />}
      {activePage === "sources" && <SourcesPage />}
      {activePage === "settings" && <SettingsPage />}
    </AppLayout>
  );
}
