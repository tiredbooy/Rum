# Rum — Drag-&-Drop, Wildcard Ranges, Lighter RAM — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add IDM-style drag-and-drop link capture and wildcard `*` part-range expansion to Rum, and cut idle RAM, without changing the look/feel.

**Architecture:** Frontend (React 19 + Vite + Tailwind v4): a global drop handler extracts links from browser drags and opens the existing Add dialog (Batch tab) pre-filled; a pure wildcard module expands `*` URLs into a numbered range and submits via the existing batch endpoint; RAM cuts are dev-tools gating, route code-splitting, dependency removal, and SSE idle-pause. Backend (Go): GC tuning + an idle-gated `FreeOSMemory` controller so RSS drops when no downloads run.

**Tech Stack:** Go 1.x + Wails v2.12.0, React 19, Vite 7, Tailwind v4, zustand, @tanstack/react-query, Vitest (added here for pure-logic tests).

## Global Constraints

- Target branch: `dev`. Commit per task. **No `Co-Authored-By: Claude` trailer** in any commit (Rum convention).
- Wails file-drop interception stays **off** — do **not** add `EnableFileDrop`/`DragAndDrop` to `options.App` in `main.go`.
- Keep framer-motion as a dependency; do not remove it. Use CSS only where trivially equivalent.
- Do **not** migrate to Wails v3.
- Reuse the existing `POST /api/v1/downloads/batch` endpoint and `useCreateBatch()` — no backend API changes for drag-drop or wildcard.
- Frontend import alias: `@/` → `frontend/src/`.
- Build/verify commands: `cd frontend && npm run build` (runs `tsc && vite build`); `go build ./...`; `go test ./...`. Full GUI build (for RAM measurement): `wails build -tags webkit2_41`.
- Wildcard expansion hard cap: **1000** generated URLs.

---

## File Structure

**New (frontend):**
- `frontend/src/features/drag-drop/extract-links.ts` — pure link extraction + smart classification.
- `frontend/src/features/drag-drop/extract-links.test.ts` — Vitest unit tests.
- `frontend/src/features/drag-drop/DropProvider.tsx` — global drop listeners + overlay.
- `frontend/src/features/wildcard/wildcard.ts` — pure detection + range expansion.
- `frontend/src/features/wildcard/wildcard.test.ts` — Vitest unit tests.
- `frontend/src/features/wildcard/WildcardExpander.tsx` — inline From/To/pad/preview panel.
- `frontend/src/stores/add-dialog-store.ts` — Add-dialog open-state + dropped-URL handoff.
- `frontend/vitest.config.ts` — test config.

**Modified (frontend):**
- `frontend/package.json` — add Vitest; remove unused `react-magic-ui`.
- `frontend/src/App.tsx` — mount `DropProvider`; gate `ReactQueryDevtools` to dev.
- `frontend/src/router.tsx` — lazy-load routes.
- `frontend/src/pages/Dashboard.tsx`, `features/dashboard/Toolbar.tsx`, `features/dashboard/create-download/DownloadDialog.tsx`, `DownloadDialogTabs.tsx` — use the store for open-state + initial tab.
- `frontend/src/features/dashboard/create-download/BatchDownloadForm.tsx` — consume dropped URLs, smart-filter toggle, "expand `*` lines".
- `frontend/src/features/dashboard/create-download/SingleDownloadForm.tsx` — wildcard detection → expander.
- `frontend/src/hooks/useAllProgressStream.tsx` — SSE idle-pause.
- `frontend/src/_lib/queryClient.ts` — `gcTime`.

**New / modified (backend):**
- `backend/internal/pkg/download/memory.go` (new) — `shouldFreeOSMemory`, idle memory controller.
- `backend/internal/pkg/download/memory_test.go` (new) — controller decision test.
- `main.go` — `debug.SetGCPercent` / `SetMemoryLimit`; start/stop the controller.

---

## Execution Order (dependencies)

1. **Task 1** (Vitest setup) → enables Tasks 2 and 5 tests.
2. **Task 3** (add-dialog-store) → required by Tasks 4, 6, 8.
3. **Task 2** (extract-links) → required by Task 4.
4. **Task 5** (wildcard) → required by Task 7.
5. Tasks **2, 3, 5, 9** are file-disjoint and may run in parallel after Task 1.
6. Tasks **4, 6, 7, 8** (UI wiring) run after their deps; file-disjoint among themselves except as noted.
7. **Task 10** (frontend RAM) and **Task 11** (batch `*`) after the UI tasks; **Task 12** (verify) last.

---

### Task 1: Add Vitest for pure-logic tests

**Files:**
- Modify: `frontend/package.json`
- Create: `frontend/vitest.config.ts`

- [ ] **Step 1: Add Vitest dev dependency**

Run:
```bash
cd frontend && npm install -D vitest@^2.1.0
```

- [ ] **Step 2: Add the test script**

In `frontend/package.json`, add to `"scripts"`:
```json
"test": "vitest run",
"test:watch": "vitest"
```

- [ ] **Step 3: Create `frontend/vitest.config.ts`**

```ts
import path from "path";
import { defineConfig } from "vitest/config";

export default defineConfig({
  resolve: {
    alias: { "@": path.resolve(__dirname, "./src") },
  },
  test: {
    environment: "node",
    include: ["src/**/*.test.ts"],
  },
});
```

- [ ] **Step 4: Verify the runner works (no tests yet)**

Run: `cd frontend && npx vitest run`
Expected: exits 0 with "No test files found" (or runs 0 files). Not an error.

- [ ] **Step 5: Commit**

