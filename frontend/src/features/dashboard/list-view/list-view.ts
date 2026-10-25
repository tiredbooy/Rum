import type { Download } from "@/_lib/types/download-types";

/** Tab values used by the dashboard filter tabs (the status chips). */
export const VIEW_TABS = ["all", "running", "paused", "pending", "completed", "failed"] as const;
export type ViewTab = (typeof VIEW_TABS)[number];

export const SORT_KEYS = ["default", "added", "name", "size", "progress"] as const;
export type SortKey = (typeof SORT_KEYS)[number];

export const SORT_LABELS: Record<SortKey, string> = {
  default: "Default",
  added: "Date added",
  name: "Name",
  size: "Size",
  progress: "Progress",
};

export interface ListView {
  query: string;
  tab: ViewTab;
  sort: SortKey;
}

export const DEFAULT_VIEW: ListView = { query: "", tab: "all", sort: "default" };

const STORAGE_KEY = "rum.listView";

/** Display name: file name, else the last path segment of the URL. */
export function downloadName(d: Download): string {
  if (d.filename) return d.filename;
  try {
    const seg = new URL(d.url).pathname.split("/").filter(Boolean).pop();
    if (seg) return decodeURIComponent(seg);
  } catch {
    /* not a parseable URL: fall through */
  }
  return d.url;
}

function matchesQuery(d: Download, q: string): boolean {
  return downloadName(d).toLowerCase().includes(q) || d.url.toLowerCase().includes(q);
}

const time = (d: Download) => (d.created_at ? Date.parse(d.created_at) || 0 : 0);

const COMPARE: Record<Exclude<SortKey, "default">, (a: Download, b: Download) => number> = {
  added: (a, b) => time(b) - time(a),
  name: (a, b) => downloadName(a).localeCompare(downloadName(b), undefined, { sensitivity: "base" }),
  size: (a, b) => (b.total_size ?? 0) - (a.total_size ?? 0),
  progress: (a, b) => (b.progress ?? 0) - (a.progress ?? 0),
};

/** Applies search and sort. "default" keeps the server order. Pure. */
export function filterAndSort(downloads: Download[], view: ListView): Download[] {
  const q = view.query.trim().toLowerCase();
  const matched = q ? downloads.filter((d) => matchesQuery(d, q)) : [...downloads];
  if (view.sort !== "default") matched.sort(COMPARE[view.sort]);
  return matched;
}

export function hasActiveFilters(view: ListView): boolean {
  return view.query.trim() !== "" || view.tab !== "all";
}

export type ListViewState = "empty" | "no-results" | "list";

/** What the list should show: nothing yet, a search that hid everything, or rows. */
export function listViewState(total: number, shown: number, view: ListView): ListViewState {
  if (shown > 0) return "list";
  return total > 0 && view.query.trim() !== "" ? "no-results" : "empty";
}

function defaultStorage(): Storage | undefined {
  try {
    return window.localStorage;
  } catch {
    return undefined;
  }
}

/** Reads the saved view; any failure or invalid value falls back to defaults. */
export function loadView(storage: Storage | undefined = defaultStorage()): ListView {
  try {
    const raw = storage?.getItem(STORAGE_KEY);
    if (!raw) return DEFAULT_VIEW;
    const p = JSON.parse(raw) as Partial<ListView>;
    return {
      query: typeof p.query === "string" ? p.query : DEFAULT_VIEW.query,
      tab: VIEW_TABS.includes(p.tab as ViewTab) ? (p.tab as ViewTab) : DEFAULT_VIEW.tab,
      sort: SORT_KEYS.includes(p.sort as SortKey) ? (p.sort as SortKey) : DEFAULT_VIEW.sort,
    };
  } catch {
    return DEFAULT_VIEW;
  }
}

export function saveView(view: ListView, storage: Storage | undefined = defaultStorage()): void {
  try {
    storage?.setItem(STORAGE_KEY, JSON.stringify(view));
  } catch {
    /* storage blocked: the view just won't persist */
  }
}
