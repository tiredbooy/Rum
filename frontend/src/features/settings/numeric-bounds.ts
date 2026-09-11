/**
 * Bounds for the numeric settings, shared by the Downloads and Integrity cards
 * so the two never disagree about what a valid value is. They mirror the
 * server-side limits in handlers.validateSettingReq / config.Setting.Validate —
 * the client clamps for convenience, the server is still the authority.
 */
export type NumField =
  | "speed_limit_kb"
  | "max_parallel"
  | "connections"
  | "max_retries"
  | "retry_backoff_sec";

export const NUM_BOUNDS: Record<
  NumField,
  { min: number; max: number; fallback: number; message: string }
> = {
  speed_limit_kb: {
    min: 0,
    max: 1_000_000,
    fallback: 0,
    message: "Must be 0 or more.",
  },
  max_parallel: { min: 1, max: 64, fallback: 3, message: "Must be 1 to 64." },
  connections: { min: 1, max: 16, fallback: 8, message: "Must be 1 to 16." },
  max_retries: { min: 0, max: 10, fallback: 3, message: "Must be 0 to 10." },
  retry_backoff_sec: {
    min: 1,
    max: 60,
    fallback: 1,
    message: "Must be 1 to 60 seconds.",
  },
};

/** Clamp a typed numeric string into the field's range. */
export function clampNum(raw: string, field: NumField): number {
  const { min, max, fallback } = NUM_BOUNDS[field];
  const n = raw.trim() === "" ? fallback : Math.round(Number(raw));
  if (Number.isNaN(n)) return fallback;
  return Math.min(max, Math.max(min, n));
}

/**
 * The inline message to show while a typed value is out of range, or undefined
 * when it is fine. Blank is fine — it commits to the field's default on blur.
 */
export function rangeError(raw: string, field: NumField): string | undefined {
  if (raw.trim() === "") return undefined;
  const { min, max, message } = NUM_BOUNDS[field];
  const n = Number(raw);
  if (Number.isNaN(n) || n < min || n > max) return message;
  return undefined;
}
