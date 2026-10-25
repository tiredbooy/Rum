import { ApiError } from "@/_lib/services/api/api";
import type { Download } from "@/_lib/types/download-types";

/** The server answered 403/404/410: the link no longer serves the file. */
export function isLinkExpired(d: Pick<Download, "status" | "error_code">) {
  return d.status === "error" && d.error_code === "link_expired";
}

/** Why `raw` can't be used as a replacement link, or null when it can. */
export function linkProblem(raw: string): string | null {
  const value = raw.trim();
  if (!value) return "Paste the new link.";
  try {
    const u = new URL(value);
    if (u.protocol !== "http:" && u.protocol !== "https:") {
      return "Link must start with http or https.";
    }
    return null;
  } catch {
    return "That is not a valid link.";
  }
}

export type RefreshFailure =
  | { kind: "confirm-restart" }
  | { kind: "error"; message: string };

/**
 * What the dialog does with a failed refresh. remote_changed means the new link
 * is a different file than the partial download: ask before starting over.
 */
export function refreshFailure(err: unknown): RefreshFailure {
  if (err instanceof ApiError) {
    if (err.code === "remote_changed") return { kind: "confirm-restart" };
    if (err.status === 400) {
      return { kind: "error", message: "Link not reachable. Check it and retry." };
    }
  }
  return { kind: "error", message: "Could not update link. Try again." };
}
