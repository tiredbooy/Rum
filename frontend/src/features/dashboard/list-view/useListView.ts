import { create } from "zustand";
import {
  DEFAULT_VIEW,
  loadView,
  saveView,
  type ListView,
} from "./list-view";

interface ListViewStore extends ListView {
  setQuery: (query: string) => void;
  setTab: (tab: ListView["tab"]) => void;
  setSort: (sort: ListView["sort"]) => void;
  /** Clears the search and goes back to the All tab (sort is kept). */
  clearFilters: () => void;
}

const pick = (s: ListView): ListView => ({ query: s.query, tab: s.tab, sort: s.sort });

/** Per-viewer list controls (search, status tab, sort), saved in localStorage. */
export const useListView = create<ListViewStore>((set, get) => {
  const update = (patch: Partial<ListView>) => {
    set(patch);
    saveView(pick(get()));
  };
  return {
    ...loadView(),
    setQuery: (query) => update({ query }),
    setTab: (tab) => update({ tab }),
    setSort: (sort) => update({ sort }),
    clearFilters: () => update({ query: DEFAULT_VIEW.query, tab: DEFAULT_VIEW.tab }),
  };
});
