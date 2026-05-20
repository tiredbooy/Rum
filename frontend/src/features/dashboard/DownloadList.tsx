import type { Download } from "@/_lib/types/download-types";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Skeleton } from "@/components/ui/skeleton";
import { DownloadItemWrapper } from "./DownloadWrapper";

interface DownloadListProps {
  downloads?: Download[];
  isLoading?: boolean;
  onPause: (id: string) => void;
  onResume: (id: string) => void;
  onStart: (id: string) => void;
  onDelete: (id: string) => void;
  onRetry: (id: string) => void;
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
  onPause,
  onResume,
  onStart,
  onDelete,
  onRetry,
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

  if (downloads?.length === 0) {
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
          />
        ))}
      </div>
    </ScrollArea>
  );
}
