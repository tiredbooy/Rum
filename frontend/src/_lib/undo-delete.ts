import type { DeleteOptions, DeleteResult } from "@/_lib/services/api/download-api";

/** Toast stays up this long; the server keeps the delete undoable a bit longer. */
export const UNDO_TOAST_MS = 10_000;

export interface UndoDeleteDeps {
  /** The delete call; resolves with the ids that are now hidden. */
  run: (opts: DeleteOptions) => Promise<DeleteResult>;
  restore: (ids: string[]) => Promise<unknown>;
  /** Refetch lists after a delete or an undo. */
  refresh: () => void;
  notify: {
    undoable: (message: string, action: { label: string; onClick: () => void }, durationMs: number) => void;
    info: (message: string) => void;
    error: (message: string) => void;
  };
}

function summary(count: number, deleteFile: boolean): string {
  if (deleteFile) return count === 1 ? "File deleted" : `${count} files deleted`;
  return count === 1 ? "Removed from list" : `${count} removed from list`;
}

/**
 * Act now, undo later: run the delete immediately, then offer Undo in a toast.
 * Returns the hidden ids (empty when nothing matched or the call failed).
 */
export async function deleteWithUndo(
  deleteFile: boolean,
  deps: UndoDeleteDeps,
): Promise<string[]> {
  let result: DeleteResult;
  try {
    result = await deps.run({ deleteFile });
  } catch (err) {
    deps.refresh();
    deps.notify.error(err instanceof Error ? err.message : "Delete failed. Try again.");
    return [];
  }
  deps.refresh();
  const ids = result.ids ?? [];
  if (ids.length === 0) {
    deps.notify.info("Nothing to remove");
    return [];
  }
  deps.notify.undoable(
    summary(ids.length, deleteFile),
    {
      label: "Undo",
      onClick: () => {
        deps
          .restore(ids)
          .catch(() => deps.notify.error("Too late to undo"))
          .finally(() => deps.refresh());
      },
    },
    UNDO_TOAST_MS,
  );
  return ids;
}

/** Merge several single-item delete results into one, so a bulk delete shows one Undo. */
export function mergeResults(results: DeleteResult[]): DeleteResult {
  return {
    ids: results.flatMap((r) => r.ids ?? []),
    pending_delete_at: results[0]?.pending_delete_at ?? "",
    undo_seconds: Math.min(...results.map((r) => r.undo_seconds), UNDO_TOAST_MS / 1000),
  };
}
