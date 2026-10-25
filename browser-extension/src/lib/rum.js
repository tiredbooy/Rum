// Talks to Rum's /ext API. fetch is injectable for tests.
import { portCandidates } from "./rules.js";

export const TOKEN_HEADER = "X-Rum-Token";
const APP_HEADER = "X-Rum-App";
const PING_TIMEOUT_MS = 1500;
// Rum probes the link (up to a minute) before it accepts; stay under what an
// MV3 service worker will reliably wait.
const ADD_TIMEOUT_MS = 20000;
// Listing a page's formats runs yt-dlp, which Rum allows up to a minute.
const VIDEO_TIMEOUT_MS = 75000;

const base = (port) => `http://127.0.0.1:${port}`;

async function call(fetchImpl, port, token, path, { method = "GET", body, timeoutMs }) {
  const ctl = new AbortController();
  const timer = setTimeout(() => ctl.abort(), timeoutMs);
  try {
    return await fetchImpl(base(port) + path, {
      method,
      headers: { ...(token ? { [TOKEN_HEADER]: token } : {}), ...(body ? { "Content-Type": "application/json" } : {}) },
      body: body ? JSON.stringify(body) : undefined,
      signal: ctl.signal,
      cache: "no-store",
    });
  } finally {
    clearTimeout(timer);
  }
}

/** Does a Rum answer on this port? Asked WITHOUT the token. */
async function isRum(port, fetchImpl) {
  try {
    const res = await call(fetchImpl, port, "", "/ext/ping", { timeoutMs: PING_TIMEOUT_MS });
    return res.headers?.get?.(APP_HEADER) === "rum";
  } catch {
    return false;
  }
}

async function pingPort(settings, port, fetchImpl) {
  // Identify first: the token only goes to a port that proved to be Rum.
  if (!(await isRum(port, fetchImpl))) return { state: "not_running", port };
  if (!settings.token) return { state: "not_paired", port };
  try {
    const res = await call(fetchImpl, port, settings.token, "/ext/ping", { timeoutMs: PING_TIMEOUT_MS });
    if (res.status === 200) return { state: "paired", port };
  } catch {
    // fall through
  }
  return { state: "not_paired", port };
}

const RANK = { paired: 0, not_paired: 1, not_running: 2 };

/**
 * Locate Rum. Tries the configured port and the few above it (Rum moves up when
 * 8080 is taken) in parallel. state: "paired" | "not_paired" | "not_running".
 */
export async function findRum(settings, fetchImpl = fetch) {
  const results = await Promise.all(
    portCandidates(settings.port).map((p) => pingPort(settings, p, fetchImpl)),
  );
  return results.reduce((best, r) => (RANK[r.state] < RANK[best.state] ? r : best), results[0]);
}

/** Hand a download to Rum. ok is true only when Rum accepted the job. */
export async function addDownload(payload, settings, fetchImpl = fetch) {
  try {
    const res = await call(fetchImpl, settings.port, settings.token, "/ext/add", {
      method: "POST",
      body: payload,
      timeoutMs: ADD_TIMEOUT_MS,
    });
    return { ok: res.ok, status: res.status };
  } catch {
    return { ok: false, status: 0 };
  }
}

/** Hand a video page to Rum (yt-dlp picks the best format). ok only when queued. */
export async function addVideo(pageUrl, settings, fetchImpl = fetch) {
  try {
    const res = await call(fetchImpl, settings.port, settings.token, "/ext/video", {
      method: "POST",
      body: { url: pageUrl },
      timeoutMs: VIDEO_TIMEOUT_MS,
    });
    let code = "";
    if (!res.ok) code = (await res.json().catch(() => null))?.code || "";
    return { ok: res.ok, status: res.status, code };
  } catch {
    return { ok: false, status: 0, code: "" };
  }
}

/** Ask Rum to raise its window. */
export async function openRum(settings, fetchImpl = fetch) {
  try {
    const res = await call(fetchImpl, settings.port, settings.token, "/ext/open", {
      method: "POST",
      body: {},
      timeoutMs: PING_TIMEOUT_MS,
    });
    return res.ok;
  } catch {
    return false;
  }
}
