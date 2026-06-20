import { useCallback, type ReactNode } from "react";
import { DownloadCard } from "./DownloadCard";
import { useJobProgress } from "@/hooks/useJobProgress";
import type { Download, DownloadPriority } from "@/_lib/types/download-types";
import { useSelection } from "../selection/useSelection";

interface DownloadItemWrapperProps {
  download: Download;
  onPause: (id: string) => void;
  onResume: (id: string) => void;
  onStart: (id: string) => void;
  onDelete: (id: string) => void;
  onRetry: (id: string) => void;
  onSetPriority: (id: string, priority: DownloadPriority) => void;
  /** Ordered id list of the rendered group, for shift-range selection. */
  orderedIds: string[];
  /** True when any selection is active (keeps checkboxes shown). */
  selectionActive: boolean;
  /** Optional drag handle injected by a reorderable list. */
  dragHandle?: ReactNode;
}

/**
 * Bridges live SSE progress and the selection store to a single card. Selection
 * membership is read with a granular selector (`has(id)`) so toggling one row
 * does not re-render every other wrapper.
 */
export function DownloadItemWrapper({
  download,
  orderedIds,
  selectionActive,
  dragHandle,
  ...actions
}: DownloadItemWrapperProps) {
  const isRunning = download.status === "running";

  const { data: liveData } = useJobProgress(download.id, download, isRunning);

  const merged = liveData ?? download;

  const selected = useSelection((s) => s.selectedIds.has(download.id));
  const toggle = useSelection((s) => s.toggle);
  const selectRange = useSelection((s) => s.selectRange);
  const setSelected = useSelection((s) => s.setSelected);

  const onSelectChange = useCallback(
    (
      id: string,
      next: boolean,
      modifiers: { shift: boolean; meta: boolean },
    ) => {
      if (modifiers.shift) {
        selectRange(id, orderedIds);
      } else if (modifiers.meta) {
        toggle(id);
      } else {
        setSelected(id, next);
      }
    },
    [orderedIds, selectRange, toggle, setSelected],
  );

  return (
    <DownloadCard
      download={merged}
      {...actions}
      selected={selected}
      selectionActive={selectionActive}
      onSelectChange={onSelectChange}
      dragHandle={dragHandle}
    />
  );
}
