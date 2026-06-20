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
  return request<{ jobs: { id: string; url: string; status: string }[] }>(
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

export async function deleteDownloads(
  status?: Download["status"] | "all",
): Promise<Download[]> {
  const query = status && status !== "all" ? `?status=${status}` : "";
  const data = await request<Download[] | { message: string }>(
    `/api/v1/downloads${query}`,
    { method: "DELETE" },
  );
  return Array.isArray(data) ? data : [];
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

export async function deleteDownload(id: string): Promise<void> {
  await request(`/api/v1/downloads/${id}`, { method: "DELETE" });
}

export async function retryDownload(id: string): Promise<void> {
  await request(`/api/v1/downloads/${id}/retry`, { method: "POST" });
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

export async function startAllDownloads(): Promise<void> {
  await request("/api/v1/downloads/start-all", { method: "POST" });
}

export async function pauseAllDownloads(): Promise<void> {
  await request("/api/v1/downloads/pause-all", { method: "POST" });
}

export async function fetchDashboardStats(): Promise<DashboardStats> {
  return request<DashboardStats>("/api/v1/downloads/stats");
}
