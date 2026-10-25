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

/** Number of downloads that failed for a connectivity reason. */
export function countNetworkBlocked(
  list: ReadonlyArray<{ status?: string; error_code?: string } | null | undefined>,
): number {
  return list.filter(
    (d) => d?.status === "error" && !!d.error_code && NETWORK_ERROR_CODES.has(d.error_code),
  ).length;
}
