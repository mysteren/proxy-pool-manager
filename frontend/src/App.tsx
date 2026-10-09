import { useEffect } from "react";
import { Events } from "@wailsio/runtime";

import { AppLayout } from "@/components/layout/AppLayout";
import { Toaster } from "@/components/ui/toaster";
import { ProxiesPage } from "@/pages/Proxies";
import { MTProtoPage } from "@/pages/MTProto";
import { SourcesPage } from "@/pages/Sources";
import { SettingsPage } from "@/pages/Settings";
import { useThemeStore } from "@/stores/themeStore";
import { useUiStore } from "@/stores/uiStore";
import { useTestStore } from "@/stores/testStore";
import { useGeoStore } from "@/stores/geoStore";
import { toast } from "@/stores/toastStore";
import { ThemeService } from "../bindings/proxy-pool-manager/internal/services";

export default function App() {
  const activePage = useUiStore((s) => s.activePage);
  const setSystemTheme = useThemeStore((s) => s.setSystemTheme);
  const testProgress = useTestStore((s) => s.progress);
  const testFinish = useTestStore((s) => s.finish);
  const geoProgress = useGeoStore((s) => s.progress);
  const geoFinish = useGeoStore((s) => s.finish);

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

    const offGeoProgress = Events.On("geo:progress", (event) => {
      geoProgress({
        total: event.data.total,
        completed: event.data.completed,
        current: event.data.current,
      });
    });

    const offGeoCompleted = Events.On("geo:completed", (event) => {
      const result = event.data;
      geoFinish({
        cancelled: result.cancelled,
        tested: result.tested,
        working: result.working,
      });
      if (result.cancelled) {
        toast.info("Определение гео отменено");
      } else {
        toast.success(`Гео определено: ${result.working} из ${result.tested}`);
      }
    });

    return () => {
      offTheme();
      offProgress();
      offCompleted();
      offGeoProgress();
      offGeoCompleted();
    };
  }, [setSystemTheme, testProgress, testFinish, geoProgress, geoFinish]);

  return (
    <>
      <AppLayout>
        {activePage === "proxies" && <ProxiesPage />}
        {activePage === "mtproto" && <MTProtoPage />}
        {activePage === "sources" && <SourcesPage />}
        {activePage === "settings" && <SettingsPage />}
      </AppLayout>
      <Toaster />
    </>
  );
}
