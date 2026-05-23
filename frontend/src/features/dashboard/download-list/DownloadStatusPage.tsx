import { useState, useEffect, useMemo } from "react";
import { DownloadList } from "./DownloadList";
import type { Download, DownloadStatus } from "@/_lib/types/download-types";
import {
  useDeleteDownload,
  useDownloads,
  usePauseDownload,
  useResumeDownload,
  useStartDownload,
} from "@/_lib/services/queries/download.queries";
import { toast } from "sonner";

interface DownloadStatusPageProps {
  status: Download["status"] | "all";
  title?: string;
}

export function DownloadStatusPage({ status, title }: DownloadStatusPageProps) {
  const { data: downloads, isLoading } = useDownloads(status);
  const startDownload = useStartDownload();
  const pauseDownload = usePauseDownload();
  const resumeDownload = useResumeDownload();
  const deleteDownload = useDeleteDownload();

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
    resumeDownload.mutateAsync(id);
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
        onStart={handleStart}
        onPause={handlePause}
        onResume={handleResume}
        onDelete={handleDelete}
        onRetry={handleRetry}
      />
    </div>
  );
}
