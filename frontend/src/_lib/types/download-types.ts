export type DownloadStatus =
  | "running"
  | "completed"
  | "error"
  | "paused"
  | "pending";

export type DownloadPriority = "low" | "normal" | "high";

export interface Download {
  id: string;
  url: string;
  filename?: string;
  status: DownloadStatus;
  progress?: number;
  downloaded: number;
  total_size?: number;
  eta?: number;
  speed?: number;
  remaining: number;
  error?: string;
  priority?: DownloadPriority;
  dest_path?: string;
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

export interface DashboardStats {
  active_downloads: number;
  completed_today: number;
  downloaded_today_gb: number;
  current_speed_mbps: number;
  timestamp?: string; // optional, if you added it
}
