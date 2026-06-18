# Rum — Drag-&-Drop Add, Wildcard Part-Ranges, and Lighter Idle RAM

**Date:** 2026-06-18
**Status:** Approved design (pre-plan)
**Target branch:** `dev`
**Scope:** Three independent improvements to the Rum download manager, shipped together:
1. Drag-and-drop links/HTML from a browser into Rum (IDM-style batch add).
2. Wildcard `*` part-range expansion (IDM-style "download with wildcard").
3. Reduce idle RAM (currently 150–300 MB) by cutting the controllable bloat.

---

## 1. Context

Rum is a Wails **v2.12.0** desktop app: a Go download engine (`backend/`), a React 19 + Vite + Tailwind v4 frontend (`frontend/`), and a thin desktop layer at the repo root (`main.go`, `app.go`, …). The "Add Downloads" dialog already has four tabs — Single / Bulk / From-URL / Batch — and a working `POST /api/v1/downloads/batch` endpoint that validates each URL independently and returns `{ created, errors }`.

None of the three features below exist yet (verified: no `dataTransfer`/`ondrop` handlers, no `*`-expansion code, no Go GC tuning).

### Decisions locked during brainstorming
- **Drag-drop UX:** drop anywhere on the window → opens the Add dialog on the **Batch** tab pre-filled for review (not instant-add, not a floating OS widget).
- **Link filter:** smart filter (downloadable file links) with a "show all links" toggle.
- **Wildcard:** IDM-style From/To + auto/editable zero-padding + live first/last preview.
- **RAM:** cut the controllable part, keep the current visual design.
- **framer-motion:** **kept** as a dependency (more readable to author). Use it where it makes an animation clearer; use plain CSS / `tw-animate-css` only where trivially equivalent. No forced removal.
- **Wails v3:** **deferred** to a future, separate round. Research (adversarially verified, high confidence) found v3 is still **alpha** (`v3.0.0-alpha.103`, mid-2026) and renders through the **same OS webview** as v2 (WebView2 / WKWebView / WebKitGTK), so it does **not** lower the idle-RAM floor. Its real win is a native cross-platform system tray (fixes Rum's Linux/macOS tray gap) — revisit at v3 beta/stable for that reason, not for memory. Migration note for later: v3 defaults to GTK4 / WebKitGTK 6.0 on Linux (legacy `-tags gtk3`, removed in v3.1), whereas Rum builds against WebKit2GTK 4.1 today.

### Goals
- Dragging a link or a selection of links from a browser onto Rum adds them, with a review step.
- A `*` in a URL lets the user generate a numbered range of downloads.
- Idle RAM materially lower than today, with no change to the look/feel.

### Non-goals
- No floating always-on-top "drop target" OS window.
- No instant-add-without-review drop mode (may be a future setting).
- No Wails v3 migration; no native-toolkit rewrite.
- No backend protocol/endpoint redesign — wildcard expansion happens client-side and reuses the existing batch endpoint.

---

## 2. Feature 1 — Drag & drop links from the browser

### 2.1 User flow
1. User drags a link (or a selection containing links) from any browser over the Rum window.
2. A full-window dashed overlay appears: *"Drop links to add to Rum."*
3. On drop, Rum extracts every candidate URL, applies the smart filter, and opens the Add-Downloads dialog on the **Batch** tab — pre-filled, downloadable links checked, page-like links unchecked.
4. User reviews (toggle "show all", uncheck noise), then clicks Add.

### 2.2 New modules (frontend)
**`frontend/src/features/drag-drop/extract-links.ts`** — pure, unit-testable:
- `extractUrlsFromDataTransfer(dt: DataTransfer): string[]`
  - Priority order: `text/uri-list` (split lines, drop `#`-comment lines) → `text/html` (parse via `DOMParser`, collect `a[href]`, `img[src]`, `video[src]`, `source[src]`, `[data-href]`) → `text/plain` (regex `https?://…`).
  - Resolve relative URLs against the fragment's `<base>` when present; otherwise keep absolute only.
  - Dedupe preserving order. Cap parsed anchors (e.g. 5000) to bound a whole-page drop.
- `classifyLinks(urls: string[]): { all: string[]; downloadable: string[] }`
  - `downloadable` = path ends in a known file extension (archives `zip|rar|7z|tar|gz|bz2|xz|part\d+`, video `mp4|mkv|avi|mov|webm|flv`, audio `mp3|flac|wav|m4a|aac|ogg`, disk `iso|img|bin`, docs `pdf|epub|apk|exe|msi|dmg|deb|rpm|appimage`).
  - Excluded as page-like: no extension, or `.html?|php|aspx?|jsp`, or query-only links.
  - `all` = every extracted http(s) URL.

**`frontend/src/features/drag-drop/DropProvider.tsx`** — mounted once in `App.tsx`:
- Window-level listeners for `dragenter`, `dragover`, `dragleave`, `drop`.
- `dragenter`/`dragover`: `preventDefault()` (critical — stops the WebKitGTK/WebView2 webview from navigating to the dropped URL) and set an `isDragging` flag that renders the overlay.
- `drop`: `preventDefault()`, run extraction + classification, then push the result into the Add-dialog store (see §2.3). Empty result → toast "No links found in the dropped content."
- Overlay is mounted only while dragging; `pointer-events` must not block the drop.

### 2.3 Refactor — lift Add-dialog open-state to a store
Today `Dashboard.tsx` owns `open`/`setOpen` and renders `<DownloadDialog>`. To let the global `DropProvider` (and the existing "Add Download" button) open it pre-filled:

**`frontend/src/stores/add-dialog-store.ts`** (new, zustand):
```ts
interface AddDialogState {
  open: boolean;
  initialTab: "single" | "bulk" | "url" | "batch";
  droppedUrls: string[] | null;   // consumed by BatchDownloadForm on open
  openWith: (opts?: { tab?; urls?: string[] }) => void;
  close: () => void;
  consumeDropped: () => string[] | null; // returns + clears droppedUrls
}
```
- `Dashboard.tsx` and `Toolbar` "Add Download" → `openWith()`.
- `DownloadDialogTabs` reads `initialTab` for its default `activeTab`.
- `BatchDownloadForm` on mount/open calls `consumeDropped()`; if non-null it seeds `rawText` and shows the smart-filter toggle (below).

### 2.4 BatchDownloadForm changes
- Accept seeded URLs from the store (in addition to manual paste).
- Add a **"Show all (N) / downloadable only (M)"** toggle. Default = downloadable only when seeded from a drop. The toggle swaps which set populates the textarea/checklist; manual edits always win.
- Everything else (per-URL validity count, submit to `useCreateBatch`, error summary) is unchanged.

### 2.5 Edge cases & risks
- **Webview navigation on drop** → prevented by global `preventDefault` on `dragover` + `drop`. (Wails file-drop interception stays **off** in `main.go` — do not enable `EnableFileDrop`.)
- Cross-platform: WebKitGTK (Linux) and WebView2 (Windows) both deliver `text/uri-list` + `text/html` for browser-link drags; verify on both in F.
- Garbage/empty drop → toast, dialog not opened.
- Huge page drop → anchor cap + dedupe.

---

## 3. Feature 2 — Wildcard `*` part-range expansion

### 3.1 User flow
When a URL the user types/pastes contains `*` (e.g. `…/game.part*.rar`), an inline **"Wildcard detected — expand range"** panel appears: From / To / Zero-pad digits, with a **live preview of the first and last generated filename** and an "Add N downloads" button. Submitting expands the pattern and creates the downloads via the batch endpoint.

### 3.2 New modules (frontend)
**`frontend/src/features/wildcard/wildcard.ts`** — pure, unit-testable:
- `hasWildcard(url: string): boolean` — contains at least one `*`.
- `expandWildcard(url, { from, to, pad }): string[]` — replace the **first** `*` with each integer in `[from, to]`, left-padded with `0` to `pad` digits (`pad = 0` → no padding). **Hard cap 1000** results; throw/return error above it.
- `validateRange({ from, to, pad })` — `to >= from`, both ≥ 0, count ≤ 1000.

**`frontend/src/features/wildcard/WildcardExpander.tsx`** — the inline panel:
- Inputs: From (default 1), To, Zero-pad digits (default 0; a "leading zeros" affordance). Live preview of first + last URL and the total count. Disabled Add until valid.
- Calls `useCreateBatch` with the expanded URL list (+ optional dest/category from the surrounding form).

### 3.3 Integration points
- **Single tab (primary):** when the Single form's URL field contains `*`, render `<WildcardExpander>` in place of the plain Add button.
- **Batch tab (secondary):** an "Expand `*` lines" action that runs `expandWildcard` on each wildcard line and merges the results into the textarea for review before submit.

### 3.4 Edge cases
- No `*` → panel absent.
- `to < from`, negative, or count > 1000 → inline validation error, Add disabled.
- `*` anywhere (path or query) works (plain string replace of the first `*`).
- Multiple `*` → only the first is expanded (documented in the panel hint).

---

## 4. Feature 3 — Lighter idle RAM (keep the look)

**Honest floor:** Rum embeds an OS webview (WebKitGTK / WebView2), which dominates idle RAM — ~80–150 MB is structural, the same as Electron/Tauri and unchanged by Wails v3. We cut only the controllable part. **Realistic idle target: ~100–150 MB** (from 150–300), measured on a production `wails build`, with 0 downloads.

### 4.1 Frontend levers
1. **Dev-tools out of production.** `App.tsx` ships `ReactQueryDevtools` unconditionally. Gate it behind `import.meta.env.DEV` with a lazy/dynamic import so it's tree-shaken from the prod bundle. *(Biggest easy win.)*
2. **Remove unused dependency `react-magic-ui`** (zero `src` usages) from `package.json`. Audit for other unused deps while there.
3. **Route-level code-splitting.** `router.tsx` eagerly imports all 5 pages. Convert each route `element` to `React.lazy(() => import(...))` wrapped in `<Suspense>` so Settings/charts/calendar aren't parsed or retained until visited.
4. **SSE idle-pause.** `AllProgressStream` holds an `EventSource` open for the app's lifetime. Pause (close) it when there are **0 active/pending** downloads and resume on activity (new download, mutation, or a list showing actives). Keep the existing reconnect/backoff for the active case.
5. **React Query cache hygiene.** SSE seeds per-job detail caches (`applyPatch`) that can accumulate. Set a sensible `gcTime` so idle/stale detail+list caches are collected.

### 4.2 Backend (Go) levers
6. **Return idle memory to the OS.** Add at engine/process init:
   - `debug.SetGCPercent(...)` (more aggressive than default 100, e.g. 50).
   - `debug.SetMemoryLimit(...)` soft cap as a backstop.
   - A lightweight goroutine that calls `debug.FreeOSMemory()` when no downloads are active (Go releases freed pages lazily via `MADV_FREE`; forcing it makes RSS actually drop). Gate on idle to avoid churn during active downloads.
7. **Verify no large buffers are retained at idle** (segmented engine uses 32 KB × connections × jobs; these should be freed when jobs finish). No change expected; confirm.

### 4.3 Explicitly kept
- **framer-motion stays.** Use it where it aids readability; use Tailwind / `tw-animate-css` CSS only where trivially equivalent. Accessibility is already covered by the `prefers-reduced-motion` block in `index.css`.
- No Wails v3 migration this round.

### 4.4 Measurement (acceptance)
- Build prod: `wails build -tags webkit2_41`.
- Launch, leave idle with 0 downloads ~60 s, record RSS (Linux `ps`/`smem`, Windows Task Manager) before and after the changes.
- Pass = a clear, repeatable reduction toward the ~100–150 MB target; no functional regressions.

---

## 5. Implementation strategy — Opus-4.8 agent workflow

Implementation runs as a **Workflow of Opus-4.8 subagents in isolated git worktrees** (the shared files `App.tsx`, the Add dialog, and the new store are touched by multiple workstreams, so worktree isolation + a careful merge prevents conflicts), followed by an integration/verify pass.

| Agent | Scope | Primary files |
|---|---|---|
| **A** | Drag-drop capture + extraction + overlay | `features/drag-drop/{extract-links.ts,DropProvider.tsx}`, `App.tsx`, tests |
| **B** | Add-dialog store refactor + Batch smart-filter toggle | `stores/add-dialog-store.ts`, `DownloadDialog.tsx`, `DownloadDialogTabs.tsx`, `Dashboard.tsx`, `Toolbar.tsx`, `BatchDownloadForm.tsx` |
| **C** | Wildcard logic + expander + Single/Batch wiring | `features/wildcard/{wildcard.ts,WildcardExpander.tsx}`, `SingleDownloadForm.tsx`, `BatchDownloadForm.tsx`, tests |
| **D** | Frontend RAM cuts | `App.tsx`, `router.tsx`, `package.json`, `AllProgressStream.tsx`, `queryClient.ts` |
| **E** | Backend Go RAM tuning | engine/process init in `backend/` and/or `main.go`, tests |
| **F** | Integration + verification | merge seams; run build/typecheck/tests; measure RAM |

**Sequencing / contract notes:**
- **B before A's final wire-up and before C's Batch hook** — B defines `add-dialog-store` (the shared contract A and C depend on). Define the store interface up front (§2.3) so A and C can code against it in parallel.
- A, C, D all edit `App.tsx`/`BatchDownloadForm.tsx` → reconcile in F (or assign those specific hunks to one agent).
- C and E are largely independent of the others.
- F runs last: `cd frontend && npm run build` (tsc + vite), `go build ./...`, `go test ./...`, and the RAM measurement in §4.4. Fix integration seams.

---

## 6. Testing & verification

**Unit (frontend, Vitest or equivalent — add if absent):**
- `extract-links`: uri-list / html / plain parsing, dedupe, relative resolution, anchor cap; `classifyLinks` downloadable vs page filtering.
- `wildcard`: detection, expansion with/without padding, range validation, 1000-cap.

**Unit (Go):** GC/idle tuning init is wired and the idle `FreeOSMemory` goroutine starts/stops correctly; existing `go test ./...` stays green.

**Manual / integration (F):**
- Drag a single browser link and a multi-link HTML selection onto Rum (Linux + Windows) → dialog opens on Batch with the right pre-fill; webview does **not** navigate.
- Paste `…/file.part*.rar` in Single → expander appears; From 1 / To 12 / pad 2 previews `part01`…`part12`; Add creates 12.
- Idle RAM measured before/after per §4.4.
- Full app smoke: add/start/pause/resume/complete a real download; SSE pause/resume across idle↔active transitions.

**Definition of done:** all three features work as described; `npm run build` + `go build ./...` + `go test ./...` pass; idle RAM measurably reduced toward target; no visual/functional regressions.

---

## 7. Risks & mitigations
- **Webview swallows/redirects drops** → global `preventDefault` on `dragover`+`drop`; keep Wails `EnableFileDrop` off; verify per-OS in F.
- **Worktree merge conflicts on shared files** → define the store contract first; isolate shared-file hunks to one agent or reconcile in F.
- **RAM target optimism** → target is framed as a range and measured on a prod build; the webview floor is acknowledged up front.
- **SSE idle-pause missing updates** → resume on any download-creating/mutating action and when a list contains actives; keep reconnect/backoff.
