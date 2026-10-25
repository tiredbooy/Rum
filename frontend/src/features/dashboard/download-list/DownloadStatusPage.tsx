import { useMemo, useState } from "react";
import { filterAndSort, listViewState } from "../list-view/list-view";
import { NoResults } from "../list-view/NoResults";
import { useListView } from "../list-view/useListView";
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

  const query = useListView((s) => s.query);
  const tab = useListView((s) => s.tab);
  const sort = useListView((s) => s.sort);
  const view = useMemo(() => ({ query, tab, sort }), [query, tab, sort]);
  const shown = useMemo(
    () => (downloads ? filterAndSort(downloads, view) : undefined),
    [downloads, view],
  );
  const noResults =
    shown !== undefined &&
    listViewState(downloads?.length ?? 0, shown.length, view) === "no-results";

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
  // A searched or re-sorted list no longer maps position to priority.
  const reorderable =
    (status === "pending" || status === "running") &&
    sort === "default" &&
    query.trim() === "";

  return (
    <div className="space-y-4 p-4">
      <h1 className="text-xl font-bold">{displayTitle}</h1>
      {noResults ? (
        <NoResults />
      ) : (
      <DownloadList
        downloads={shown}
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
      )}

    </div>
  );
}
