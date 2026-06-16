import { DownloadCard } from "./DownloadCard";
import { useJobProgress } from "@/hooks/useJobProgress";
import type { Download, DownloadPriority } from "@/_lib/types/download-types";

interface DownloadItemWrapperProps {
  download: Download;
  onPause: (id: string) => void;
  onResume: (id: string) => void;
  onStart: (id: string) => void;
  onDelete: (id: string) => void;
  onRetry: (id: string) => void;
  onSetPriority: (id: string, priority: DownloadPriority) => void;
}

export function DownloadItemWrapper({ download, ...actions }: DownloadItemWrapperProps) {
  const isRunning = download.status === "running";

  const { data: liveData } = useJobProgress(
    download.id,          
    download,         
    isRunning        
  );

  const merged = liveData ?? download;

  return <DownloadCard download={merged} {...actions} />;
}