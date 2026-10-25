import type { NightPlan, NightPlanState } from "@/_lib/types/setting-types";
import { request } from "./api";

/** Fetch the night plan plus whether its window is open right now. */
export async function getNightPlan(): Promise<NightPlanState> {
  return request<NightPlanState>(`/api/v1/settings/night-plan`);
}

/** Replace the night plan. Rejects with field errors when invalid. */
export async function putNightPlan(data: NightPlan): Promise<NightPlanState> {
  return request<NightPlanState>(`/api/v1/settings/night-plan`, {
    method: "PUT",
    body: JSON.stringify(data),
  });
}

/** Move downloads into (enabled) or out of the night queue. */
export async function moveToNightQueue(
  ids: string[],
  enabled: boolean,
): Promise<{ updated: number }> {
  return request<{ updated: number }>(`/api/v1/downloads/night-queue`, {
    method: "POST",
    body: JSON.stringify({ ids, enabled }),
  });
}
