import { create } from "zustand";

export type Page = "proxies" | "mtproto" | "sources" | "settings";

interface UiState {
  activePage: Page;
  setActivePage: (page: Page) => void;
}

export const useUiStore = create<UiState>((set) => ({
  activePage: "proxies",
  setActivePage: (page) => set({ activePage: page }),
}));
