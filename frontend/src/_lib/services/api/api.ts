import { apiBaseSync } from "@/_lib/wails";

/**
 * Resolved API base URL. The real value is resolved once at boot in `main.tsx`
 * via `getApiBase()` (which prefers the Wails binding). Read it through
 * `apiBaseSync()` at call time so it reflects the resolved value rather than a
 * stale import-time snapshot.
 *
 * `API_URL` is kept as a backward-compatible re-export for existing callers; it
 * is a function-backed getter so `${API_URL}` keeps yielding the resolved base.
 */
export function apiUrl(): string {
  return apiBaseSync();
}

// Backward-compatible string-like export. Defined as a getter on a module-level
// object so any `${API_URL}` interpolation reads the live resolved base.
export const API_URL: string = apiBaseSync();

export async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${apiBaseSync()}${path}`, {
    headers: { "Content-Type": "application/json" },
    ...options,
  });

  if (!res.ok) {
    const error = await res.json().catch(() => ({ error: "Request failed" }));
    throw new Error(error.error || `HTTP ${res.status}`);
  }

  return res.json();
}
