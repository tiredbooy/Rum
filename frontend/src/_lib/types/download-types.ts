export type DownloadStatus =
  | "running"
  | "completed"
  | "error"
  | "paused"
  | "pending";

export type DownloadPriority = "low" | "normal" | "high";

export type ChecksumAlgo = "sha256" | "md5";

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
  /** Auto-organize category, echoed by the backend when set. */
  category?: string;
}

/**
 * Result of POST /downloads/:id/verify (and the post-repair verdict returned by
 * POST /downloads/:id/repair). Mirrors download.VerifyResult (Go).
 * - ok: the file is intact.
 * - bad_segments: indices of bad/missing segments (empty when ok).
 * - method: how the verdict was reached ("stored_hash" | "structural" | "deep"
 *   | "missing").
 * - detail: optional human-readable explanation.
 */
export interface VerifyResult {
  ok: boolean;
  bad_segments: number[];
  method: string;
  detail?: string;
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
  // Round-3 additive optional fields (POST /downloads).
  /** Expected checksum to verify the finished file against. */
  checksum?: string;
  /** Algorithm for `checksum`. */
  checksum_algo?: ChecksumAlgo;
  /** RFC3339 timestamp to defer the download start to. */
  start_at?: string;
  /** Category override for auto-organize. */
  category?: string;
}

/** Options accepted by the batch endpoint (POST /downloads/batch). */
export interface BatchOptions {
  dest_path?: string;
  speed_limit?: number;
  max_retries?: number;
  group_folder?: string;
  category?: string;
  checksum?: string;
  checksum_algo?: ChecksumAlgo;
  start_at?: string;
}

export interface BatchRequest {
  urls: string[];
  options?: BatchOptions;
}

export interface DownloadResponse {
  id: string;
  url: string;
  status: string;
  filename?: string;
}

export interface BatchError {
  url: string;
  error: string;
}

export interface BatchResult {
  created: DownloadResponse[];
  errors: BatchError[];
}

export interface DashboardStats {
  active_downloads: number;
  completed_today: number;
  downloaded_today_gb: number;
  current_speed_mbps: number;
  timestamp?: string; // optional, if you added it
}
