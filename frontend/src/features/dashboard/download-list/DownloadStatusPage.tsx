import { useState } from "react";
import { DownloadList } from "./DownloadList";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import type {
  Download,
  DownloadPriority,
} from "@/_lib/types/download-types";
import {
  useDeleteDownload,
  useDownloads,
  usePauseDownload,
  useResumeDownload,
  useRetryDownload,
  useSetDownloadPriority,
  useStartDownload,
} from "@/_lib/services/queries/download.queries";

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
  const deleteDownload = useDeleteDownload();
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
  // Single-card delete removes the file from disk, so confirm first (the bulk
  // actions already confirm). handleDelete just arms the dialog.
  const [pendingDeleteId, setPendingDeleteId] = useState<string | null>(null);
  const handleDelete = (id: string) => setPendingDeleteId(id);
  const confirmDelete = () => {
    if (pendingDeleteId) deleteDownload.mutateAsync(pendingDeleteId);
    setPendingDeleteId(null);
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

      <AlertDialog
        open={pendingDeleteId !== null}
        onOpenChange={(o) => {
          if (!o) setPendingDeleteId(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete this download?</AlertDialogTitle>
            <AlertDialogDescription>
              This removes the download and deletes its file from disk. This
              can't be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={confirmDelete}>
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
