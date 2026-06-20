import { useEffect, useRef, useState } from "react";
import { Reorder, useDragControls, useReducedMotion } from "framer-motion";
import type { Download, DownloadPriority } from "@/_lib/types/download-types";
import { toast } from "@/lib/toast";
import { CardDragHandle } from "./DownloadCard";
import { DownloadItemWrapper } from "./DownloadWrapper";

interface ReorderableDownloadListProps {
  downloads: Download[];
  orderedIds: string[];
  selectionActive: boolean;
  onPause: (id: string) => void;
  onResume: (id: string) => void;
  onStart: (id: string) => void;
  onDelete: (id: string) => void;
  onRetry: (id: string) => void;
  onSetPriority: (id: string, priority: DownloadPriority) => void;
}

/**
 * Bucket an item's index in the (re)ordered list into a priority.
 *
 * NOTE: the backend exposes a 3-level priority (low/normal/high), not a free
 * per-item order field, so drag-to-reorder is persisted *pragmatically* by
 * mapping vertical thirds to priority buckets (top → high, middle → normal,
 * bottom → low). True per-item ordering is a roadmap item (a dedicated order
 * field on the job); until then this gives the user meaningful, persisted
 * control over scheduling without a backend change.
 */
function bucketForIndex(index: number, total: number): DownloadPriority {
  if (total <= 1) return "normal";
  const ratio = index / (total - 1);
  if (ratio < 1 / 3) return "high";
  if (ratio < 2 / 3) return "normal";
  return "low";
}

export function ReorderableDownloadList({
  downloads,
  orderedIds,
  selectionActive,
  onSetPriority,
  ...actions
}: ReorderableDownloadListProps) {
  const reduceMotion = useReducedMotion();
  // Local visual order, seeded from incoming ids. We only resync when the *set*
  // of ids changes (add/remove) so live SSE progress ticks don't fight a drag.
  const [order, setOrder] = useState<string[]>(orderedIds);
  const byId = useRef(new Map<string, Download>());
  byId.current = new Map(downloads.map((d) => [d.id, d]));

  useEffect(() => {
    setOrder((prev) => {
      const incoming = new Set(orderedIds);
      const sameMembership =
        prev.length === orderedIds.length && prev.every((id) => incoming.has(id));
      if (sameMembership) return prev;
      // Preserve the user's current ordering for ids that still exist, then
      // append any new ids in their incoming order.
      const kept = prev.filter((id) => incoming.has(id));
      const added = orderedIds.filter((id) => !prev.includes(id));
      return [...kept, ...added];
    });
  }, [orderedIds]);

  const commitOrder = (next: string[]) => {
    // Map the new positions to priority buckets and persist only the changes.
    let changed = 0;
    next.forEach((id, idx) => {
      const dl = byId.current.get(id);
      if (!dl) return;
      const target = bucketForIndex(idx, next.length);
      if ((dl.priority ?? "normal") !== target) {
        onSetPriority(id, target);
        changed++;
      }
    });
    if (changed > 0) {
      toast.success(
        `Reordered queue — updated ${changed} priorit${
          changed === 1 ? "y" : "ies"
        }`,
      );
    }
  };

  return (
    <Reorder.Group
      axis="y"
      values={order}
      onReorder={setOrder}
      className="space-y-2"
      // Disable layout animation entirely under reduced-motion.
      layoutScroll
    >
      {order.map((id) => {
        const dl = byId.current.get(id);
        if (!dl) return null;
        return (
          <ReorderRow
            key={id}
            download={dl}
            orderedIds={order}
            selectionActive={selectionActive}
            reduceMotion={reduceMotion ?? false}
            onCommit={() => commitOrder(order)}
            onSetPriority={onSetPriority}
            {...actions}
          />
        );
      })}
    </Reorder.Group>
  );
}

interface ReorderRowProps {
  download: Download;
  orderedIds: string[];
  selectionActive: boolean;
  reduceMotion: boolean;
  onCommit: () => void;
  onPause: (id: string) => void;
  onResume: (id: string) => void;
  onStart: (id: string) => void;
  onDelete: (id: string) => void;
  onRetry: (id: string) => void;
  onSetPriority: (id: string, priority: DownloadPriority) => void;
}

function ReorderRow({
  download,
  orderedIds,
  selectionActive,
  reduceMotion,
  onCommit,
  ...actions
}: ReorderRowProps) {
  const controls = useDragControls();

  return (
    <Reorder.Item
      value={download.id}
      dragListener={false}
      dragControls={controls}
      onDragEnd={onCommit}
      // Respect reduced motion: skip the layout spring, drop instantly.
      transition={reduceMotion ? { duration: 0 } : undefined}
      className="relative"
    >
      <DownloadItemWrapper
        download={download}
        orderedIds={orderedIds}
        selectionActive={selectionActive}
        dragHandle={
          <CardDragHandle onPointerDown={(e) => controls.start(e)} />
        }
        {...actions}
      />
    </Reorder.Item>
  );
}