```bash
git add frontend/package.json frontend/package-lock.json frontend/vitest.config.ts
git commit -m "test(frontend): add Vitest for pure-logic unit tests"
```

---

### Task 2: Link extraction + smart classification (pure)

**Files:**
- Create: `frontend/src/features/drag-drop/extract-links.ts`
- Test: `frontend/src/features/drag-drop/extract-links.test.ts`

**Interfaces:**
- Produces:
  - `extractUrlsFromDataTransfer(dt: DataTransfer): string[]`
  - `extractUrlsFromStrings(input: { uriList?: string; html?: string; plain?: string }): string[]` (the pure core, used by tests without a real `DataTransfer`)
  - `classifyLinks(urls: string[]): { all: string[]; downloadable: string[] }`

- [ ] **Step 1: Write the failing tests**

`frontend/src/features/drag-drop/extract-links.test.ts`:
```ts
import { describe, expect, it } from "vitest";
import { classifyLinks, extractUrlsFromStrings } from "./extract-links";

describe("extractUrlsFromStrings", () => {
  it("parses text/uri-list, ignoring comment lines", () => {
    const urls = extractUrlsFromStrings({
      uriList: "# comment\nhttps://x.com/a.zip\nhttps://x.com/b.mp4\n",
    });
    expect(urls).toEqual(["https://x.com/a.zip", "https://x.com/b.mp4"]);
  });

  it("extracts hrefs and media src from HTML", () => {
    const urls = extractUrlsFromStrings({
      html: `<a href="https://x.com/a.rar">a</a><img src="https://x.com/i.png"><video src="https://x.com/v.mp4">`,
    });
    expect(urls).toContain("https://x.com/a.rar");
    expect(urls).toContain("https://x.com/i.png");
    expect(urls).toContain("https://x.com/v.mp4");
  });

  it("falls back to http(s) URLs in plain text", () => {
    const urls = extractUrlsFromStrings({
      plain: "see https://x.com/a.iso and http://y.com/b.bin here",
    });
    expect(urls).toEqual(["https://x.com/a.iso", "http://y.com/b.bin"]);
  });

  it("dedupes preserving order and prefers uri-list over html", () => {
    const urls = extractUrlsFromStrings({
      uriList: "https://x.com/a.zip",
      html: `<a href="https://x.com/a.zip">dup</a><a href="https://x.com/c.7z">c</a>`,
    });
    expect(urls).toEqual(["https://x.com/a.zip", "https://x.com/c.7z"]);
  });
});

describe("classifyLinks", () => {
  it("separates downloadable file links from page-like links", () => {
    const { all, downloadable } = classifyLinks([
      "https://x.com/game.part1.rar",
      "https://x.com/movie.mkv",
      "https://x.com/page.html",
      "https://x.com/about",
      "https://x.com/search?q=1",
    ]);
    expect(all).toHaveLength(5);
    expect(downloadable).toEqual([
      "https://x.com/game.part1.rar",
      "https://x.com/movie.mkv",
    ]);
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd frontend && npx vitest run src/features/drag-drop/extract-links.test.ts`
Expected: FAIL — cannot find module `./extract-links`.

- [ ] **Step 3: Implement `extract-links.ts`**

