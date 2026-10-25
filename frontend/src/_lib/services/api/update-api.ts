import type { UpdateStatus } from "@/_lib/update-notice";
import { request } from "./api";

export function getUpdateStatus(): Promise<UpdateStatus> {
  return request<UpdateStatus>("/api/v1/update");
}

/** `auto` = the startup check (obeys the setting and once-a-day throttle). */
export function checkForUpdate(auto: boolean): Promise<UpdateStatus> {
  return request<UpdateStatus>("/api/v1/update/check", {
    method: "POST",
    body: JSON.stringify({ auto }),
  });
}

export function dismissUpdate(tag: string): Promise<UpdateStatus> {
  return request<UpdateStatus>("/api/v1/update/dismiss", {
    method: "POST",
    body: JSON.stringify({ tag }),
  });
}

export interface UpdateDownloadResult {
  path: string;
  verified: boolean;
}

/** Saves the installer for this system and verifies its SHA-256. */
export function downloadUpdate(): Promise<UpdateDownloadResult> {
  return request<UpdateDownloadResult>("/api/v1/update/download", { method: "POST" });
}
