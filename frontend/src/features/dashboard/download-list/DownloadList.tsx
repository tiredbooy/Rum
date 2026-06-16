import type { Download, DownloadPriority } from "@/_lib/types/download-types";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Skeleton } from "@/components/ui/skeleton";
import { AlertCircle } from "lucide-react";
import { DownloadItemWrapper } from "./DownloadWrapper";

interface DownloadListProps {
  downloads?: Download[];
  isLoading?: boolean;
  isError?: boolean;
  error?: unknown;
  onRetryFetch?: () => void;
  onPause: (id: string) => void;
  onResume: (id: string) => void;
  onStart: (id: string) => void;
  onDelete: (id: string) => void;
  onRetry: (id: string) => void;
  onSetPriority: (id: string, priority: DownloadPriority) => void;
}

function SkeletonItem() {
  return (
    <div className="flex items-center gap-3 p-3 border rounded-lg">
      <Skeleton className="h-8 w-8 rounded" />
      <div className="flex-1 space-y-2">
        <Skeleton className="h-4 w-3/4" />
        <Skeleton className="h-3 w-1/2" />
      </div>
    </div>
  );
}

export function DownloadList({
  downloads,
  isLoading,
  isError,
  error,
  onRetryFetch,
  onPause,
  onResume,
  onStart,
  onDelete,
  onRetry,
  onSetPriority,
}: DownloadListProps) {
  if (isLoading) {
    return (
      <div className="space-y-2 p-4">
        {Array.from({ length: 3 }).map((_, i) => (
          <SkeletonItem key={i} />
        ))}
      </div>
    );
  }

  if (isError) {
    const message =
      error instanceof Error ? error.message : "Failed to load downloads";
    return (
      <div className="flex flex-col items-center justify-center gap-3 py-16 text-center text-muted-foreground">
        <AlertCircle className="h-8 w-8 text-destructive" />
        <p className="text-sm">{message}</p>
        {onRetryFetch && (
          <Button variant="outline" size="sm" onClick={onRetryFetch}>
            Try again
          </Button>
        )}
      </div>
    );
  }

  if (!downloads || downloads.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-16 text-muted-foreground">
        <p className="text-sm">No downloads here yet</p>
      </div>
    );
  }

  return (
    <ScrollArea className="h-[calc(100vh-16rem)] pr-2">
      <div className="space-y-2">
        {downloads?.map((dl) => (
          <DownloadItemWrapper
            key={dl.id}
            download={dl}
            onPause={onPause}
            onResume={onResume}
            onStart={onStart}
            onDelete={onDelete}
            onRetry={onRetry}
            onSetPriority={onSetPriority}
          />
        ))}
      </div>
    </ScrollArea>
  );
}
