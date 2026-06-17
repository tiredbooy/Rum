import { SettingReq } from "@/_lib/types/setting-types";
import { request } from "./api";

export async function getSettings(): Promise<SettingReq> {
  return request<SettingReq>(`/api/v1/settings`);;
}

export async function updateSettings(
  data: Partial<SettingReq>,
): Promise<SettingReq> {
  return request<SettingReq>(`/api/v1/settings`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });
}
