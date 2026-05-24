import { DashboardStats, Download, DownloadReq } from "@/_lib/types/download-types";
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

export async function startAllDownloads(): Promise<void> {
  await request("/api/v1/downloads/start-all", { method: "POST" });
}

export async function pauseAllDownloads(): Promise<void> {
  await request("/api/v1/downloads/pause-all", { method: "POST" });
}

export async function fetchDashboardStats(): Promise<DashboardStats> {
  return request<DashboardStats>("/api/v1/downloads/stats");
}
