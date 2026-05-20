import { useState, useEffect, useMemo } from "react";
import { DownloadList } from "./DownloadList";
import type { Download, DownloadStatus } from "@/_lib/types/download-types";
import { useDownloads } from "@/_lib/services/queries/download.queries";

interface DownloadStatusPageProps {
  status: Download["status"] | "all";
  title?: string;
}

export function DownloadStatusPage({ status, title }: DownloadStatusPageProps) {
  const { data: downloads, isLoading } = useDownloads(status);

  const handlePause = (id: string) => console.log("Pause", id);
  const handleResume = (id: string) => console.log("Resume", id);
  const handleCancel = (id: string) => console.log("Cancel", id);
  const handleRetry = (id: string) => console.log("Retry", id);

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
        onPause={handlePause}
        onResume={handleResume}
        onCancel={handleCancel}
        onRetry={handleRetry}
      />
    </div>
  );
}
