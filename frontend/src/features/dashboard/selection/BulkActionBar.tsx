import { useMemo, useState } from "react";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { Moon, Pause, Play, RotateCcw, Trash2, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Spinner } from "@/components/ui/spinner";
import { useDownloads } from "@/_lib/services/queries/download.queries";
import type { Download } from "@/_lib/types/download-types";
import { useMoveToNightQueue } from "@/_lib/services/queries/night-plan.queries";
import { useSelection } from "./useSelection";
import { useBulkActions, type BulkActionKind } from "./useBulkActions";

/**
 * Floating bulk-action bar. Renders only when the selection is non-empty.
 * Resolves the selected ids against the full ("all") download list so it can
 * filter actions by status and show an accurate count, independent of which
 * status tab is currently visible.
 *
 * Accessibility: rendered in a `role="toolbar"` region with an aria-label and a
 * polite live region announcing the count; the destructive delete goes through
 * a dropdown (Remove from list / Delete file) and
 * is undoable from the toast, so there is no confirm dialog.
 */
export function BulkActionBar() {
  const selectedIds = useSelection((s) => s.selectedIds);
  const clear = useSelection((s) => s.clear);
  const { run } = useBulkActions();
  const moveToNight = useMoveToNightQueue();
  const reduceMotion = useReducedMotion();

  // Resolve ids → Download objects from the canonical "all" list (already
  // cached by the dashboard). We only need statuses here.
  const { data: allDownloads } = useDownloads("all");
  const selected = useMemo<Download[]>(() => {
    const list: Download[] = allDownloads ?? [];
    return list.filter((d: Download) => selectedIds.has(d.id));
  }, [allDownloads, selectedIds]);

  const [pending, setPending] = useState<BulkActionKind | null>(null);

  const count = selectedIds.size;

  const handle = async (kind: BulkActionKind, deleteFile = false) => {
    if (pending) return;
    setPending(kind);
    try {
      await run(kind, selected, { deleteFile });
      if (kind === "delete") clear();
    } finally {
      setPending(null);
    }
  };

  const visible = count > 0;

  return (
    <>
      <AnimatePresence>
        {visible && (
          <motion.div
            key="bulk-bar"
            role="toolbar"
            aria-label={`Bulk actions for ${count} selected download${
              count === 1 ? "" : "s"
            }`}
            initial={reduceMotion ? false : { y: 24, opacity: 0 }}
            animate={{ y: 0, opacity: 1 }}
            exit={reduceMotion ? { opacity: 0 } : { y: 24, opacity: 0 }}
            transition={{ duration: 0.18, ease: "easeOut" }}
            className="fixed bottom-6 left-1/2 z-40 -translate-x-1/2"
          >
            <div className="flex items-center gap-1 rounded-full border bg-background/95 px-2 py-1.5 shadow-lg shadow-black/10 backdrop-blur-xl">
              <span
                className="px-2 text-sm font-medium tabular-nums"
                aria-live="polite"
              >
                {count} selected
              </span>
              <span className="mx-0.5 h-5 w-px bg-border" aria-hidden />

              <Button
                variant="ghost"
                size="sm"
                className="gap-1.5 h-8"
                onClick={() => handle("pause")}
                disabled={pending !== null}
              >
                {pending === "pause" ? (
                  <Spinner className="size-3.5" />
                ) : (
                  <Pause className="size-3.5" />
                )}
                Pause
              </Button>
              <Button
                variant="ghost"
                size="sm"
                className="gap-1.5 h-8"
                onClick={() => handle("resume")}
                disabled={pending !== null}
              >
                {pending === "resume" ? (
                  <Spinner className="size-3.5" />
                ) : (
                  <Play className="size-3.5" />
                )}
                Resume
              </Button>
              <Button
                variant="ghost"
                size="sm"
                className="gap-1.5 h-8"
                onClick={() => handle("retry")}
                disabled={pending !== null}
              >
                {pending === "retry" ? (
                  <Spinner className="size-3.5" />
                ) : (
                  <RotateCcw className="size-3.5" />
                )}
                Retry
              </Button>
              <Button
                variant="ghost"
                size="sm"
                className="gap-1.5 h-8"
                onClick={() =>
                  moveToNight.mutate({
                    ids: selected
                      .filter((d) => d.status !== "completed")
                      .map((d) => d.id),
                    enabled: true,
                  })
                }
                disabled={pending !== null || moveToNight.isPending}
                aria-label="Move to night queue"
                title="Move to night queue"
              >
                {moveToNight.isPending ? (
                  <Spinner className="size-3.5" />
                ) : (
                  <Moon className="size-3.5" />
                )}
                Night
              </Button>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="gap-1.5 h-8 text-destructive hover:text-destructive"
                    disabled={pending !== null}
                  >
                    {pending === "delete" ? (
                      <Spinner className="size-3.5" />
                    ) : (
                      <Trash2 className="size-3.5" />
                    )}
                    Delete
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" side="top">
                  <DropdownMenuItem onSelect={() => void handle("delete")}>
                    Remove from list
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    className="text-destructive focus:text-destructive"
                    onSelect={() => void handle("delete", true)}
                  >
                    Delete file
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>

              <span className="mx-0.5 h-5 w-px bg-border" aria-hidden />
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8"
                onClick={clear}
                aria-label="Clear selection"
                title="Clear selection"
                disabled={pending !== null}
              >
                <X className="size-4" />
              </Button>
            </div>
          </motion.div>
        )}
      </AnimatePresence>

    </>
  );
}
