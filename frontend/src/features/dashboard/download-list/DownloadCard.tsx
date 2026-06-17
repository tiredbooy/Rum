import type { Download, DownloadPriority } from "@/_lib/types/download-types";
import { formatBytes, formatETA, formatSpeed } from "@/_lib/utils/format";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { cn } from "@/lib/utils";
import { useSpeedHistory } from "@/hooks/useSpeedHistory";
import {
  AlertCircle,
  CheckCircle2,
  Clock,
  ClockFadingIcon,
  HelpCircle,
  Pause,
  Play,
  RotateCcw,
  X,
} from "lucide-react";
import { ElementType } from "react";
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

export function DownloadCard({
  download,
  onPause,
  onResume,
  onStart,
  onDelete,
  onRetry,
  onSetPriority,
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

  return (
    <Card className="group hover:shadow-md transition-shadow p-3 flex gap-3 relative">
      {/* File extension icon */}
      <div className="shrink-0 h-8 w-8 rounded bg-muted flex items-center justify-center text-[10px] font-mono uppercase text-muted-foreground">
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
            <Progress value={safeProgress} className="h-1.5" />
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
