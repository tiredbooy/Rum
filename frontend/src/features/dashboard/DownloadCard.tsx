import type { Download } from "@/_lib/types/download-types";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { cn } from "@/lib/utils";
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
  onCancel?: (id: string) => void;
  onRetry?: (id: string) => void;
}

const statusConfig: Record<
  Download["status"],
  { icon: React.ElementType; colorClass: string; label: string }
> = {
  active: { icon: Clock, colorClass: "text-cyan-400", label: "Downloading" },
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
  queued: { icon: Clock, colorClass: "text-muted-foreground", label: "Queued" },
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
  onCancel,
  onRetry,
}: DownloadCardProps) {
  const { id, fileName, status, progress, speed, size, eta } = download;
  const StatusIcon = statusConfig[status].icon;

  return (
    <Card className="group hover:shadow-md transition-shadow p-3 flex gap-3 relative">
      <div className="shrink-0 h-8 w-8 rounded bg-muted flex items-center justify-center text-[10px] font-mono uppercase text-muted-foreground">
        {fileName.split(".").pop()?.slice(0, 3) ?? "?"}
      </div>

      <div className="flex-1 min-w-0 space-y-1.5 pr-12">
        <div className="flex items-center gap-2">
          <p className="text-sm font-medium truncate">{fileName}</p>
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
          {size && (
            <span className="text-[10px] text-muted-foreground hidden sm:inline">
              {size}
            </span>
          )}
        </div>

        {(status === "active" || status === "paused") && (
          <div className="space-y-1">
            <Progress value={progress} className="h-1.5" />
            <div className="flex justify-between text-[10px] text-muted-foreground">
              <span>{progress.toFixed(1)}%</span>
              <span>
                {speed} · {eta} left
              </span>
            </div>
          </div>
        )}
      </div>

      <div className="absolute top-3 right-3 flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
        {status === "active" && (
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
              onClick={() => onCancel?.(id)}
              title="Cancel"
            >
              <X className="w-4 h-4" />
            </Button>
          </>
        )}
        {status === "paused" && (
          <Button
            variant="ghost"
            size="icon"
            className="h-7 w-7"
            onClick={() => onResume?.(id)}
            title="Resume"
          >
            <Play className="w-4 h-4" />
          </Button>
        )}
        {status === "failed" && (
          <Button
            variant="ghost"
            size="icon"
            className="h-7 w-7"
            onClick={() => onRetry?.(id)}
            title="Retry"
          >
            <RotateCcw className="w-4 h-4" />
          </Button>
        )}
      </div>
    </Card>
  );
}
