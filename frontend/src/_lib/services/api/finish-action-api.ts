import type { FinishPending } from "@/_lib/types/setting-types";
import { request } from "./api";

/** The running sleep/shutdown countdown, or null. */
export async function getFinishAction(): Promise<FinishPending | null> {
  const res = await request<{ pending: FinishPending | null }>(`/api/v1/finish-action`);
  return res.pending ?? null;
}

export async function cancelFinishAction(): Promise<void> {
  await request<{ cancelled: boolean }>(`/api/v1/finish-action/cancel`, {
    method: "POST",
  });
}
