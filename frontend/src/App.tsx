import { useEffect } from "react";
import { Events } from "@wailsio/runtime";

import { AppLayout } from "@/components/layout/AppLayout";
import { Toaster } from "@/components/ui/toaster";
import { ProxiesPage } from "@/pages/Proxies";
import { SourcesPage } from "@/pages/Sources";
import { SettingsPage } from "@/pages/Settings";
import { useThemeStore } from "@/stores/themeStore";
import { useUiStore } from "@/stores/uiStore";
import { useTestStore } from "@/stores/testStore";
import { toast } from "@/stores/toastStore";
import { ThemeService } from "../bindings/proxy-pool-manager/internal/services";

export default function App() {
  const activePage = useUiStore((s) => s.activePage);
  const setSystemTheme = useThemeStore((s) => s.setSystemTheme);
  const testProgress = useTestStore((s) => s.progress);
  const testFinish = useTestStore((s) => s.finish);

  useEffect(() => {
    ThemeService.GetSystemTheme()
      .then((theme) => setSystemTheme(theme === "dark" ? "dark" : "light"))
      .catch(console.error);

    const offTheme = Events.On("theme:changed", (event) => {
      setSystemTheme(event.data === "dark" ? "dark" : "light");
    });

    const offProgress = Events.On("test:progress", (event) => {
      testProgress({
        total: event.data.total,
        completed: event.data.completed,
        current: event.data.current,
      });
    });

    const offCompleted = Events.On("test:completed", (event) => {
      const result = event.data;
      testFinish({
        cancelled: result.cancelled,
        tested: result.tested,
        working: result.working,
      });
      if (result.cancelled) {
        toast.info("Проверка отменена");
      } else {
        toast.success(`Проверка завершена: рабочих ${result.working} из ${result.tested}`);
      }
    });

    return () => {
      offTheme();
      offProgress();
      offCompleted();
    };
  }, [setSystemTheme, testProgress, testFinish]);

  return (
    <>
      <AppLayout>
        {activePage === "proxies" && <ProxiesPage />}
        {activePage === "sources" && <SourcesPage />}
        {activePage === "settings" && <SettingsPage />}
      </AppLayout>
      <Toaster />
    </>
  );
}
