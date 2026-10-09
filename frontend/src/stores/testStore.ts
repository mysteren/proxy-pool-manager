import { create } from "zustand";

export interface TestProgressData {
  total: number;
  completed: number;
  current: string;
}

export interface TestCompletedData {
  cancelled: boolean;
  tested: number;
  working: number;
}

interface TestState {
  running: boolean;
  total: number;
  completed: number;
  current: string;
  lastResult: TestCompletedData | null;
  /** Растёт после завершения проверки — им пользуются страницы для перезагрузки. */
  refreshToken: number;
  start: () => void;
  progress: (data: TestProgressData) => void;
  finish: (data: TestCompletedData) => void;
  /** Сброс, если запуск проверки не удался (событие test:completed не придёт). */
  reset: () => void;
}

export const useTestStore = create<TestState>((set) => ({
  running: false,
  total: 0,
  completed: 0,
  current: "",
  lastResult: null,
  refreshToken: 0,
  start: () => set({ running: true, total: 0, completed: 0, current: "", lastResult: null }),
  // Наличие проверки задаётся только start()/finish()/reset(); событие
  // прогресса его не «пере-взводит» — иначе одиночное опоздавшее событие
  // могло бы оставить running=true и навсегда заблокировать кнопки.
  progress: (data) => set({ total: data.total, completed: data.completed, current: data.current }),
  finish: (data) =>
    set((state) => ({
      running: false,
      completed: data.tested,
      current: "",
      lastResult: data,
      refreshToken: state.refreshToken + 1,
    })),
  reset: () => set({ running: false, total: 0, completed: 0, current: "" }),
}));
