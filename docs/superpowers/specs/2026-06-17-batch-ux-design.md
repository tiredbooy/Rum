# Batch UX Enhancements — Design Spec

Date: 2026-06-17
Status: Approved (pending implementation; sequenced AFTER the download-throughput fix)

## Goal

Make Rum's batch downloading better than IDM with: (1) a wildcard/pattern
generator, (2) preview + server-side existence verification, (3) drag-and-drop of
links from the browser, and (4) an IDM-style floating "DropBox" mode.

## Core principle

All inputs converge on a single **URL list** that feeds the EXISTING
`POST /api/v1/downloads/batch` endpoint. This keeps the work almost entirely in
the frontend; the engine is untouched. The only new backend surface is a
verification (probe) endpoint, because the webview cannot cross-origin HEAD
arbitrary remote servers (their CORS blocks it) while a Go server-side HEAD can.

```
Wildcard generator ─┐
Dropped links ──────┼─▶ unified URL list (preview · select · verify)
Pasted text ────────┘            │
                                 ├─ POST /downloads/batch/probe  (NEW)
                                 └─ POST /downloads/batch        (EXISTING)
```

## Feature 1 — Wildcard / pattern batch (frontend only)

A collapsible **"Generate from pattern"** panel at the top of the existing **Batch**
tab (`BatchDownloadForm.tsx`). No new tab.

- Paste a sample URL (`https://site/file01.zip`). Auto-detect the numeric run
  (`01`), highlight it, replace with `*` → `file*.zip`. If multiple number groups
  exist, the user clicks which one is the counter.
- **Range**: `from`, `to`, `step` (default 1). **Padding auto-inferred** from the
  sample (`01` → zero-padded, width 2); user can override width / toggle padding.
- **Alpha ranges** supported too (`a–z`, `A–Z`) — a nicety over IDM.
- **Live preview**: generated count, first/last URL. Hard cap (default 2000) with a
  visible warning so a bad range can't generate a runaway list.
- Generated URLs append into the unified list.

### Pattern expansion rules (pure function, unit-tested)
- Exactly one `*` (or the selected number group) is the counter.
- Numeric: `from..to` step `step`, formatted to inferred width with optional zero
  pad. Alpha: single-char `a..z`/`A..Z` by code point.
- Reject: `from > to` with positive step, `step <= 0`, count > cap.

## Feature 2 — Preview + verify (frontend + new backend endpoint)

- Each URL row shows a status chip. **"Check which exist"** → `POST
  /downloads/batch/probe` → each row resolves to ✓ *exists (size, filename)* or
  ✗ *status/unreachable*; ✗ rows auto-deselect.
- **"Add N kept"** → `POST /downloads/batch` with selected URLs + shared options
  (dest folder, category, auto-start).

### New endpoint: `POST /api/v1/downloads/batch/probe`
- Request: `{ "urls": ["...", ...] }` (count-capped, e.g. ≤ 500 per call).
- Response: `{ "results": [{ "url", "exists": bool, "status": int,
  "size": int64, "filename": string, "content_type": string, "error": string }] }`
- Server-side HEAD with bounded concurrency + per-request timeout; on a 405/501
  (server rejects HEAD) fall back to a ranged GET (`Range: bytes=0-0`). Reuses the
  engine's existing HEAD/metadata logic where possible. Loopback-only, same CORS
  allowlist as the rest of the API.

## Feature 3 — Drag & drop, in-app zone (frontend only)

- A drop target inside the Batch tab AND a full-window overlay that appears while
  dragging over Rum.
- On drop, read `dataTransfer` in priority order: `text/uri-list`, then
  `text/plain`, then `text/html` (extract every `<a href>` so dragging a selection
  of links captures them all). Filter to http(s), dedupe → unified list.
- Pure-function drop parser, unit-tested (uri-list/plain/html extraction).
- Wails note: the build does not enable Wails `OnFileDrop`, so the webview passes
  native drops to the DOM; the real browser→window drag must be verified manually
  on WebKitGTK with a dev build.

## Feature 4 — Floating DropBox mode (frontend + small desktop-layer change)

Wails v2 is single-window, so this is a **compact "drop mode"** of the main window,
not a second coexisting window:
- A toggle (button + tray item) shrinks Rum to ~190×190, always-on-top, positioned
  at a screen corner (via `WindowSetSize`/`WindowSetPosition`/`WindowSetAlwaysOnTop`).
- Dropping links onto it queues straight to `/downloads/batch` with a toast.
- Click to restore the full window (previous geometry remembered, reusing the
  existing window-state persistence).
- A truly separate, coexisting mini-window needs a helper process — out of scope;
  noted as a Wails-v3 follow-up.

## Delivery phases (each independently testable)

1. Wildcard generator + preview (frontend only).
2. Backend probe endpoint + wire up "Check which exist".
3. In-app drop zone.
4. Floating DropBox mode.

## Testing

- Go: probe endpoint — exists/404/size, concurrency cap, malformed URLs, HEAD-reject
  GET-range fallback, count cap.
- Frontend: pattern expander (padding/step/alpha/caps) and drop parser
  (uri-list/plain/html) unit tests.
- Manual: real browser→window drag on WebKitGTK via a dev build.

## Constraints

- Do NOT touch the installed binary at `~/.local/bin/Rum`. Build dev binaries to a
  separate path (or `wails dev`); the user installs when satisfied.
- Reuse the existing batch endpoint and options; no engine internals changed by
  these features.

## Out of scope (noted for later)

- Browser extension ("Download with Rum", grab-all-links) — most IDM-like but a
  separate component.
- Truly separate floating window (Wails v3).
```
