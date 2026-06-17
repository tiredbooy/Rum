import type { ScheduleSettings } from "@/_lib/types/setting-types";
import { request } from "./api";

/** Fetch the current bandwidth schedule + scheduled-start toggle. */
export async function getSchedule(): Promise<ScheduleSettings> {
  return request<ScheduleSettings>(`/api/v1/settings/schedule`);
}

/** Replace the bandwidth schedule + scheduled-start toggle. */
export async function putSchedule(
  data: ScheduleSettings,
): Promise<ScheduleSettings> {
  return request<ScheduleSettings>(`/api/v1/settings/schedule`, {
    method: "PUT",
    body: JSON.stringify(data),
  });
}
