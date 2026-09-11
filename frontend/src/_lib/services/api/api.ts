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

/**
 * Error carrying the API's structured envelope: `{ error, code, fields }`.
 * `fields` is what lets a form put the message next to the offending control
 * instead of dropping it into a generic toast — previously `request` threw a
 * bare Error and every per-field message the backend sent was discarded.
 */
export class ApiError extends Error {
  readonly status: number;
  readonly code?: string;
  readonly fields?: Record<string, string>;

  constructor(
    message: string,
    status: number,
    code?: string,
    fields?: Record<string, string>,
  ) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.fields = fields;
  }

  /** The message for `field`, if the server reported one. */
  fieldError(field: string): string | undefined {
    return this.fields?.[field];
  }
}

export async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${apiBaseSync()}${path}`, {
    headers: { "Content-Type": "application/json" },
    ...options,
  });

  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as {
      error?: string;
      code?: string;
      fields?: Record<string, string>;
    } | null;
    throw new ApiError(
      body?.error || `HTTP ${res.status}`,
      res.status,
      body?.code,
      body?.fields,
    );
  }

  return res.json();
}
