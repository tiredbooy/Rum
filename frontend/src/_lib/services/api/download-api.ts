import {
  BatchOptions,
  BatchResult,
  DashboardStats,
  Download,
  DownloadPriority,
  DownloadReq,
  VerifyResult,
} from "@/_lib/types/download-types";
import { request } from "./api";

export async function getDownloads(
  status?: Download["status"] | "all",
): Promise<Download[]> {
  const query = status && status !== "all" ? `?status=${status}` : "";
  const data = await request<Download[] | { message: string }>(
    `/api/v1/downloads${query}`,
  );
  return Array.isArray(data) ? data : [];
}

export async function fetchDownloadProgress(id: string): Promise<Download> {
  return request<Download>(`/api/v1/downloads/${id}`);
}

export async function createDownloads(payload: DownloadReq) {
  return request<{
    jobs: {
      id: string;
      url: string;
      status: string;
      error_code?: string;
      needed_bytes?: number;
      free_bytes?: number;
    }[];
  }>(
    "/api/v1/downloads",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
  );
}

/**
 * Batch-create downloads. Returns the created jobs plus a per-URL error list for
 * any URLs the backend rejected (e.g. bad scheme).
 */
export async function createBatch(
  urls: string[],
  options?: BatchOptions,
): Promise<BatchResult> {
  return request<BatchResult>("/api/v1/downloads/batch", {
    method: "POST",
    body: JSON.stringify({ urls, options }),
  });
}

/** Soft delete result: ids now hidden and the window to undo them. */
export interface DeleteResult {
  ids: string[];
  pending_delete_at: string;
  undo_seconds: number;
}

export interface DeleteOptions {
  /** Also delete the file / partial. Default keeps data ("remove from list"). */
  deleteFile?: boolean;
}

function deleteQuery(params: Record<string, string | undefined>, opts?: DeleteOptions) {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) if (v) q.set(k, v);
  if (opts?.deleteFile) q.set("delete_file", "true");
  const s = q.toString();
  return s ? `?${s}` : "";
}

export async function deleteDownloads(
  status?: Download["status"] | "all",
  opts?: DeleteOptions,
): Promise<DeleteResult> {
  const query = deleteQuery({ status: status && status !== "all" ? status : undefined }, opts);
  return request<DeleteResult>(`/api/v1/downloads${query}`, { method: "DELETE" });
}

/** Undo a delete while its window is open. Rejects once it has expired. */
export async function restoreDownloads(ids: string[]): Promise<{ restored: number }> {
  return request<{ restored: number }>("/api/v1/downloads/restore", {
    method: "POST",
    body: JSON.stringify({ ids }),
  });
}

export async function startDownload(id: string): Promise<void> {
  await request(`/api/v1/downloads/${id}/start`, { method: "POST" });
}

export async function pauseDownload(id: string): Promise<void> {
  await request(`/api/v1/downloads/${id}/pause`, { method: "PUT" });
}

export async function resumeDownload(id: string): Promise<void> {
  await request(`/api/v1/downloads/${id}/resume`, { method: "PUT" });
}

export async function deleteDownload(
  id: string,
  opts?: DeleteOptions,
): Promise<DeleteResult> {
  return request<DeleteResult>(`/api/v1/downloads/${id}${deleteQuery({}, opts)}`, {
    method: "DELETE",
  });
}

export async function retryDownload(id: string): Promise<void> {
  await request(`/api/v1/downloads/${id}/retry`, { method: "POST" });
}

/** Restart a download that stopped on a block page, accepting HTML once. */
export async function downloadAnyway(id: string): Promise<void> {
  await request(`/api/v1/downloads/${id}/download-anyway`, { method: "POST" });
}

/**
 * Verify a (finished) download's integrity. `deep` forces a network
 * re-validation (re-HEAD to confirm the source still matches). Never mutates the
 * file, so it is always safe to call. Returns the engine VerifyResult.
 */
export async function verifyDownload(
  id: string,
  deep?: boolean,
): Promise<VerifyResult> {
  const query = deep ? "?deep=true" : "";
  return request<VerifyResult>(`/api/v1/downloads/${id}/verify${query}`, {
    method: "POST",
  });
}

/**
 * Repair a download by re-fetching its bad/missing segments, then re-verifying.
 * Pass explicit `segments` to repair only those; omit them to let the backend
 * verify first and repair whatever it finds bad. Idempotent and safe on an
 * already-good completed download. Returns the post-repair VerifyResult.
 */
export async function repairDownload(
  id: string,
  segments?: number[],
): Promise<VerifyResult> {
  return request<VerifyResult>(`/api/v1/downloads/${id}/repair`, {
    method: "POST",
    ...(segments ? { body: JSON.stringify({ segments }) } : {}),
  });
}

export async function setDownloadPriority(
  id: string,
  priority: DownloadPriority,
): Promise<void> {
  await request(`/api/v1/downloads/${id}/priority`, {
    method: "PATCH",
    body: JSON.stringify({ priority }),
  });
}

export interface RefreshUrlResult {
  id: string;
  /** Partial data was kept and the download continues from it. */
  resumed: boolean;
  /** Partial data was discarded and the download starts from zero. */
  restarted: boolean;
}

/**
 * Give a failed download a fresh link. Rejects with an ApiError coded
 * "remote_changed" when the new link is a different file; call again with
 * `restart: true` once the user agrees to start over.
 */
export async function refreshDownloadUrl(
  id: string,
  url: string,
  restart = false,
): Promise<RefreshUrlResult> {
  return request<RefreshUrlResult>(`/api/v1/downloads/${id}/url`, {
    method: "PATCH",
    body: JSON.stringify({ url: url.trim(), restart }),
  });
}

/** Replace a download's mirror list (empty clears it). Returns the stored list. */
export async function setDownloadMirrors(
  id: string,
  mirrors: string[],
): Promise<{ id: string; mirrors: string[] }> {
  return request<{ id: string; mirrors: string[] }>(
    `/api/v1/downloads/${id}/mirrors`,
    { method: "PUT", body: JSON.stringify({ mirrors }) },
  );
}

export async function startAllDownloads(): Promise<void> {
  await request("/api/v1/downloads/start-all", { method: "POST" });
}

export interface RetryFailedResult {
  retried: number;
  skipped_disk_full: number;
  skipped_blocked: number;
  skipped_expired: number;
}

/** Restart every failed download that is worth retrying now. */
export async function retryFailedDownloads(): Promise<RetryFailedResult> {
  return request<RetryFailedResult>("/api/v1/downloads/retry-failed", {
    method: "POST",
  });
}

export async function pauseAllDownloads(): Promise<void> {
  await request("/api/v1/downloads/pause-all", { method: "POST" });
}

export async function fetchDashboardStats(): Promise<DashboardStats> {
  return request<DashboardStats>("/api/v1/downloads/stats");
}
