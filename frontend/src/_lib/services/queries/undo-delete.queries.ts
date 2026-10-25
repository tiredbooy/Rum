import { useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "@/lib/toast";
import { deleteWithUndo, mergeResults } from "@/_lib/undo-delete";
import type { Download } from "@/_lib/types/download-types";
import {
  deleteDownload,
  deleteDownloads,
  restoreDownloads,
  type DeleteResult,
} from "../api/download-api";
import { downloadKeys } from "./download.queries";

/**
 * Delete with an Undo toast. `removeIds` hides specific downloads, `removeByStatus`
 * hides every download matching a status filter ("all" | "completed" | ...).
 * `deleteFile` false = "Remove from list" (data stays on disk); true = "Delete file".
 */
export function useUndoableDelete() {
  const qc = useQueryClient();

  const go = useCallback(
    (deleteFile: boolean, run: (o: { deleteFile?: boolean }) => Promise<DeleteResult>) =>
      deleteWithUndo(deleteFile, {
        run,
        restore: restoreDownloads,
        refresh: () => void qc.invalidateQueries({ queryKey: downloadKeys.all }),
        notify: {
          undoable: (message, action, duration) => toast(message, { action, duration }),
          info: (m) => toast.info(m),
          error: (m) => toast.error(m),
        },
      }),
    [qc],
  );

  const removeIds = useCallback(
    (ids: string[], deleteFile = false) =>
      go(deleteFile, async (o) => {
        const settled = await Promise.allSettled(ids.map((id) => deleteDownload(id, o)));
        const ok = settled.flatMap((s) => (s.status === "fulfilled" ? [s.value] : []));
        if (ok.length === 0) {
          const failed = settled.find((s) => s.status === "rejected") as PromiseRejectedResult | undefined;
          throw failed?.reason ?? new Error("Delete failed. Try again.");
        }
        if (ok.length < ids.length) toast.error(`${ids.length - ok.length} could not be removed`);
        return mergeResults(ok);
      }),
    [go],
  );

  const removeByStatus = useCallback(
    (status: Download["status"] | "all", deleteFile = false) =>
      go(deleteFile, (o) => deleteDownloads(status, o)),
    [go],
  );

  return { removeIds, removeByStatus };
}
