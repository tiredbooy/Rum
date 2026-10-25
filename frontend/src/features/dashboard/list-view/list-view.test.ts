import { describe, expect, it } from "vitest";
import type { Download } from "@/_lib/types/download-types";
import {
  DEFAULT_VIEW,
  filterAndSort,
  hasActiveFilters,
  listViewState,
  loadView,
  saveView,
} from "./list-view";

const mk = (o: Partial<Download>): Download => ({
  id: o.id ?? "x",
  url: "https://host/file.bin",
  status: "running",
  downloaded: 0,
  remaining: 0,
  ...o,
});

const rows = [
  mk({ id: "a", filename: "Zebra.zip", total_size: 300, progress: 10, created_at: "2025-01-01T00:00:00Z" }),
  mk({ id: "b", filename: "apple.iso", total_size: 100, progress: 90, created_at: "2025-03-01T00:00:00Z" }),
  mk({ id: "c", url: "https://cdn.example.com/movie.mkv", total_size: 200, progress: 50, created_at: "2025-02-01T00:00:00Z" }),
];
const ids = (l: Download[]) => l.map((d) => d.id);

describe("filterAndSort", () => {
  it("keeps server order by default", () => {
    expect(ids(filterAndSort(rows, DEFAULT_VIEW))).toEqual(["a", "b", "c"]);
  });
  it("searches name and URL, case-insensitively", () => {
    expect(ids(filterAndSort(rows, { ...DEFAULT_VIEW, query: "APPLE" }))).toEqual(["b"]);
    expect(ids(filterAndSort(rows, { ...DEFAULT_VIEW, query: " cdn.example " }))).toEqual(["c"]);
  });
  it("sorts by name (case-insensitive, falling back to URL file name)", () => {
    expect(ids(filterAndSort(rows, { ...DEFAULT_VIEW, sort: "name" }))).toEqual(["b", "c", "a"]);
  });
  it("sorts by size, progress (largest first) and date added (newest first)", () => {
    expect(ids(filterAndSort(rows, { ...DEFAULT_VIEW, sort: "size" }))).toEqual(["a", "c", "b"]);
    expect(ids(filterAndSort(rows, { ...DEFAULT_VIEW, sort: "progress" }))).toEqual(["b", "c", "a"]);
    expect(ids(filterAndSort(rows, { ...DEFAULT_VIEW, sort: "added" }))).toEqual(["b", "c", "a"]);
  });
  it("does not mutate its input", () => {
    const copy = [...rows];
    filterAndSort(rows, { ...DEFAULT_VIEW, sort: "name" });
    expect(rows).toEqual(copy);
  });
});

describe("listViewState", () => {
  it("is empty when there is nothing and no search", () => {
    expect(listViewState(0, 0, DEFAULT_VIEW)).toBe("empty");
  });
  it("is no-results when a search hides everything", () => {
    expect(listViewState(3, 0, { ...DEFAULT_VIEW, query: "zzz" })).toBe("no-results");
  });
  it("is list otherwise", () => {
    expect(listViewState(3, 2, { ...DEFAULT_VIEW, query: "a" })).toBe("list");
  });
});

describe("hasActiveFilters", () => {
  it("is false for defaults and true for query or non-all tab", () => {
    expect(hasActiveFilters(DEFAULT_VIEW)).toBe(false);
    expect(hasActiveFilters({ ...DEFAULT_VIEW, query: "a" })).toBe(true);
    expect(hasActiveFilters({ ...DEFAULT_VIEW, tab: "failed" })).toBe(true);
  });
});

function fakeStorage(init: Record<string, string> = {}, broken = false) {
  const data = { ...init };
  return {
    getItem: (k: string) => {
      if (broken) throw new Error("denied");
      return data[k] ?? null;
    },
    setItem: (k: string, v: string) => {
      if (broken) throw new Error("denied");
      data[k] = v;
    },
  } as unknown as Storage;
}

describe("view persistence", () => {
  it("round-trips", () => {
    const s = fakeStorage();
    saveView({ query: "x", tab: "failed", sort: "size" }, s);
    expect(loadView(s)).toEqual({ query: "x", tab: "failed", sort: "size" });
  });
  it("falls back to defaults on corrupt or invalid data", () => {
    expect(loadView(fakeStorage({ "rum.listView": "{nope" }))).toEqual(DEFAULT_VIEW);
    expect(loadView(fakeStorage({ "rum.listView": '{"tab":"bogus","sort":7,"query":3}' }))).toEqual(DEFAULT_VIEW);
  });
  it("never throws when storage is unavailable", () => {
    const s = fakeStorage({}, true);
    expect(loadView(s)).toEqual(DEFAULT_VIEW);
    expect(() => saveView(DEFAULT_VIEW, s)).not.toThrow();
  });
});
