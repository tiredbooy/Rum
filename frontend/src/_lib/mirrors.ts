import type { Download, DownloadReq } from "@/_lib/types/download-types";

/** Matches the backend limit (download.MaxMirrors). */
export const MAX_MIRRORS = 5;

/** Why `raw` can't be a mirror link, or null when it can (or is blank). */
export function mirrorProblem(raw: string): string | null {
  const value = raw.trim();
  if (!value) return null;
  try {
    const u = new URL(value);
    return u.protocol === "http:" || u.protocol === "https:"
      ? null
      : "Use an http or https link.";
  } catch {
    return "That is not a valid link.";
  }
}

/** Trim, drop blanks and repeats, keep at most MAX_MIRRORS. */
export function cleanMirrors(rows: string[]): string[] {
  const seen = new Set<string>();
  for (const row of rows) {
    const value = row.trim();
    if (value) seen.add(value);
  }
  return [...seen].slice(0, MAX_MIRRORS);
}

/** Short card label for a download served by a mirror, else null. */
export function mirrorLabel(
  d: Pick<Download, "active_mirror" | "status">,
): string | null {
  if (!d.active_mirror) return null;
  return `Mirror ${d.active_mirror}`;
}

/** The request to send: mirrors cleaned, and only kept for a single link. */
export function withCleanMirrors(req: DownloadReq): DownloadReq {
  const { mirrors, ...rest } = req;
  const clean = req.urls.length === 1 ? cleanMirrors(mirrors ?? []) : [];
  return clean.length ? { ...rest, mirrors: clean } : rest;
}