```ts
// Pure link extraction from browser drag-and-drop payloads, plus a smart
// downloadable/page classifier. Kept dependency-free and DOM-light so it is
// unit-testable in a node environment (HTML parsing uses DOMParser only when
// a real DataTransfer is read in the browser; the string core accepts html
// and parses it with a tiny regex fallback when DOMParser is absent).

const HTTP_URL_RE = /\bhttps?:\/\/[^\s"'<>)\]]+/gi;
const MAX_ANCHORS = 5000;

// File extensions we treat as directly downloadable.
const DOWNLOADABLE_EXT = new Set([
  "zip", "rar", "7z", "tar", "gz", "bz2", "xz", "tgz",
  "mp4", "mkv", "avi", "mov", "webm", "flv", "m4v",
  "mp3", "flac", "wav", "m4a", "aac", "ogg",
  "iso", "img", "bin",
  "pdf", "epub", "apk", "exe", "msi", "dmg", "deb", "rpm", "appimage",
]);
// part1/part2... style archive segments.
const PART_RE = /\.part\d+$/i;
// Page-like extensions that are never "downloadable files".
const PAGE_EXT = new Set(["html", "htm", "php", "asp", "aspx", "jsp", "do"]);

function dedupe(urls: string[]): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const u of urls) {
    const t = u.trim();
    if (!t || seen.has(t)) continue;
    seen.add(t);
    out.push(t);
  }
  return out;
}

function fromUriList(uriList: string): string[] {
  return uriList
    .split(/\r?\n/)
    .map((l) => l.trim())
    .filter((l) => l && !l.startsWith("#"));
}

function fromPlain(plain: string): string[] {
  return plain.match(HTTP_URL_RE) ?? [];
}

function fromHtml(html: string): string[] {
  // Prefer a real DOM parser when available (browser); fall back to regex in node.
  if (typeof DOMParser !== "undefined") {
    const doc = new DOMParser().parseFromString(html, "text/html");
    const out: string[] = [];
    const push = (v: string | null) => {
      if (v && /^https?:\/\//i.test(v)) out.push(v);
    };
    const anchors = Array.from(doc.querySelectorAll("a[href]")).slice(0, MAX_ANCHORS);
    anchors.forEach((a) => push(a.getAttribute("href")));
    doc.querySelectorAll("img[src],video[src],source[src],[data-href]").forEach((el) => {
      push(el.getAttribute("src"));
      push(el.getAttribute("data-href"));
    });
    return out;
  }
  return html.match(HTTP_URL_RE) ?? [];
}

export function extractUrlsFromStrings(input: {
  uriList?: string;
  html?: string;
  plain?: string;
}): string[] {
  const out: string[] = [];
  if (input.uriList) out.push(...fromUriList(input.uriList));
  if (input.html) out.push(...fromHtml(input.html));
  if (input.plain) out.push(...fromPlain(input.plain));
  return dedupe(out);
}

export function extractUrlsFromDataTransfer(dt: DataTransfer): string[] {
  return extractUrlsFromStrings({
    uriList: dt.getData("text/uri-list") || undefined,
    html: dt.getData("text/html") || undefined,
    plain: dt.getData("text/plain") || undefined,
  });
}

function extOf(url: string): string {
  try {
    const path = new URL(url).pathname;
    const base = path.split("/").pop() ?? "";
    const dot = base.lastIndexOf(".");
    return dot >= 0 ? base.slice(dot + 1).toLowerCase() : "";
  } catch {
    return "";
  }
}

function isDownloadable(url: string): boolean {
  if (PART_RE.test(new URL(url).pathname)) return true;
  const ext = extOf(url);
  if (!ext || PAGE_EXT.has(ext)) return false;
  return DOWNLOADABLE_EXT.has(ext);
}

export function classifyLinks(urls: string[]): {
  all: string[];
  downloadable: string[];
} {
  const all = dedupe(urls.filter((u) => /^https?:\/\//i.test(u)));
  const downloadable = all.filter((u) => {
    try {
      return isDownloadable(u);
    } catch {
      return false;
    }
  });
  return { all, downloadable };
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd frontend && npx vitest run src/features/drag-drop/extract-links.test.ts`
Expected: PASS (all tests).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/features/drag-drop/extract-links.ts frontend/src/features/drag-drop/extract-links.test.ts
git commit -m "feat(drag-drop): pure link extraction + smart classification"
```

---

### Task 3: Add-dialog store (open-state + dropped-URL handoff)

**Files:**
- Create: `frontend/src/stores/add-dialog-store.ts`
- Modify: `frontend/src/pages/Dashboard.tsx`, `frontend/src/features/dashboard/create-download/DownloadDialog.tsx`, `frontend/src/features/dashboard/create-download/DownloadDialogTabs.tsx`

**Interfaces:**
- Produces (store):
  - `useAddDialogStore` with state `{ open: boolean; initialTab: AddTab; droppedUrls: string[] | null }`
  - actions `openWith(opts?: { tab?: AddTab; urls?: string[] }): void`, `setOpen(open: boolean): void`, `consumeDropped(): string[] | null`
  - `type AddTab = "single" | "bulk" | "url" | "batch"`

- [ ] **Step 1: Create `frontend/src/stores/add-dialog-store.ts`**

```ts
import { create } from "zustand";

export type AddTab = "single" | "bulk" | "url" | "batch";

interface AddDialogState {
  open: boolean;
  initialTab: AddTab;
  droppedUrls: string[] | null;
  openWith: (opts?: { tab?: AddTab; urls?: string[] }) => void;
  setOpen: (open: boolean) => void;
  consumeDropped: () => string[] | null;
}

export const useAddDialogStore = create<AddDialogState>((set, get) => ({
  open: false,
  initialTab: "single",
  droppedUrls: null,
  openWith: (opts) =>
    set({
      open: true,
      initialTab: opts?.tab ?? "single",
      droppedUrls: opts?.urls ?? null,
    }),
  setOpen: (open) => set({ open }),
  consumeDropped: () => {
    const urls = get().droppedUrls;
    if (urls) set({ droppedUrls: null });
    return urls;
  },
}));
```

- [ ] **Step 2: Make `DownloadDialog` and tabs read the store**

In `DownloadDialog.tsx`, replace the `open`/`setOpen` props with the store. New file body:
```tsx
import { Button } from "@/components/ui/button";
import { ResponsiveDialog } from "@/features/reusable/dialog/DialogShell";
import { DownloadTabs } from "./DownloadDialogTabs";
import { useDownloadRequestStore } from "@/stores/download-request-store";
import { useCreateDownloads } from "@/_lib/services/queries/download.queries";
import { useAddDialogStore } from "@/stores/add-dialog-store";

