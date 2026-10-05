import { create } from "zustand";

export type ThemeMode = "system" | "light" | "dark";
export type ResolvedTheme = "light" | "dark";

const STORAGE_KEY = "theme-mode";

interface ThemeState {
  /** Выбор пользователя. */
  mode: ThemeMode;
  /** Фактически применённая тема. */
  resolved: ResolvedTheme;
  setMode: (mode: ThemeMode) => void;
  /** Вызывается, когда Go сообщает системную тему (GetSystemTheme / theme:changed). */
  setSystemTheme: (theme: ResolvedTheme) => void;
}

function readStoredMode(): ThemeMode {
  const value = localStorage.getItem(STORAGE_KEY);
  return value === "light" || value === "dark" || value === "system"
    ? value
    : "system";
}

function applyResolved(theme: ResolvedTheme) {
  document.documentElement.classList.toggle("dark", theme === "dark");
}

export const useThemeStore = create<ThemeState>((set, get) => ({
  mode: readStoredMode(),
  // Значение уже могло быть проставлено inline-скриптом в index.html.
  resolved: document.documentElement.classList.contains("dark") ? "dark" : "light",
  setMode: (mode) => {
    localStorage.setItem(STORAGE_KEY, mode);
    set({ mode });
    if (mode === "light" || mode === "dark") {
      applyResolved(mode);
      set({ resolved: mode });
    }
    // В режиме "system" фактическую тему сообщит Go (см. App.tsx).
  },
  setSystemTheme: (theme) => {
    if (get().mode !== "system") return;
    applyResolved(theme);
    set({ resolved: theme });
  },
}));
