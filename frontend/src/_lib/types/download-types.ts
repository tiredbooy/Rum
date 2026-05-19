
export type DownloadStatus = "active" | "completed" | "failed" | "paused" | "queued" | "pending";

export interface Download {
  id: string;
  fileName: string;
  url: string;
  status: DownloadStatus;
  progress: number; // 0-100
  speed: string; // e.g. "2.4 MB/s"
  size: string; // total size e.g. "1.2 GB"
  eta: string; // e.g. "3m 12s"
  addedAt: string; // ISO string
}