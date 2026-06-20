import { useCallback } from "react";
import type { Download, DownloadStatus } from "@/_lib/types/download-types";
import {
  useDeleteDownload,
  usePauseDownload,
  useResumeDownload,
  useRetryDownload,
} from "@/_lib/services/queries/download.queries";
import { toast } from "@/lib/toast";

/**
 * Which statuses a given bulk action is meaningful for. Actions silently skip
 * items whose status is not applicable (e.g. you can't pause a completed file),
 * and the summary toast reports how many were skipped.
 */
const APPLICABLE: Record<BulkActionKind, ReadonlySet<DownloadStatus>> = {
  pause: new Set<DownloadStatus>(["running"]),
  resume: new Set<DownloadStatus>(["paused", "pending"]),
  retry: new Set<DownloadStatus>(["error"]),
  // Delete applies to everything.
  delete: new Set<DownloadStatus>([
    "running",
    "completed",
    "error",
    "paused",
    "pending",
  ]),
};

export type BulkActionKind = "pause" | "resume" | "retry" | "delete";

const VERB: Record<BulkActionKind, string> = {
  pause: "Paused",
  resume: "Resumed",
  retry: "Retried",
  delete: "Deleted",
};

interface RunResult {
  ok: number;
  failed: number;
  skipped: number;
}

/**
 * Bulk-operation runner. Implements each bulk action by mapping the selected
 * ids over the existing *single-item* mutations and awaiting them with
 * `Promise.allSettled`, then surfacing a single summary toast. Items whose
 * status doesn't apply to the action are skipped (not errored).
 *
 * Returns stable callbacks; pass the currently-selected `Download[]` so each
 * action can filter by status without re-deriving from the cache.
 */
export function useBulkActions() {
  const pause = usePauseDownload();
  const resume = useResumeDownload();
  const retry = useRetryDownload();
  const del = useDeleteDownload();

  const run = useCallback(
    async (kind: BulkActionKind, items: Download[]): Promise<RunResult> => {
      const applicable = APPLICABLE[kind];
      const targets = items.filter((d) => applicable.has(d.status));
      const skipped = items.length - targets.length;

      if (targets.length === 0) {
        toast.info(
          `Nothing to ${kind} — selected items don't support this action`,
        );
        return { ok: 0, failed: 0, skipped };
      }

      const mutateFor = (id: string): Promise<unknown> => {
        switch (kind) {
          case "pause":
            return pause.mutateAsync(id);
          case "resume":
            return resume.mutateAsync(id);
          case "retry":
            return retry.mutateAsync(id);
          case "delete":
            return del.mutateAsync(id);
        }
      };

      const settled = await Promise.allSettled(
        targets.map((d) => mutateFor(d.id)),
      );
      const ok = settled.filter((s) => s.status === "fulfilled").length;
      const failed = settled.length - ok;

      const verb = VERB[kind];
      if (failed === 0) {
        toast.success(
          `${verb} ${ok} download${ok === 1 ? "" : "s"}` +
            (skipped > 0 ? ` · ${skipped} skipped` : ""),
        );
      } else {
        toast.error(
          `${verb} ${ok}/${targets.length} — ${failed} failed` +
            (skipped > 0 ? ` · ${skipped} skipped` : ""),
        );
      }
      return { ok, failed, skipped };
    },
    [pause, resume, retry, del],
  );

  return { run };
}
