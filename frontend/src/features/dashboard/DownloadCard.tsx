import type { Download } from "@/_lib/types/download-types";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { cn } from "@/lib/utils";
import { formatBytes, formatSpeed, formatETA } from "@/_lib/utils/format";
import {
  AlertCircle,
  CheckCircle2,
  Clock,
  Pause,
  Play,
  RotateCcw,
  X,
} from "lucide-react";

interface DownloadCardProps {
  download: Download;
  onPause?: (id: string) => void;
  onResume?: (id: string) => void;
  onStart?: (id: string) => void;
  onDelete?: (id: string) => void;
  onRetry?: (id: string) => void;
}

const statusConfig: Record<
  Download["status"],
  { icon: React.ElementType; colorClass: string; label: string }
> = {
  running: { icon: Clock, colorClass: "text-cyan-400", label: "Downloading" },
  completed: {
    icon: CheckCircle2,
    colorClass: "text-emerald-400",
    label: "Completed",
  },
  failed: {
    icon: AlertCircle,
    colorClass: "text-destructive",
    label: "Failed",
  },
  paused: { icon: Pause, colorClass: "text-amber-400", label: "Paused" },
  pending: {
    icon: Clock,
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
}: DownloadCardProps) {
  const { id, filename, status, progress, speed, total_size, remaining } =
    download;
  const StatusIcon = statusConfig[status].icon;

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
              statusConfig[status].colorClass,
              "bg-background/50",
            )}
          >
            <StatusIcon className="w-3 h-3" />
            {statusConfig[status].label}
          </span>
          {total_size && (
            <span className="text-[10px] text-muted-foreground hidden sm:inline">
              {formatBytes(total_size)}
            </span>
          )}
        </div>

        {(status === "running" || status === "paused") && (
          <div className="space-y-1">
            <Progress value={progress ?? 0} className="h-1.5" />
            <div className="flex justify-between text-[10px] text-muted-foreground">
              <span>{progress?.toFixed(1) ?? 0}%</span>
              <span>
                {speed != null ? formatSpeed(speed) : "—"} ·{" "}
                {remaining != null ? formatETA(remaining) : "—"} left
              </span>
            </div>
          </div>
        )}
      </div>

      {/* Action buttons – top right, visible on hover */}
      <div className="absolute top-3 right-3 flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
        {/* Pending: start + delete */}
        {status === "pending" && (
          <>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              onClick={() => onStart?.(id)}
              title="Start"
            >
              <Play className="w-4 h-4" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-destructive"
              onClick={() => onDelete?.(id)}
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
              title="Pause"
            >
              <Pause className="w-4 h-4" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-destructive"
              onClick={() => onDelete?.(id)}
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
              title="Resume"
            >
              <Play className="w-4 h-4" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-destructive"
              onClick={() => onDelete?.(id)}
              title="Delete"
            >
              <X className="w-4 h-4" />
            </Button>
          </>
        )}

        {/* Failed: retry + delete */}
        {status === "failed" && (
          <>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              onClick={() => onRetry?.(id)}
              title="Retry"
            >
              <RotateCcw className="w-4 h-4" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-destructive"
              onClick={() => onDelete?.(id)}
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
            title="Delete"
          >
            <X className="w-4 h-4" />
          </Button>
        )}
      </div>
    </Card>
  );
}
