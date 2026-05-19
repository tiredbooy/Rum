import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Pause, Play, X, RotateCcw } from "lucide-react";
import { StatusBadge } from "./StatusBadge";
import { DownloadProgress } from "./DownloadProgress";
import type { Download } from "@/_lib/types/download-types";

interface DownloadItemProps {
  download: Download;
  onPause: (id: string) => void;
  onResume: (id: string) => void;
  onCancel: (id: string) => void;
  onRetry: (id: string) => void;
}

export function DownloadItem({
  download,
  onPause,
  onResume,
  onCancel,
  onRetry,
}: DownloadItemProps) {
  const { id, fileName, status, progress, speed, eta, size } = download;
  const isActive = status === "active";
  const isPaused = status === "paused";
  const isFailed = status === "failed";

  return (
    <Card className="p-3 flex items-center gap-3 hover:bg-muted/30 transition-colors group">
      {/* File type icon */}
      <div className="shrink-0 h-8 w-8 rounded bg-muted flex items-center justify-center text-[10px] font-mono uppercase">
        {fileName.split(".").pop()?.slice(0, 3) ?? "?"}
      </div>

      {/* Main content */}
      <div className="flex-1 min-w-0">
        <div className="flex items-center justify-between gap-2">
          <p className="text-sm font-medium truncate">{fileName}</p>
          <StatusBadge status={status} />
        </div>
        <div className="text-xs text-muted-foreground mt-0.5">{size}</div>
        <div className="mt-1.5">
          <DownloadProgress progress={progress} speed={speed} eta={eta} />
        </div>
      </div>

      {/* Action buttons – visible on hover */}
      <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
        {isActive && (
          <Button
            variant="ghost"
            size="icon"
            className="h-7 w-7"
            onClick={() => onPause(id)}
          >
            <Pause className="h-4 w-4" />
          </Button>
        )}
        {isPaused && (
          <Button
            variant="ghost"
            size="icon"
            className="h-7 w-7"
            onClick={() => onResume(id)}
          >
            <Play className="h-4 w-4" />
          </Button>
        )}
        {isFailed && (
          <Button
            variant="ghost"
            size="icon"
            className="h-7 w-7"
            onClick={() => onRetry(id)}
          >
            <RotateCcw className="h-4 w-4" />
          </Button>
        )}
        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7"
          onClick={() => onCancel(id)}
        >
          <X className="h-4 w-4" />
        </Button>
      </div>
    </Card>
  );
}
