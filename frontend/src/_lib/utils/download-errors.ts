/**
 * One shared map from a job's `error_code` (backend error_codes.go) to a short,
 * plain-language message plus a suggested action. Sibling features (disk_full,
 * blocked_page, stalled, link_expired) render their own richer UI; the entries
 * here are the fallback text used on the card.
 */
export interface DownloadErrorInfo {
  /** What happened, sentence case, no trailing detail. */
  message: string;
  /** What to do next. */
  action: string;
}

export const DOWNLOAD_ERROR_MESSAGES: Record<string, DownloadErrorInfo> = {
  network_down: { message: "Connection lost.", action: "Check internet or VPN." },
  dns: { message: "Site not found.", action: "Check the link, DNS or VPN." },
  tls: { message: "Secure connection failed.", action: "Try turning off VPN or proxy." },
  timeout: { message: "Server took too long.", action: "Retry, or switch VPN." },
  proxy: { message: "Proxy failed.", action: "Check proxy settings." },
  server_error: { message: "Server had a problem.", action: "Retry in a while." },
  permission: { message: "Can't write to folder.", action: "Choose another folder." },
  disk_full: { message: "Disk is full.", action: "Free space, then retry." },
  blocked_page: { message: "Link opened a web page, not a file.", action: "Get the direct link." },
  stalled: { message: "Connection stalled, no data received.", action: "Retry, or switch VPN." },
  site_unreachable: { message: "This site is unreachable.", action: "Try a VPN or proxy." },
  link_expired: { message: "Link expired.", action: "Get a fresh link." },
  night_limit: { message: "Night limit reached.", action: "Resumes next window." },
  update_check_failed: { message: "Can't reach update server.", action: "Try a proxy or VPN." },
  update_verify_failed: { message: "Update failed its check, deleted.", action: "Get it from the release page." },
  unknown: { message: "Download failed.", action: "Retry; copy details for a bug report." },
};

/** Codes that mean "the network is the problem" (used for the global notice). */
export const NETWORK_ERROR_CODES: ReadonlySet<string> = new Set([
  "network_down",
  "dns",
  "timeout",
  "proxy",
]);

/** Resolve a code to its message; unknown or missing codes fall back to `unknown`. */
export function describeDownloadError(code?: string | null): DownloadErrorInfo {
  return (code && DOWNLOAD_ERROR_MESSAGES[code]) || DOWNLOAD_ERROR_MESSAGES.unknown;
}
