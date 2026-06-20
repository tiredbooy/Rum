/**
 * useDownloadNotifications — mounted ONCE near the app root. Subscribes to the
 * React Query downloads cache and fires a desktop notification + optional sound
 * when a job transitions into `completed` or `error`.
 *
 * It reads from the cache only (it does NOT touch useAllProgressStream
 * internals); the SSE stream keeps the cache fresh, so cache mutations are the
 * single source of truth for status transitions. De-duped per job+status so
 * each completion/failure notifies exactly once for the app's lifetime.
 */
import { useEffect, useRef } from "react";
import { useQueryClient } from "@tanstack/react-query";
import type { Download, DownloadStatus } from "@/_lib/types/download-types";
import { useSettings } from "@/_lib/services/queries/settings.queries";
import { notify, playCompletionSound } from "./notifier";

const TERMINAL: ReadonlySet<DownloadStatus> = new Set(["completed", "error"]);

/** Friendly label for a download (filename, else the URL's last path segment). */
function labelFor(d: Download): string {
  if (d.filename && d.filename.trim()) return d.filename;
  try {
    const path = new URL(d.url).pathname;
    const last = path.split("/").filter(Boolean).pop();
    if (last) return decodeURIComponent(last);
  } catch {
    // ignore malformed URL
  }
  return d.url || "Download";
}

/**
 * Reduce every cached downloads query (lists + details) to one entry per id,
 * preferring whichever copy carries the most advanced status (terminal beats
 * non-terminal) so a stale list entry can't mask a completion.
 */
function collectDownloads(
  entries: [unknown, unknown][],
): Map<string, Download> {
  const byId = new Map<string, Download>();
  const consider = (d: Download | undefined | null) => {
    if (!d || typeof d.id !== "string" || typeof d.status !== "string") return;
    const existing = byId.get(d.id);
    if (!existing) {
      byId.set(d.id, d);
      return;
    }
    if (TERMINAL.has(d.status) && !TERMINAL.has(existing.status)) {
      byId.set(d.id, d);
    }
  };
  for (const [, data] of entries) {
    if (Array.isArray(data)) {
      for (const item of data) consider(item as Download);
    } else if (data && typeof data === "object" && "status" in data) {
      consider(data as Download);
    }
  }
  return byId;
}

export function useDownloadNotifications(): null {
  const queryClient = useQueryClient();
  const { data: settings } = useSettings();

  // Keep the latest `silent` available to the long-lived subscription without
  // re-subscribing whenever settings change.
  const silentRef = useRef<boolean>(settings?.silent ?? false);
  silentRef.current = settings?.silent ?? false;

  useEffect(() => {
    const cache = queryClient.getQueryCache();
    const snapshot = (): Map<string, Download> =>
      collectDownloads(
        cache
          .findAll({ queryKey: ["downloads"] })
          .map((q) => [q.queryKey, q.state.data] as [unknown, unknown]),
      );

    // Per-job last status we've *notified* for. App-lifetime de-dupe.
    const notified = new Map<string, DownloadStatus>();

    // Seed from the current cache WITHOUT notifying so already-finished jobs at
    // mount don't fire a burst of stale notifications.
    for (const [id, d] of snapshot()) {
      if (TERMINAL.has(d.status)) notified.set(id, d.status);
    }

    const handle = () => {
      for (const [id, d] of snapshot()) {
        const status = d.status;
        if (!TERMINAL.has(status)) {
          // Left a terminal state (retry/repair) — allow a future re-notify.
          notified.delete(id);
          continue;
        }
        if (notified.get(id) === status) continue;
        notified.set(id, status);

        const name = labelFor(d);
        if (status === "completed") {
          void notify({
            title: "Download complete",
            body: name,
            silent: silentRef.current,
            tag: `rum-dl-${id}`,
          });
          playCompletionSound();
        } else {
          void notify({
            title: "Download failed",
            body: d.error ? `${name} — ${d.error}` : name,
            silent: silentRef.current,
            tag: `rum-dl-${id}`,
          });
        }
      }
    };

    const unsubscribe = cache.subscribe(handle);
    return () => unsubscribe();
    // queryClient is stable for the app's lifetime; subscribe once.
  }, [queryClient]);

  return null;
}