export default function DownloadDialog() {
  const open = useAddDialogStore((s) => s.open);
  const setOpen = useAddDialogStore((s) => s.setOpen);
  const draft = useDownloadRequestStore((s) => s.draft);
  const saveDownloads = useCreateDownloads();

  const handleSaveDownloads = () => {
    if (draft?.urls?.length) saveDownloads.mutateAsync(draft);
    setOpen(false);
  };

  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={setOpen}
      title="Add Downloads"
      size="2xl"
      description="Choose a method to add downloads."
      footer={
        <div className="flex gap-2 justify-end w-full">
          <Button variant="outline" onClick={() => setOpen(false)}>
            Cancel
          </Button>
          <Button onClick={handleSaveDownloads}>Save</Button>
        </div>
      }
    >
      <DownloadTabs />
    </ResponsiveDialog>
  );
}
```

In `DownloadDialogTabs.tsx`, seed `activeTab` from the store's `initialTab`:
```tsx
import { useAddDialogStore } from "@/stores/add-dialog-store";
// ...
export function DownloadTabs() {
  const initialTab = useAddDialogStore((s) => s.initialTab);
  const [activeTab, setActiveTab] = useState(initialTab);
  useEffect(() => setActiveTab(initialTab), [initialTab]);
  // ...rest unchanged
}
```
(Add `useEffect` to the existing `import { useState } from "react"`.)

- [ ] **Step 3: Update `Dashboard.tsx` to drive the dialog from the store**

In `Dashboard.tsx`: remove the local `open`/`setOpen` state for the dialog; render `<DownloadDialog />` with no props; change the "Add Download" trigger to call `useAddDialogStore.getState().openWith()` (or a bound `openWith`). Concretely:
```tsx
import { useAddDialogStore } from "@/stores/add-dialog-store";
// inside component:
const openWith = useAddDialogStore((s) => s.openWith);
// the Toolbar onAddDownload handler:
onAddDownload={() => openWith()}
// at the bottom, replace <DownloadDialog open={open} setOpen={setOpen} /> with:
<DownloadDialog />
```
(If `Toolbar` is passed `onAddDownload`, keep that prop and wire it to `openWith()`. Read the current `Dashboard.tsx` to preserve all other wiring.)

- [ ] **Step 4: Typecheck**

Run: `cd frontend && npx tsc --noEmit`
Expected: no errors related to these files.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/stores/add-dialog-store.ts frontend/src/pages/Dashboard.tsx frontend/src/features/dashboard/create-download/DownloadDialog.tsx frontend/src/features/dashboard/create-download/DownloadDialogTabs.tsx
git commit -m "refactor(add-dialog): drive open-state + initial tab from a store"
```

---

### Task 4: BatchDownloadForm — consume dropped URLs + smart-filter toggle

**Files:**
- Modify: `frontend/src/features/dashboard/create-download/BatchDownloadForm.tsx`

**Interfaces:**
- Consumes: `useAddDialogStore().consumeDropped()` (Task 3), `classifyLinks` (Task 2).

- [ ] **Step 1: Seed from dropped URLs + add the toggle**

Add to the top of `BatchDownloadForm`:
```tsx
import { useEffect } from "react";
import { useAddDialogStore } from "@/stores/add-dialog-store";
import { classifyLinks } from "@/features/drag-drop/extract-links";
```
Add state + an effect that runs once when the form mounts with dropped URLs:
```tsx
const consumeDropped = useAddDialogStore((s) => s.consumeDropped);
const open = useAddDialogStore((s) => s.open);
const [showAll, setShowAll] = useState(false);
const [dropped, setDropped] = useState<{ all: string[]; downloadable: string[] } | null>(null);

useEffect(() => {
  if (!open) return;
  const urls = consumeDropped();
  if (urls && urls.length) {
    const classified = classifyLinks(urls);
    setDropped(classified);
    setRawText((classified.downloadable.length ? classified.downloadable : classified.all).join("\n"));
    setShowAll(classified.downloadable.length === 0);
  }
}, [open, consumeDropped]);

// When the toggle flips and we have a drop set, swap the textarea contents.
useEffect(() => {
  if (!dropped) return;
  setRawText((showAll ? dropped.all : dropped.downloadable).join("\n"));
}, [showAll, dropped]);
```
Render the toggle above the URLs textarea (only when `dropped` exists):
```tsx
{dropped && (
  <label className="flex items-center gap-2 text-xs text-muted-foreground">
    <input type="checkbox" checked={showAll} onChange={(e) => setShowAll(e.target.checked)} />
    Show all links ({dropped.all.length}) · downloadable only ({dropped.downloadable.length})
  </label>
)}
```

- [ ] **Step 2: Typecheck**

Run: `cd frontend && npx tsc --noEmit`
Expected: no new errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/features/dashboard/create-download/BatchDownloadForm.tsx
git commit -m "feat(batch): seed from dropped links with downloadable/all filter toggle"
```

---

### Task 5: Wildcard detection + range expansion (pure)

**Files:**
- Create: `frontend/src/features/wildcard/wildcard.ts`
- Test: `frontend/src/features/wildcard/wildcard.test.ts`

**Interfaces:**
- Produces:
  - `hasWildcard(url: string): boolean`
  - `WILDCARD_MAX = 1000`
  - `validateRange(r: { from: number; to: number; pad: number }): string | null` (returns an error message or `null`)
  - `expandWildcard(url: string, r: { from: number; to: number; pad: number }): string[]`

- [ ] **Step 1: Write the failing tests**

`frontend/src/features/wildcard/wildcard.test.ts`:
```ts
import { describe, expect, it } from "vitest";
import { expandWildcard, hasWildcard, validateRange, WILDCARD_MAX } from "./wildcard";

describe("hasWildcard", () => {
  it("detects a star", () => {
    expect(hasWildcard("https://x.com/file*.rar")).toBe(true);
    expect(hasWildcard("https://x.com/file1.rar")).toBe(false);
  });
});

describe("validateRange", () => {
  it("rejects to < from", () => {
    expect(validateRange({ from: 5, to: 2, pad: 0 })).toMatch(/from/i);
  });
  it("rejects counts over the cap", () => {
    expect(validateRange({ from: 1, to: WILDCARD_MAX + 1, pad: 0 })).toMatch(/1000/);
  });
  it("accepts a valid range", () => {
    expect(validateRange({ from: 1, to: 12, pad: 2 })).toBeNull();
  });
});

