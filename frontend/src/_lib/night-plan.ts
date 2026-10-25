import type { NightPlan } from "@/_lib/types/setting-types";
import type { Download } from "@/_lib/types/download-types";

/** Upper bound for the data cap; mirrors the backend (maxNightCapGB). */
export const MAX_NIGHT_CAP_GB = 100000;

export const DEFAULT_NIGHT_PLAN: NightPlan = {
  enabled: false,
  start: "02:00",
  end: "07:00",
  start_at_open: true,
  pause_at_close: true,
  cap_gb: 0,
};

const CLOCK = /^([01]\d|2[0-3]):([0-5]\d)$/;

export function isClock(value: string): boolean {
  return CLOCK.test(value);
}

/**
 * Field -> message for every invalid field (empty when valid). Mirrors
 * config.NightPlan.Problems on the backend; messages stay short.
 */
export function validateNightPlan(plan: NightPlan): Record<string, string> {
  const out: Record<string, string> = {};
  if (!isClock(plan.start)) out.start = "Use a time like 02:00.";
  if (!isClock(plan.end)) out.end = "Use a time like 07:00.";
  if (isClock(plan.start) && plan.start === plan.end) {
    out.window = "Start and end must differ.";
  }
  if (
    !Number.isFinite(plan.cap_gb) ||
    plan.cap_gb < 0 ||
    plan.cap_gb > MAX_NIGHT_CAP_GB
  ) {
    out.cap_gb = "Use 0 (no cap) or more.";
  }
  return out;
}

/** True when the window crosses midnight (end earlier than start). */
export function crossesMidnight(plan: Pick<NightPlan, "start" | "end">) {
  return isClock(plan.start) && isClock(plan.end) && plan.end < plan.start;
}

/** Time choices in 15-minute steps, plus `current` when it is off-grid. */
export function clockOptions(current?: string): string[] {
  const out: string[] = [];
  for (let m = 0; m < 24 * 60; m += 15) {
    out.push(
      `${String(Math.floor(m / 60)).padStart(2, "0")}:${String(m % 60).padStart(2, "0")}`,
    );
  }
  if (current && isClock(current) && !out.includes(current)) {
    out.push(current);
    out.sort();
  }
  return out;
}

export interface NightChip {
  label: string;
  tone: "waiting" | "active" | "capped";
}

/**
 * Status chip for a night job, or null when none applies. "Starts 02:00" while
 * the window is closed, "Night" while it is open, "Night limit reached" when the
 * window's data cap is spent.
 */
export function nightChip(
  job: Pick<Download, "night_queue" | "status"> & { error_code?: string },
  plan: { enabled: boolean; start: string; active: boolean; cap_reached: boolean } | undefined,
): NightChip | null {
  if (!job.night_queue || job.status === "completed" || !plan?.enabled) {
    return null;
  }
  if (job.error_code === "night_limit" || (plan.active && plan.cap_reached)) {
    return { label: "Night limit reached", tone: "capped" };
  }
  if (plan.active) return { label: "Night", tone: "active" };
  return { label: `Starts ${plan.start}`, tone: "waiting" };
}
