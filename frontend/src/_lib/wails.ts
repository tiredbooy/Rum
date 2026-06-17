/**
 * Typed, defensive wrappers around the Wails JS bindings that the desktop shell
 * injects on `window.go.main.App`. These bindings are ONLY present when the app
 * runs inside the Wails webview — in `wails dev`, plain browser dev, or Vite
 * preview they are `undefined`. Every accessor therefore uses optional chaining
 * and a graceful fallback so the TypeScript build never depends on the generated
 * bindings existing at runtime.
 */

declare global {
  interface Window {
    go?: {
      main?: {
        App?: {
          GetApiBase?: () => string | Promise<string>;
          ChooseDir?: () => Promise<string> | string;
        };
      };
    };
  }
}

/** Fallback base URL used in browser/dev when no Wails binding is available. */
const FALLBACK_API_BASE =
  (import.meta.env.VITE_API_URL as string | undefined) ?? "http://127.0.0.1:8080";

/** Strip a single trailing slash so callers can safely append `/api/...`. */
function normalizeBase(base: string): string {
  return base.replace(/\/+$/, "");
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
    const fn = window.go?.main?.App?.GetApiBase;
    if (typeof fn === "function") {
      const value = await fn();
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
 * Open the native folder picker via the Wails binding. Returns the chosen
 * absolute path, or `null` when the binding is unavailable (browser/dev) or the
 * user cancelled the dialog.
 */
export async function chooseDir(): Promise<string | null> {
  try {
    const fn = window.go?.main?.App?.ChooseDir;
    if (typeof fn === "function") {
      const dir = await fn();
      return dir && typeof dir === "string" ? dir : null;
    }
  } catch {
    // Ignore — fall through to null so the caller can show a text input.
  }
  return null;
}

/** Whether the native folder picker binding is present (desktop runtime). */
export function hasChooseDir(): boolean {
  return typeof window.go?.main?.App?.ChooseDir === "function";
}
