import { DownloadList } from "./DownloadList";
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
  const handleDelete = (id: string) => {
    deleteDownload.mutateAsync(id);
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
      />
    </div>
  );
}
