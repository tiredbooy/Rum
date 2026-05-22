export type DownloadStatus =
  | "running"
  | "completed"
  | "error"
  | "paused"
  | "pending";

export interface Download {
  id: string;
  url: string;
  filename?: string;
  status: DownloadStatus;
  progress?: number;
  downloaded: number;
  total_size?: number;
  speed?: number;
  remaining: number;
  error?: string;
  created_at?: string;
  completed_at?: string;
}

export interface DownloadReq {
  urls: string[];
  dest_path?: string;
  filename?: string;
  speed_limit?: number;
  user_agent?: string;
  referer?: string;
  group_folder?: string;
  max_retries?: string;
  auto_start?: boolean;
}