describe("expandWildcard", () => {
  it("expands without padding", () => {
    expect(expandWildcard("https://x.com/f*.zip", { from: 1, to: 3, pad: 0 })).toEqual([
      "https://x.com/f1.zip",
      "https://x.com/f2.zip",
      "https://x.com/f3.zip",
    ]);
  });
  it("expands with zero-padding", () => {
    expect(expandWildcard("https://x.com/game.part*.rar", { from: 1, to: 2, pad: 2 })).toEqual([
      "https://x.com/game.part01.rar",
      "https://x.com/game.part02.rar",
    ]);
  });
  it("only replaces the first star", () => {
    expect(expandWildcard("https://x.com/*/f*.zip", { from: 1, to: 1, pad: 0 })).toEqual([
      "https://x.com/1/f*.zip",
    ]);
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd frontend && npx vitest run src/features/wildcard/wildcard.test.ts`
Expected: FAIL — cannot find module `./wildcard`.

- [ ] **Step 3: Implement `wildcard.ts`**

```ts
// Pure IDM-style wildcard expansion: replace the first `*` in a URL with each
// integer in [from, to], optionally zero-padded. Capped to avoid runaway lists.

export const WILDCARD_MAX = 1000;

export interface WildcardRange {
  from: number;
  to: number;
  pad: number; // 0 = no padding
}

export function hasWildcard(url: string): boolean {
  return url.includes("*");
}

export function validateRange(r: WildcardRange): string | null {
  if (!Number.isFinite(r.from) || !Number.isFinite(r.to)) return "From and To must be numbers";
  if (r.from < 0 || r.to < 0) return "From and To must be ≥ 0";
  if (r.to < r.from) return "To must be ≥ From";
  if (r.pad < 0 || r.pad > 12) return "Zero-pad digits must be 0–12";
  if (r.to - r.from + 1 > WILDCARD_MAX) return `Range too large (max ${WILDCARD_MAX})`;
  return null;
}

export function expandWildcard(url: string, r: WildcardRange): string[] {
  const err = validateRange(r);
  if (err) throw new Error(err);
  if (!hasWildcard(url)) return [url];
  const out: string[] = [];
  for (let n = r.from; n <= r.to; n++) {
    const num = r.pad > 0 ? String(n).padStart(r.pad, "0") : String(n);
    out.push(url.replace("*", num));
  }
  return out;
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd frontend && npx vitest run src/features/wildcard/wildcard.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/features/wildcard/wildcard.ts frontend/src/features/wildcard/wildcard.test.ts
git commit -m "feat(wildcard): pure detection + range expansion with zero-padding"
```

---

### Task 6: WildcardExpander component

**Files:**
- Create: `frontend/src/features/wildcard/WildcardExpander.tsx`

**Interfaces:**
- Consumes: `expandWildcard`, `validateRange`, `WildcardRange` (Task 5); `useCreateBatch` from `@/_lib/services/queries/download.queries`; `BatchOptions` from `@/_lib/types/download-types`.
- Produces: `WildcardExpander({ url, options?, onDone? }: { url: string; options?: BatchOptions; onDone?: () => void })`

- [ ] **Step 1: Implement `WildcardExpander.tsx`**

```tsx
import { useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Loader2, Send } from "lucide-react";
import { expandWildcard, validateRange, type WildcardRange } from "./wildcard";
import { useCreateBatch } from "@/_lib/services/queries/download.queries";
import type { BatchOptions } from "@/_lib/types/download-types";

export function WildcardExpander({
  url,
  options,
  onDone,
}: {
  url: string;
  options?: BatchOptions;
  onDone?: () => void;
}) {
  const createBatch = useCreateBatch();
  const [range, setRange] = useState<WildcardRange>({ from: 1, to: 10, pad: 0 });

  const error = validateRange(range);
  const preview = useMemo(() => {
    if (error) return null;
    try {
      const urls = expandWildcard(url, range);
      return { count: urls.length, first: urls[0], last: urls[urls.length - 1] };
    } catch {
      return null;
    }
  }, [url, range, error]);

  const set = (k: keyof WildcardRange) => (e: React.ChangeEvent<HTMLInputElement>) =>
    setRange((r) => ({ ...r, [k]: Number(e.target.value) }));

  const handleAdd = () => {
    if (error) return;
    const urls = expandWildcard(url, range);
    createBatch.mutate({ urls, options }, { onSuccess: () => onDone?.() });
  };

  return (
    <div className="space-y-3 rounded-md border border-border p-3">
      <p className="text-sm font-medium">Wildcard detected — expand range</p>
      <p className="text-xs text-muted-foreground break-all">{url}</p>
      <div className="grid grid-cols-3 gap-2">
        <div className="space-y-1">
          <Label className="text-xs">From</Label>
          <Input type="number" min={0} value={range.from} onChange={set("from")} />
        </div>
        <div className="space-y-1">
          <Label className="text-xs">To</Label>
          <Input type="number" min={0} value={range.to} onChange={set("to")} />
        </div>
        <div className="space-y-1">
          <Label className="text-xs">Zero-pad digits</Label>
          <Input type="number" min={0} max={12} value={range.pad} onChange={set("pad")} />
        </div>
      </div>
      {error ? (
        <p className="text-xs text-destructive">{error}</p>
      ) : preview ? (
        <div className="text-xs text-muted-foreground font-mono">
          <div>{preview.first}</div>
          <div>… {preview.count} files …</div>
          <div>{preview.last}</div>
        </div>
      ) : null}
      <Button onClick={handleAdd} disabled={!!error || createBatch.isPending} className="w-full gap-2">
        {createBatch.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
        Add {preview ? preview.count : 0} download{preview?.count !== 1 ? "s" : ""}
      </Button>
    </div>
  );
}

export default WildcardExpander;
```

- [ ] **Step 2: Typecheck**

Run: `cd frontend && npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/features/wildcard/WildcardExpander.tsx
git commit -m "feat(wildcard): From/To/pad expander with live preview"
```

---

### Task 7: Wire wildcard into the Single tab

**Files:**
- Modify: `frontend/src/features/dashboard/create-download/SingleDownloadForm.tsx`

**Interfaces:**
- Consumes: `hasWildcard` (Task 5), `WildcardExpander` (Task 6).

- [ ] **Step 1: Read the current Single form to find its URL field state**

Run: `sed -n '1,120p' frontend/src/features/dashboard/create-download/SingleDownloadForm.tsx`
Identify the state variable holding the URL input (e.g. `url`) and the dest/options if any.

- [ ] **Step 2: Render the expander when the URL contains `*`**

Add imports:
```tsx
import { hasWildcard } from "@/features/wildcard/wildcard";
import { WildcardExpander } from "@/features/wildcard/WildcardExpander";
import { useAddDialogStore } from "@/stores/add-dialog-store";
```
Below the URL input, conditionally render (replace `urlValue` with the actual state var found in Step 1; pass `dest_path` into options if the form has a destination field):
```tsx
{hasWildcard(urlValue) && (
  <WildcardExpander
    url={urlValue.trim()}
    options={destPath?.trim() ? { dest_path: destPath.trim() } : undefined}
    onDone={() => useAddDialogStore.getState().setOpen(false)}
  />
)}
```

- [ ] **Step 3: Typecheck**

Run: `cd frontend && npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/features/dashboard/create-download/SingleDownloadForm.tsx
git commit -m "feat(single): show wildcard range expander when URL contains *"
```

---

### Task 8: Global drop handler + overlay

**Files:**
- Create: `frontend/src/features/drag-drop/DropProvider.tsx`
- Modify: `frontend/src/App.tsx`

**Interfaces:**
- Consumes: `extractUrlsFromDataTransfer`, `classifyLinks` (Task 2); `useAddDialogStore().openWith` (Task 3); `toast` from `@/lib/toast` (used elsewhere in the codebase).

- [ ] **Step 1: Implement `DropProvider.tsx`**

```tsx
import { useEffect, useState } from "react";
import { extractUrlsFromDataTransfer } from "./extract-links";
import { useAddDialogStore } from "@/stores/add-dialog-store";
import { toast } from "@/lib/toast";

// Global browser-drag capture. Listens at the window level, prevents the
// webview from navigating to a dropped URL, extracts links, and opens the Add
// dialog (Batch tab) pre-filled. Renders only a drag overlay; no layout impact.
export function DropProvider() {
  const openWith = useAddDialogStore((s) => s.openWith);
  const [dragging, setDragging] = useState(false);

  useEffect(() => {
    let depth = 0;
    const onEnter = (e: DragEvent) => {
      e.preventDefault();
      depth++;
      setDragging(true);
    };
    const onOver = (e: DragEvent) => {
      e.preventDefault();
    };
    const onLeave = (e: DragEvent) => {
      e.preventDefault();
      depth = Math.max(0, depth - 1);
      if (depth === 0) setDragging(false);
    };
    const onDrop = (e: DragEvent) => {
      e.preventDefault();
      depth = 0;
      setDragging(false);
      if (!e.dataTransfer) return;
      const urls = extractUrlsFromDataTransfer(e.dataTransfer);
      if (!urls.length) {
        toast.error?.("No links found in the dropped content.");
        return;
      }
      openWith({ tab: "batch", urls });
    };
    window.addEventListener("dragenter", onEnter);
    window.addEventListener("dragover", onOver);
    window.addEventListener("dragleave", onLeave);
    window.addEventListener("drop", onDrop);
    return () => {
      window.removeEventListener("dragenter", onEnter);
      window.removeEventListener("dragover", onOver);
      window.removeEventListener("dragleave", onLeave);
      window.removeEventListener("drop", onDrop);
    };
  }, [openWith]);

  if (!dragging) return null;
  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center bg-background/70 backdrop-blur-sm pointer-events-none">
      <div className="rounded-xl border-2 border-dashed border-primary px-8 py-6 text-lg font-medium text-primary">
        Drop links to add to Rum
      </div>
    </div>
  );
}

export default DropProvider;
```
(If `@/lib/toast` does not export `error`, use the project's existing toast call style found in `download.queries.ts`.)

- [ ] **Step 2: Mount it in `App.tsx`**

Add `import DropProvider from "@/features/drag-drop/DropProvider";` and render `<DropProvider />` inside the providers (e.g. just inside `QueryClientProvider`, alongside `AllProgressStream`).

- [ ] **Step 3: Typecheck**

Run: `cd frontend && npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 4: Manual verification (record result)**

Run `cd frontend && npm run dev` (browser) or a full `wails build`. Drag a link from a browser onto the window: overlay appears; on drop the Add dialog opens on Batch pre-filled; the app does **not** navigate away. Note: full cross-OS (WebKitGTK + WebView2) check happens in Task 12.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/features/drag-drop/DropProvider.tsx frontend/src/App.tsx
git commit -m "feat(drag-drop): global browser-link drop -> prefilled Batch dialog"
```

---

### Task 9: Backend — idle-gated FreeOSMemory controller

**Files:**
- Create: `backend/internal/pkg/download/memory.go`
- Test: `backend/internal/pkg/download/memory_test.go`

**Interfaces:**
- Produces:
  - `func shouldFreeOSMemory(activeCount int) bool`
  - `func StartMemoryController(ctx context.Context, activeCount func() int, interval time.Duration)`

- [ ] **Step 1: Write the failing test**

`backend/internal/pkg/download/memory_test.go`:
```go
package download

import "testing"

func TestShouldFreeOSMemory(t *testing.T) {
	if !shouldFreeOSMemory(0) {
		t.Fatal("idle (0 active) should free OS memory")
	}
	if shouldFreeOSMemory(1) {
		t.Fatal("with active downloads it should not force-free")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./backend/internal/pkg/download/ -run TestShouldFreeOSMemory`
Expected: FAIL — `shouldFreeOSMemory` undefined.

- [ ] **Step 3: Implement `memory.go`**

```go
package download

import (
	"context"
	"runtime/debug"
	"time"
)

// shouldFreeOSMemory reports whether the process is idle enough to force the Go
// runtime to return freed heap pages to the OS. Go releases memory lazily
// (MADV_FREE), so RSS stays high after a big download unless we force it.
func shouldFreeOSMemory(activeCount int) bool {
	return activeCount == 0
}

// StartMemoryController periodically returns idle memory to the OS. It only
// forces a release when no downloads are active, so it never competes with a
// running transfer. It stops when ctx is cancelled.
func StartMemoryController(ctx context.Context, activeCount func() int, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if shouldFreeOSMemory(activeCount()) {
					debug.FreeOSMemory()
				}
			}
		}
	}()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./backend/internal/pkg/download/ -run TestShouldFreeOSMemory`
Expected: PASS.

- [ ] **Step 5: Add an active-count helper on the manager**

Read `backend/internal/pkg/download/manager.go` (the stats code near line 944 counts `StatusRunning`). Add an exported method that reuses `GetAllJobs()`:
```go
// ActiveCount returns the number of currently-running jobs (used by the idle
// memory controller).
func (m *JobManager) ActiveCount() int {
	n := 0
	for _, j := range m.GetAllJobs() {
		if j.GetStatus() == StatusRunning {
			n++
		}
	}
	return n
}
```

- [ ] **Step 6: Run the package tests**

Run: `go test ./backend/internal/pkg/download/`
Expected: PASS (existing tests + new one).

- [ ] **Step 7: Commit**

```bash
git add backend/internal/pkg/download/memory.go backend/internal/pkg/download/memory_test.go backend/internal/pkg/download/manager.go
git commit -m "feat(engine): idle-gated FreeOSMemory controller + JobManager.ActiveCount"
```

---

### Task 10: Wire GC tuning + controller into the desktop app

**Files:**
- Modify: `main.go`
- Possibly modify: `backend/cmd/server/main.go` (to start the controller where it can see the manager)

**Interfaces:**
- Consumes: `download.StartMemoryController`, `JobManager.ActiveCount` (Task 9).

- [ ] **Step 1: Add GC/memory tuning in `main.go`**

At the very top of `func main()` add:
```go
import "runtime/debug"
// ...
debug.SetGCPercent(50) // collect more often; trade a little CPU for lower RSS
```
(Place `debug.SetGCPercent(50)` as the first statement in `main()`.)

- [ ] **Step 2: Start the controller where the manager is reachable**

Read `backend/cmd/server/main.go` to find where `GlobalManager` (a `*download.JobManager`) is created and where a context is available. After the manager exists and the server starts, start the controller:
```go
download.StartMemoryController(ctx, GlobalManager.ActiveCount, 30*time.Second)
```
If `cmd/server` has no long-lived context, add a package-level `context.Background()`-derived one started in `Serve()`/`Listen()`. Keep it minimal; the controller self-stops on ctx cancel. (The root layer cannot import `backend/internal`, so the controller MUST be started from inside `backend/cmd/server`, which can.)

- [ ] **Step 3: Build**

Run: `go build ./...`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add main.go backend/cmd/server/main.go
git commit -m "perf(engine): SetGCPercent + start idle memory controller"
```

---

### Task 11: Frontend RAM cuts

**Files:**
- Modify: `frontend/src/App.tsx`, `frontend/src/router.tsx`, `frontend/package.json`, `frontend/src/hooks/useAllProgressStream.tsx`, `frontend/src/_lib/queryClient.ts`

- [ ] **Step 1: Gate ReactQueryDevtools to dev only**

In `App.tsx`, replace the static import + element with a dev-only lazy mount:
```tsx
import { lazy, Suspense } from "react";
const ReactQueryDevtools = import.meta.env.DEV
  ? lazy(() => import("@tanstack/react-query-devtools").then((m) => ({ default: m.ReactQueryDevtools })))
  : () => null;
// ...inside QueryClientProvider:
{import.meta.env.DEV && (
  <Suspense fallback={null}>
    <ReactQueryDevtools initialIsOpen={false} />
  </Suspense>
)}
```
Remove the old top-level `import { ReactQueryDevtools } ...` line.

- [ ] **Step 2: Remove the unused `react-magic-ui` dependency**

Run:
```bash
cd frontend && npm uninstall react-magic-ui
```
Then `grep -rn "react-magic-ui" src` → expect no matches.

- [ ] **Step 3: Lazy-load routes in `router.tsx`**

```tsx
import { createBrowserRouter } from "react-router-dom";
import { lazy, Suspense } from "react";
import Layout from "./features/layout/Layout";

const Dashboard = lazy(() => import("./pages/Dashboard"));
const Settings = lazy(() => import("./pages/Settings"));
const ActiveDownloads = lazy(() => import("./pages/ActiveDownloads"));
const CompletedDownloads = lazy(() => import("./pages/CompletedDownloads"));
const FailedDownloads = lazy(() => import("./pages/FailedDownloads"));

const wrap = (el: React.ReactNode) => <Suspense fallback={null}>{el}</Suspense>;

export const router = createBrowserRouter([
  {
    path: "/",
    element: <Layout />,
    children: [
      { index: true, element: wrap(<Dashboard />) },
      { path: "active", element: wrap(<ActiveDownloads />) },
      { path: "completed", element: wrap(<CompletedDownloads />) },
      { path: "failed", element: wrap(<FailedDownloads />) },
      { path: "settings", element: wrap(<Settings />) },
    ],
  },
]);
```
(These page modules use default exports — confirmed in the current `router.tsx`.)

- [ ] **Step 4: SSE idle-pause in `useAllProgressStream.tsx`**

Make the EventSource open only when there is work to watch. Add a subscription to the progress store's active/known downloads (or to the React Query list). Minimal approach: track an `enabled` flag derived from `useProgressStore` active count; gate `connect()` on it and close when it goes to 0. Implementation:
```tsx
// add near the other store reads:
const hasActivity = useProgressStore((s) => s.hasActivity ?? true);
// in the effect, only connect when hasActivity; add hasActivity to deps and
// close the stream + clear reconnect timer when it turns false.
```
If `useProgressStore` has no activity selector, add one: a boolean derived from whether any tracked download is running/pending. Keep the reconnect/backoff for the active case. **Do not** break the connection banner: when paused intentionally, set `online` true (not an error state).

- [ ] **Step 5: Tune React Query gcTime**

In `queryClient.ts` add `gcTime: 60_000` to the `queries` defaults (collect idle/stale caches after a minute):
```ts
queries: { staleTime: 5_000, gcTime: 60_000, refetchOnWindowFocus: true, retry: 2 },
```

- [ ] **Step 6: Build + typecheck**

Run: `cd frontend && npm run build`
Expected: success; bundle no longer contains devtools/react-magic-ui in prod.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/App.tsx frontend/src/router.tsx frontend/package.json frontend/package-lock.json frontend/src/hooks/useAllProgressStream.tsx frontend/src/_lib/queryClient.ts
git commit -m "perf(frontend): dev-only devtools, route splitting, SSE idle-pause, gcTime, drop react-magic-ui"
```

---

### Task 12: Batch tab — expand `*` lines + final integration & verification

**Files:**
- Modify: `frontend/src/features/dashboard/create-download/BatchDownloadForm.tsx`

**Interfaces:**
- Consumes: `hasWildcard`, `expandWildcard`, `WILDCARD_MAX` (Task 5).

- [ ] **Step 1: Add an "Expand `*` lines" action to the Batch form**

When any line contains `*`, show a button that expands each wildcard line (using a single shared From/To/pad — reuse `WildcardExpander`'s inputs or a compact inline prompt) and merges results into the textarea for review. Keep it simple: a button "Expand wildcards" that, for each `*` line, calls `expandWildcard(line, range)` with a small From/To/pad popover; replace those lines with the expansion (respecting `WILDCARD_MAX` total). Guard the total count; toast if exceeded.

- [ ] **Step 2: Typecheck + unit tests**

Run: `cd frontend && npx tsc --noEmit && npx vitest run`
Expected: all green.

- [ ] **Step 3: Full frontend build**

Run: `cd frontend && npm run build`
Expected: success.

- [ ] **Step 4: Full backend build + tests**

Run: `go build ./... && go test ./...`
Expected: success / green.

- [ ] **Step 5: GUI build + RAM measurement**

Run: `wails build -tags webkit2_41` then launch the binary, idle ~60s with 0 downloads, record RSS (`ps -o rss= -p <pid>` / `smem`). Compare against a pre-change baseline if available. Record the number in the commit message / PR.

- [ ] **Step 6: Manual smoke (record results)**

- Drag a single browser link and a multi-link selection → Batch dialog prefilled; no navigation (test on the available OS; note which).
- Single tab: paste `…/game.part*.rar`, From 1 / To 12 / pad 2 → previews `part01`…`part12`; Add creates 12.
- Batch tab: a `*` line expands via "Expand wildcards".
- Add/start/pause/resume/complete a real download; SSE pauses when idle and resumes on activity; connection banner behaves.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/features/dashboard/create-download/BatchDownloadForm.tsx
git commit -m "feat(batch): expand wildcard lines; integration + RAM verification"
```

---

## Self-Review

**Spec coverage:**
- Feature 1 (drag-drop) → Tasks 2 (extract), 3 (store), 4 (batch filter), 8 (drop handler). ✓
- Feature 2 (wildcard) → Tasks 5 (logic), 6 (expander), 7 (Single), 12 (Batch). ✓
- Feature 3 (RAM) → Tasks 9 + 10 (Go), 11 (frontend). framer-motion kept (not touched). Wails v3 not migrated. ✓
- Vitest infra (spec §6 "add if absent") → Task 1. ✓
- RAM measurement (spec §4.4) → Task 12 Step 5. ✓

**Placeholder scan:** Tasks 7 and 11 Step 4 and 12 Step 1 intentionally instruct the implementer to read an existing file first because the exact local variable / store shape must be matched in place — each gives the concrete code to add and the exact integration point. No "TODO/TBD" left.

**Type consistency:** Store shape (`open`, `initialTab`, `droppedUrls`, `openWith`, `setOpen`, `consumeDropped`) is used identically in Tasks 3, 4, 7, 8. `WildcardRange { from, to, pad }`, `expandWildcard`, `validateRange`, `WILDCARD_MAX` consistent across Tasks 5, 6, 12. `classifyLinks` return `{ all, downloadable }` consistent in Tasks 2, 4. `StartMemoryController(ctx, activeCount func() int, interval)` + `JobManager.ActiveCount` consistent across Tasks 9, 10.
