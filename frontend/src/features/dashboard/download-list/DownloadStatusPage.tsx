import { DownloadList } from "./DownloadList";
import type {
  Download,
  DownloadPriority,
} from "@/_lib/types/download-types";
import {
  useDownloads,
  usePauseDownload,
  useResumeDownload,
  useRetryDownload,
  useSetDownloadPriority,
  useStartDownload,
} from "@/_lib/services/queries/download.queries";
import { useUndoableDelete } from "@/_lib/services/queries/undo-delete.queries";

interface DownloadStatusPageProps {
  status: Download["status"] | "all";
  title?: string;
}

export function DownloadStatusPage({ status, title }: DownloadStatusPageProps) {
  const { data: downloads, isLoading, isError, error, refetch } =
    useDownloads(status);
  const startDownload = useStartDownload();
  const pauseDownload = usePauseDownload();
  const resumeDownload = useResumeDownload();
  const { removeIds } = useUndoableDelete();
  const retryDownload = useRetryDownload();
  const setPriority = useSetDownloadPriority();

  const handleStart = (id: string) => {
    startDownload.mutateAsync(id);
  };
  const handlePause = (id: string) => {
    pauseDownload.mutateAsync(id);
  };
  const handleResume = (id: string) => {
    resumeDownload.mutateAsync(id);
  };
  // Act now, undo from the toast. The X button removes from the list and keeps
  // the file; "Delete file" lives in the card's overflow menu.
  const handleDelete = (id: string) => {
    void removeIds([id]);
  };
  const handleRetry = (id: string) => {
    retryDownload.mutate(id);
  };
  const handleSetPriority = (id: string, priority: DownloadPriority) => {
    setPriority.mutate({ id, priority });
  };

  const displayTitle =
    title ??
    (status === "all"
      ? "All Downloads"
      : `${status.charAt(0).toUpperCase() + status.slice(1)} Downloads`);

  // Drag-to-reorder is queue-like: only the pending and active lists, where
  // vertical position maps to a priority bucket. Completed/error/all stay static.
  const reorderable = status === "pending" || status === "running";

  return (
    <div className="space-y-4 p-4">
      <h1 className="text-xl font-bold">{displayTitle}</h1>
      <DownloadList
        downloads={downloads}
        isLoading={isLoading}
        isError={isError}
        error={error}
        onRetryFetch={() => refetch()}
        onStart={handleStart}
        onPause={handlePause}
        onResume={handleResume}
        onDelete={handleDelete}
        onRetry={handleRetry}
        onSetPriority={handleSetPriority}
        reorderable={reorderable}
      />

    </div>
  );
}
