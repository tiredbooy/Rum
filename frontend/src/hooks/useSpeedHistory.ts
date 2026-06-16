import { useEffect, useRef, useState } from "react";

const MAX_SAMPLES = 30;

/**
 * Keeps a rolling window of recent speed samples (bytes/sec) for a single
 * download so a tiny sparkline can render its live throughput. Samples are
 * appended whenever the incoming `speed` value changes and capped to the last
 * `MAX_SAMPLES` (default 30) to stay cheap.
 *
 * Feed this from the already-merged live Download (per-job progress cache),
 * e.g. `useSpeedHistory(merged.speed, merged.status === "running")`.
 */
export function useSpeedHistory(
  speed: number | undefined,
  active: boolean,
  maxSamples: number = MAX_SAMPLES,
): number[] {
  const [samples, setSamples] = useState<number[]>([]);
  const lastRef = useRef<number | null>(null);

  useEffect(() => {
    if (!active) return;
    const value = typeof speed === "number" && speed >= 0 ? speed : 0;
    // Avoid pushing duplicate frames when the stream re-emits the same speed.
    if (lastRef.current === value && samples.length > 0) return;
    lastRef.current = value;
    setSamples((prev) => {
      const next = [...prev, value];
      return next.length > maxSamples ? next.slice(-maxSamples) : next;
    });
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
