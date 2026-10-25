// Pure helpers: no browser APIs, so they run under node:test.

export const MB = 1024 * 1024;

export const DEFAULTS = Object.freeze({
  port: 8080,
  token: "",
  autoCapture: true,
  minSizeMB: 5,
});

/** Rum tries 8080 first, then walks up; the extension scans the same span. */
export const PORT_SPAN = 10;

export function normalizeSettings(raw) {
  const r = raw && typeof raw === "object" ? raw : {};
  const port = Number.parseInt(r.port, 10);
  const min = Number(r.minSizeMB);
  return {
    port: Number.isInteger(port) && port >= 1 && port <= 65535 ? port : DEFAULTS.port,
    token: typeof r.token === "string" ? r.token.trim() : DEFAULTS.token,
    autoCapture: typeof r.autoCapture === "boolean" ? r.autoCapture : DEFAULTS.autoCapture,
    minSizeMB: r.minSizeMB !== "" && r.minSizeMB != null && Number.isFinite(min) && min >= 0 ? min : DEFAULTS.minSizeMB,
  };
}

export function portCandidates(port, span = PORT_SPAN) {
  const out = [];
  for (let p = port; p < port + span && p <= 65535; p++) out.push(p);
  return out;
}

/** Only real network URLs; blob:, data:, file: etc. stay with the browser. */
export function isCapturableUrl(url) {
  if (typeof url !== "string") return false;
  try {
    const u = new URL(url);
    return u.protocol === "http:" || u.protocol === "https:";
  } catch {
    return false;
  }
}

/**
 * Decide whether to take over a browser download.
 * Returns { capture, reason }; reason explains a refusal (useful in tests/logs).
 */
export function shouldCapture(item, settings) {
  if (!settings.autoCapture) return { capture: false, reason: "disabled" };
  if (!settings.token) return { capture: false, reason: "not-paired" };
  if (!isCapturableUrl(item.url)) return { capture: false, reason: "scheme" };
  if (item.incognito) return { capture: false, reason: "incognito" };
  if (item.state && item.state !== "in_progress") return { capture: false, reason: "state" };
  const size = Math.max(item.totalBytes ?? -1, item.fileSize ?? -1);
  if (!(size > 0)) return { capture: false, reason: "unknown-size" };
  if (size < settings.minSizeMB * MB) return { capture: false, reason: "too-small" };
  return { capture: true, reason: "" };
}

/** Last path segment of a local path (either separator); "" if none. */
export function basename(path) {
  if (typeof path !== "string") return "";
  const parts = path.split(/[\\/]/);
  return parts[parts.length - 1] || "";
}

/** Cookie header value from browser cookie objects. */
export function cookieHeader(cookies) {
  if (!Array.isArray(cookies)) return "";
  return cookies
    .filter((c) => c && c.name)
    .map((c) => `${c.name}=${c.value ?? ""}`)
    .join("; ");
}

/** Body for POST /ext/add; empty fields are left out. */
export function buildAddPayload({ url, filename, referer, userAgent, cookie }) {
  const body = { url };
  if (filename) body.filename = filename;
  if (referer) body.referer = referer;
  if (userAgent) body.user_agent = userAgent;
  if (cookie) body.cookie = cookie;
  return body;
}

/** The URL a context-menu click means: media source first, else the link. */
export function pickMenuUrl(info) {
  const url = info.mediaType && info.srcUrl ? info.srcUrl : info.linkUrl || info.srcUrl || "";
  return isCapturableUrl(url) ? url : "";
}

/** What the popup shows for a connection state. */
export function statusView(state) {
  switch (state) {
    case "paired":
      return { label: "Connected", tone: "ok", canOpen: true };
    case "not_paired":
      return { label: "Not paired. Paste the token in Options.", tone: "warn", canOpen: false };
    default:
      return { label: "Rum is not running", tone: "bad", canOpen: false };
  }
}
