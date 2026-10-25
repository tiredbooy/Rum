import { NETWORK_ERROR_CODES } from "./download-errors";

/** True when the backend answers a cheap request. Never throws. */
export async function probeBackend(
  baseUrl: string,
  fetchImpl: typeof fetch = fetch,
  timeoutMs = 4000,
): Promise<boolean> {
  const ctrl = new AbortController();
  const timer = setTimeout(() => ctrl.abort(), timeoutMs);
  try {
    const res = await fetchImpl(`${baseUrl}/api/v1/downloads/stats`, {
      signal: ctrl.signal,
      cache: "no-store",
    });
    return res.ok;
  } catch {
    return false;
  } finally {
    clearTimeout(timer);
  }
}

/** Number of distinct downloads that failed for a connectivity reason. */
export function countNetworkBlocked(
  list: ReadonlyArray<{ id?: string; status?: string; error_code?: string } | null | undefined>,
): number {
  const seen = new Set<string>();
  let n = 0;
  for (const d of list) {
    if (d?.status !== "error" || !d.error_code || !NETWORK_ERROR_CODES.has(d.error_code)) continue;
    if (d.id) {
      if (seen.has(d.id)) continue;
      seen.add(d.id);
    }
    n++;
  }
  return n;
}
