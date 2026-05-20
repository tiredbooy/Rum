import { DownloadCard } from "./DownloadCard";
import { useJobProgress } from "@/hooks/useJobProgress";
import type { Download } from "@/_lib/types/download-types";

interface DownloadItemWrapperProps {
  download: Download;
  onPause: (id: string) => void;
  onResume: (id: string) => void;
  onStart: (id: string) => void;
  onDelete: (id: string) => void;
  onRetry: (id: string) => void;
}

export function DownloadItemWrapper({
  download,
  ...actions
}: DownloadItemWrapperProps) {
  const isRunning = download.status === "running";
  const { progress: liveData } = useJobProgress(
    isRunning ? download.id : undefined,
  );

  const merged =
    isRunning && liveData ? { ...download, ...liveData } : download;

  return <DownloadCard download={merged} {...actions} />;
}
