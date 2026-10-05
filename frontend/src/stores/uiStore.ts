import { create } from "zustand";

export type Page = "proxies" | "sources" | "settings";

interface UiState {
  activePage: Page;
  setActivePage: (page: Page) => void;
}

export const useUiStore = create<UiState>((set) => ({
  activePage: "proxies",
  setActivePage: (page) => set({ activePage: page }),
}));
