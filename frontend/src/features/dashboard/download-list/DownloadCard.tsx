import type { Download, DownloadPriority } from "@/_lib/types/download-types";
import { formatBytes, formatETA, formatSpeed } from "@/_lib/utils/format";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { cn } from "@/lib/utils";
import { useSpeedHistory } from "@/hooks/useSpeedHistory";
import { DownloadProgress } from "./DownloadProgress";
import {
  AlertCircle,
  CheckCircle2,
  Clock,
  ClockFadingIcon,
  GripVertical,
  HelpCircle,
  Pause,
  Play,
  RotateCcw,
  X,
} from "lucide-react";
import { ElementType, memo, type ReactNode } from "react";
import { SpeedSparkline } from "./SpeedSparkline";
import { PriorityBadge } from "./PriorityBadge";
import { CategoryBadge } from "./CategoryBadge";
import { CardActionsMenu } from "./CardActionsMenu";

interface DownloadCardProps {
  download: Download;
  onPause?: (id: string) => void;
  onResume?: (id: string) => void;
  onStart?: (id: string) => void;
  onDelete?: (id: string) => void;
  onRetry?: (id: string) => void;
  onSetPriority?: (id: string, priority: DownloadPriority) => void;
  /** Whether this row is currently selected for bulk actions. */
  selected?: boolean;
  /** True when any selection is active (keeps checkboxes visible, not hover-only). */
  selectionActive?: boolean;
  /** Toggle/extend selection. `modifiers` carries shift/meta for range/ctrl select. */
  onSelectChange?: (
    id: string,
    selected: boolean,
    modifiers: { shift: boolean; meta: boolean },
  ) => void;
  /**
   * Optional drag handle (rendered when the list is reorderable). The handle is
   * wired by the parent Reorder.Item so the card stays presentational.
   */
  dragHandle?: ReactNode;
}

function getStatusInfo(status: Download["status"]) {
  return (
    statusConfig[status] ?? {
      icon: HelpCircle,
      colorClass: "text-muted-foreground",
      label: status,
    }
  );
}

const statusConfig: Record<
  Download["status"],
  { icon: ElementType; colorClass: string; label: string }
> = {
  running: { icon: Clock, colorClass: "text-cyan-400", label: "Downloading" },
  completed: {
    icon: CheckCircle2,
    colorClass: "text-emerald-400",
    label: "Completed",
  },
  error: {
    icon: AlertCircle,
    colorClass: "text-destructive",
    label: "Error",
  },
  paused: { icon: Pause, colorClass: "text-amber-400", label: "Paused" },
  pending: {
    icon: ClockFadingIcon,
    colorClass: "text-muted-foreground",
    label: "Pending",
  },
};

