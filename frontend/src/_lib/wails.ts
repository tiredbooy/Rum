/**
 * Typed, defensive wrappers around the Wails bindings the desktop shell injects
 * on `window.go.main.App`. The bindings only exist inside the Wails webview — in
 * plain browser dev or Vite preview they are `undefined` — so every call goes
 * through the generated binding module when the runtime is present and falls
 * back gracefully when it is not.
 *
 * The generated modules (`frontend/wailsjs/go/main/App`) are imported directly
 * rather than re-implemented here: they are what `wails build` regenerates, so
 * calling them keeps this file honest if a binding signature ever changes.
 */
import * as App from "../../wailsjs/go/main/App";

declare global {
  interface Window {
    go?: {
      main?: {
        App?: {
          GetApiBase?: () => string | Promise<string>;
          ChooseDir?: () => Promise<string> | string;
          Capabilities?: () => Promise<DesktopCapabilities>;
        };
      };
    };
    runtime?: {
      EventsOn?: (event: string, cb: (...data: unknown[]) => void) => () => void;
      EventsOff?: (event: string, ...extra: string[]) => void;
    };
  }
}

/** Platform features the running desktop build actually has. */
export interface DesktopCapabilities {
  tray: boolean;
  folderPicker: boolean;
  platform: string;
}

/** Conservative defaults for browser/dev, where no desktop shell is present. */
const NO_CAPABILITIES: DesktopCapabilities = {
  tray: false,
  folderPicker: false,
  platform: "web",
};

/** Fallback base URL used in browser/dev when no Wails binding is available. */
const FALLBACK_API_BASE =
  (import.meta.env.VITE_API_URL as string | undefined) ?? "http://127.0.0.1:8080";

/** Strip a single trailing slash so callers can safely append `/api/...`. */
function normalizeBase(base: string): string {
  return base.replace(/\/+$/, "");
}

/** Whether the Wails desktop runtime has injected its bindings. */
export function hasWailsRuntime(): boolean {
  return typeof window.go?.main?.App?.GetApiBase === "function";
}

// The resolved base is cached after the first successful resolution so the
// synchronous accessor (`apiBaseSync`) can be used by code paths (SSE,
// request<T>) that were written against the old synchronous `API_URL` constant.
let resolvedBase: string = normalizeBase(FALLBACK_API_BASE);
let resolved = false;

/**
 * Resolve the API base URL. Prefers the Wails `GetApiBase()` binding (which
 * returns the real loopback base, possibly on a non-8080 port), falling back to
 * `VITE_API_URL` and finally `http://127.0.0.1:8080`. Memoized: the first
 * non-empty result is cached and returned on subsequent calls.
 */
export async function getApiBase(): Promise<string> {
  if (resolved) return resolvedBase;

  try {
    if (hasWailsRuntime()) {
      const value = await App.GetApiBase();
      if (value && typeof value === "string") {
        resolvedBase = normalizeBase(value);
      }
    }
  } catch {
    // Ignore — keep the fallback base. (Binding missing or threw.)
  }

  resolved = true;
  return resolvedBase;
}

/**
 * Synchronous accessor for the already-resolved base. Returns the fallback until
 * `getApiBase()` has resolved once (call it at boot in `main.tsx`). Existing
 * synchronous `${API_URL}`-style usages keep working through this.
 */
export function apiBaseSync(): string {
  return resolvedBase;
}

/**
 * Outcome of a folder-picker request. `cancelled` and `failed` used to be
 * indistinguishable (both came back as `null`), which is exactly why a picker
 * that could not open looked like a button that did nothing.
 */
export type ChooseDirResult =
  | { status: "picked"; path: string }
  | { status: "cancelled" }
  | { status: "unavailable" }
  | { status: "failed"; message: string };

/** Open the native folder picker via the generated Wails binding. */
export async function chooseDir(): Promise<ChooseDirResult> {
  if (typeof window.go?.main?.App?.ChooseDir !== "function") {
    return { status: "unavailable" };
  }
  try {
    const dir = await App.ChooseDir();
    if (typeof dir === "string" && dir.trim()) {
      return { status: "picked", path: dir };
    }
    return { status: "cancelled" };
  } catch (err) {
    return {
      status: "failed",
      message: err instanceof Error ? err.message : String(err),
    };
  }
}

/** Whether the native folder picker binding is present (desktop runtime). */
export function hasChooseDir(): boolean {
  return typeof window.go?.main?.App?.ChooseDir === "function";
}

/**
 * Read the desktop feature matrix. Returns web-safe defaults when the shell is
 * absent or the binding is older than this frontend.
 */
export async function getCapabilities(): Promise<DesktopCapabilities> {
  if (typeof window.go?.main?.App?.Capabilities !== "function") {
    return { ...NO_CAPABILITIES, folderPicker: hasChooseDir() };
  }
  try {
    const caps = await App.Capabilities();
    return {
      tray: !!caps?.tray,
      folderPicker: !!caps?.folderPicker,
      platform: caps?.platform ?? "unknown",
    };
  } catch {
    return { ...NO_CAPABILITIES, folderPicker: hasChooseDir() };
  }
}

/**
 * Subscribe to a Wails runtime event. Returns an unsubscribe function; a no-op
 * outside the desktop shell so callers need no environment checks.
 */
export function onWailsEvent(
  event: string,
  handler: (...data: unknown[]) => void,
): () => void {
  const on = window.runtime?.EventsOn;
  if (typeof on !== "function") return () => {};
  try {
    const off = on(event, handler);
    return typeof off === "function"
      ? off
      : () => window.runtime?.EventsOff?.(event);
  } catch {
    return () => {};
  }
}
