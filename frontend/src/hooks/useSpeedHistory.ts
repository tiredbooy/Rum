import { useEffect, useRef, useState } from "react";

export const DEFAULT_MAX_SAMPLES = 30;

/**
 * Append `value` to a rolling ring buffer of at most `max` samples (oldest
 * dropped first). Returns a new array. Pure + side-effect free so the
 * windowing logic can be unit tested without React.
 */
export function pushSample(
  prev: readonly number[],
  value: number,
  max: number = DEFAULT_MAX_SAMPLES,
): number[] {
  const next = prev.length >= max ? prev.slice(prev.length - max + 1) : prev.slice();
  next.push(value);
  return next;
}

/**
 * Keeps a rolling window of recent speed samples (bytes/sec) for a single
 * download so a tiny sparkline can render its live throughput. Samples are
 * appended whenever the incoming `speed` value changes and capped to the last
 * `maxSamples` (default 30) to stay cheap.
 *
 * Feed this from the already-merged live Download (per-job progress cache),
 * e.g. `useSpeedHistory(merged.speed, merged.status === "running")`.
 */
export function useSpeedHistory(
  speed: number | undefined,
  active: boolean,
  maxSamples: number = DEFAULT_MAX_SAMPLES,
): number[] {
  const [samples, setSamples] = useState<number[]>([]);
  const lastRef = useRef<number | null>(null);

  useEffect(() => {
    if (!active) return;
    const value = typeof speed === "number" && speed >= 0 ? speed : 0;
    // Avoid pushing duplicate frames when the stream re-emits the same speed.
    if (lastRef.current === value && samples.length > 0) return;
    lastRef.current = value;
    setSamples((prev) => pushSample(prev, value, maxSamples));
    // We intentionally depend only on `speed`/`active`; `samples` is read via
    // the functional updater to keep the window rolling without re-subscribing.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [speed, active, maxSamples]);

  // Reset history when the download stops being active (e.g. paused/completed)
  // so a later resume starts a fresh chart instead of a stale flat line.
  useEffect(() => {
    if (!active) {
      lastRef.current = null;
      setSamples([]);
    }
  }, [active]);

  return samples;
}
