
import { useState, useEffect, useMemo } from "react";
import { DownloadList } from "./DownloadList";
import type { Download } from "@/_lib/types/download-types";

const mockDownloads: Download[] = [
  {
    id: "1",
    fileName: "ubuntu-24.04-desktop-amd64.iso",
    url: "https://releases.ubuntu.com/...",
    status: "paused",
    progress: 67,
    speed: "8.4 MB/s",
    size: "5.7 GB",
    eta: "2m 34s",
    addedAt: new Date().toISOString(),
  },
  {
    id: "2",
    fileName: "react-docs.pdf",
    url: "https://example.com/react-docs.pdf",
    status: "pending",
    progress: 42,
    speed: "3.1 MB/s",
    size: "12.8 MB",
    eta: "2s",
    addedAt: new Date().toISOString(),
  },
  {
    id: "3",
    fileName: "dataset_2025.zip",
    url: "https://example.com/data/dataset_2025.zip",
    status: "active",
    progress: 12,
    speed: "1.2 MB/s",
    size: "2.3 GB",
    eta: "27m 11s",
    addedAt: new Date().toISOString(),
  },
];

interface DownloadStatusPageProps {
  status: Download["status"] | "all";
  title?: string;
}

export function DownloadStatusPage({ status, title }: DownloadStatusPageProps) {
  const [isLoading, setIsLoading] = useState(true);
  const [downloads, setDownloads] = useState<Download[]>([]);

  useEffect(() => {
    setIsLoading(true);
    const timer = setTimeout(() => {
      setDownloads(mockDownloads);
      setIsLoading(false);
    }, 800);
    return () => clearTimeout(timer);
  }, [status]); 

  const filteredDownloads = useMemo(() => {
    if (status === "all") return downloads;
    return downloads.filter((d) => d.status === status);
  }, [downloads, status]);

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
        downloads={filteredDownloads}
        isLoading={isLoading}
        onPause={handlePause}
        onResume={handleResume}
        onCancel={handleCancel}
        onRetry={handleRetry}
      />
    </div>
  );
}
