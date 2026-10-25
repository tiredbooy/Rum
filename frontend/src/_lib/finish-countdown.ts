import type { FinishPending } from "@/_lib/types/setting-types";

/** Whole seconds until the deadline, rounded up, never negative. */
export function countdownSeconds(deadlineMs: number, nowMs: number): number {
  return Math.max(0, Math.ceil((deadlineMs - nowMs) / 1000));
}

export function finishTitle(action: FinishPending["action"], secs: number): string {
  return `${action === "sleep" ? "Sleeping" : "Shutting down"} in ${secs}s`;
}