function DownloadCardImpl({
  download,
  onPause,
  onResume,
  onStart,
  onDelete,
  onRetry,
  onSetPriority,
  selected = false,
  selectionActive = false,
  onSelectChange,
  dragHandle,
}: DownloadCardProps) {
  const {
    id,
    filename,
    status,
    progress,
    speed,
    total_size,
    eta,
    error,
    priority,
    category,
  } = download;

  const {
    icon: StatusIcon,
    colorClass: statusColor,
    label: statusLabel,
  } = getStatusInfo(status);

  // Live speed history for the sparkline (only while actively downloading).
  const speedSamples = useSpeedHistory(speed, status === "running");

  // Backend may emit bogus progress (negative, or >100 when total size is
  // unknown / -1). Clamp to [0, 100] so the bar never overflows or goes blank.
  const safeProgress = Math.min(
    100,
    Math.max(0, Number.isFinite(progress) ? (progress as number) : 0),
  );
  // total_size is only meaningful when > 0; the backend uses -1/0 for unknown.
  const hasKnownSize = typeof total_size === "number" && total_size > 0;

  // Row click selection: ctrl/cmd toggles this row, shift extends a range.
  // A plain click does nothing here (so the row stays clickable for buttons /
  // text selection); the checkbox is the primary plain-select affordance.
  const handleRowClick = (e: React.MouseEvent<HTMLDivElement>) => {
    if (!onSelectChange) return;
    if (!(e.ctrlKey || e.metaKey || e.shiftKey)) return;
    // Don't hijack clicks on interactive children (buttons, menu, links).
    if ((e.target as HTMLElement).closest("button,a,[role='menuitem']")) return;
    e.preventDefault();
    onSelectChange(id, !selected, { shift: e.shiftKey, meta: e.ctrlKey || e.metaKey });
  };

  return (
    <Card
      onClick={handleRowClick}
      data-selected={selected || undefined}
      className={cn(
        "group hover:shadow-md transition-shadow p-3 flex gap-3 relative",
        selected && "ring-2 ring-primary/60 bg-primary/5",
      )}
    >
      {/* Optional drag handle (reorderable lists only). */}
      {dragHandle}

      {/* Selection checkbox — visible on hover or whenever a selection is active. */}
      <div
        className={cn(
          "absolute left-1 top-1/2 -translate-y-1/2 z-10 transition-opacity",
          selected || selectionActive
            ? "opacity-100"
            : "opacity-0 group-hover:opacity-100 focus-within:opacity-100",
        )}
        onClick={(e) => e.stopPropagation()}
      >
        <Checkbox
          checked={selected}
          aria-label={selected ? `Deselect ${filename}` : `Select ${filename}`}
          onCheckedChange={(v) =>
            onSelectChange?.(id, v === true, { shift: false, meta: false })
          }
        />
      </div>

      {/* File extension icon */}
      <div
        className={cn(
          "shrink-0 h-8 w-8 rounded bg-muted flex items-center justify-center text-[10px] font-mono uppercase text-muted-foreground transition-[margin]",
          (selected || selectionActive) && "ml-5",
        )}
      >
        {filename?.split(".").pop()?.slice(0, 3) ?? "?"}
      </div>

      <div className="flex-1 min-w-0 space-y-1.5 pr-12">
        <div className="flex items-center gap-2">
          <p className="text-sm font-medium truncate">{filename}</p>
          <span
            className={cn(
              "inline-flex items-center gap-1 rounded-full px-1.5 py-0.5 text-[10px] font-medium border",
              statusColor,
              "bg-background/50",
            )}
          >
            <StatusIcon className="w-3 h-3" />
            {statusLabel}
          </span>
          {hasKnownSize ? (
            <span className="text-[10px] text-muted-foreground hidden sm:inline">
              {formatBytes(total_size as number)}
            </span>
          ) : (
            <span className="text-[10px] text-muted-foreground hidden sm:inline">
              Unknown size
            </span>
          )}
          {status !== "completed" && (
            <PriorityBadge priority={priority} className="hidden sm:inline-flex" />
          )}
          <CategoryBadge category={category} className="hidden sm:inline-flex" />
        </div>

        {(status === "running" || status === "paused") && (
          <div className="space-y-1">
            <DownloadProgress
              progress={safeProgress}
              active={status === "running"}
              stalled={status === "running" && (speed ?? 0) <= 0}
            />
            <div className="flex items-center justify-between gap-2 text-[10px] text-muted-foreground">
              <span>{hasKnownSize ? `${safeProgress.toFixed(1)}%` : "—"}</span>
              <div className="flex items-center gap-2">
                {status === "running" && speedSamples.length > 1 && (
                  <SpeedSparkline samples={speedSamples} />
                )}
                <span>
                  {speed != null ? formatSpeed(speed) : "—"} ·{" "}
                  {formatETA(eta ?? 0)}
                </span>
              </div>
            </div>
          </div>
        )}

        {status === "error" && error && (
          <p
            className="text-[10px] text-destructive truncate"
            title={error}
          >
            {error}
          </p>
        )}
      </div>

      {/* Action buttons – top right, visible on hover */}
      <div className="absolute top-3 right-3 flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
        {/* Overflow menu: priority + copy link / open folder */}
        <CardActionsMenu download={download} onSetPriority={onSetPriority} />

        {/* Pending: start + delete */}
        {status === "pending" && (
          <>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              onClick={() => onStart?.(id)}
              aria-label="Start"
              title="Start"
            >
              <Play className="w-4 h-4" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-destructive"
              onClick={() => onDelete?.(id)}
              aria-label="Delete"
              title="Delete"
            >
              <X className="w-4 h-4" />
            </Button>
          </>
        )}

        {/* Running: pause + cancel/delete */}
        {status === "running" && (
          <>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              onClick={() => onPause?.(id)}
              aria-label="Pause"
              title="Pause"
            >
              <Pause className="w-4 h-4" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-destructive"
              onClick={() => onDelete?.(id)}
              aria-label="Delete"
              title="Delete"
            >
              <X className="w-4 h-4" />
            </Button>
          </>
        )}

        {/* Paused: resume + delete */}
        {status === "paused" && (
          <>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              onClick={() => onResume?.(id)}
              aria-label="Resume"
              title="Resume"
            >
              <Play className="w-4 h-4" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-destructive"
              onClick={() => onDelete?.(id)}
              aria-label="Delete"
              title="Delete"
            >
              <X className="w-4 h-4" />
            </Button>
          </>
        )}

        {status === "error" && (
          <>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              onClick={() => onRetry?.(id)}
              aria-label="Retry"
              title="Retry"
            >
              <RotateCcw className="w-4 h-4" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-destructive"
              onClick={() => onDelete?.(id)}
              aria-label="Delete"
              title="Delete"
            >
              <X className="w-4 h-4" />
            </Button>
          </>
        )}

        {/* Completed: only delete */}
        {status === "completed" && (
          <Button
            variant="ghost"
            size="icon"
            className="h-7 w-7 text-destructive"
            onClick={() => onDelete?.(id)}
            aria-label="Delete"
              title="Delete"
          >
            <X className="w-4 h-4" />
          </Button>
        )}
      </div>
    </Card>
  );
}

/**
 * Memoized to avoid re-render storms across many cards: with live SSE patches
 * only the changed download's props differ, so unaffected cards skip rendering.
 */
export const DownloadCard = memo(DownloadCardImpl);

/** A small, accessible drag handle for reorderable rows. */
export function CardDragHandle({
  onPointerDown,
  className,
}: {
  onPointerDown: (e: React.PointerEvent) => void;
  className?: string;
}) {
  return (
    <button
      type="button"
      aria-label="Drag to reorder"
      title="Drag to reorder"
      onPointerDown={onPointerDown}
      onClick={(e) => e.stopPropagation()}
      className={cn(
        "absolute -left-0.5 top-1/2 -translate-y-1/2 z-10 flex h-6 w-4 cursor-grab touch-none items-center justify-center text-muted-foreground/50 opacity-0 transition-opacity hover:text-muted-foreground group-hover:opacity-100 active:cursor-grabbing",
        className,
      )}
    >
      <GripVertical className="h-4 w-4" />
    </button>
  );
}
