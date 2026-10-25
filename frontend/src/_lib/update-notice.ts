/** GET /api/v1/update (Go: updater.Status). */
export interface UpdateStatus {
  current: string;
  latest?: string;
  tag?: string;
  available: boolean;
  /** The user chose "Later" for this release. */
  dismissed: boolean;
  release_url?: string;
  checked_at?: string;
  /** Error code of the newest check, e.g. "update_check_failed". */
  error?: string;
}

/** The banner shows for a newer, not-yet-dismissed release. */
export function showUpdateBanner(s: UpdateStatus | undefined): boolean {
  return !!s && s.available && !s.dismissed;
}

export function updateBannerText(s: UpdateStatus): string {
  return `Rum v${s.latest} available`;
}

/** One short status line for the Settings card. */
export function updateSummary(s: UpdateStatus | undefined, checking: boolean): string {
  if (checking) return "Checking…";
  if (!s) return "";
  if (s.error) return "Can't reach the update server.";
  if (s.available) return updateBannerText(s);
  if (s.latest) return `Up to date (v${s.current})`;
  return "Not checked yet";
}
