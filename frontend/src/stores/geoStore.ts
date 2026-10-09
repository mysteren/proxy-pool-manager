import { create } from "zustand";

export interface GeoProgressData {
  total: number;
  completed: number;
  current: string;
}

export interface GeoCompletedData {
  cancelled: boolean;
  tested: number;
  working: number;
}

interface GeoState {
  running: boolean;
  total: number;
  completed: number;
  current: string;
  lastResult: GeoCompletedData | null;
  start: () => void;
  progress: (data: GeoProgressData) => void;
  finish: (data: GeoCompletedData) => void;
  reset: () => void;
}

/** Состояние определения гео для MTProto-прокси (события geo:progress/geo:completed). */
export const useGeoStore = create<GeoState>((set) => ({
  running: false,
  total: 0,
  completed: 0,
  current: "",
  lastResult: null,
  start: () => set({ running: true, total: 0, completed: 0, current: "", lastResult: null }),
  progress: (data) => set({ total: data.total, completed: data.completed, current: data.current }),
  finish: (data) => set({ running: false, current: "", lastResult: data }),
  reset: () => set({ running: false, total: 0, completed: 0, current: "" }),
}));
