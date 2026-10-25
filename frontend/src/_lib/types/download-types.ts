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
  /** Machine reason for `error` (network_down, dns, disk_full, ...). */
  error_code?: string;
  /** True when retrying later may succeed. */
  retryable?: boolean;
  /** disk_full: bytes still to download when the disk filled up. */
  needed_bytes?: number;
  /** disk_full: free bytes on the volume at that moment. */
  free_bytes?: number;
  /** disk_full: folder on the affected volume. */
  disk_path?: string;
  /** Host that answered the first probe, after redirects. */
  final_host?: string;
  priority?: DownloadPriority;
  dest_path?: string;
  created_at?: string;
  completed_at?: string;
  /** Auto-organize category, echoed by the backend when set. */
  category?: string;
  /** Assigned to the night plan: runs only inside the night window. */
  night_queue?: boolean;
  /** The night plan itself paused it; it restarts when the window opens. */
  night_paused?: boolean;
  /** Result of comparing the file with a checksum; absent when none was found. */
  checksum_status?: "verified" | "mismatch";
  checksum_result_algo?: ChecksumAlgo;
  checksum_source?: "user" | "header" | "sidecar";
  /** Extra links for the same file, tried if the main link fails. */
  mirrors?: string[];
  /** Host serving the file right now when it is a mirror; unset = main link. */
  active_mirror?: string;
}

/** Result of POST /downloads/:id/verify-checksum. */
export interface ChecksumOutcome {
  status: "verified" | "mismatch" | "unavailable";
  algo?: ChecksumAlgo;
  source?: string;
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
  /** Run only inside the night window ("Download at night"). */
  night_queue?: boolean;
  /** Extra links for the same file (single URL only, at most 5). */
  mirrors?: string[];
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
  night_queue?: boolean;
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
  error_code?: string;
  needed_bytes?: number;
  free_bytes?: number;
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
