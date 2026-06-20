import { useMemo } from "react";
import { useDownloads } from "@/_lib/services/queries/download.queries";
import type { Download } from "@/_lib/types/download-types";

export interface AggregateEta {
  /** Seconds until all running downloads finish at the combined current speed, or null when not computable. */
  etaSeconds: number | null;
  /** Combined bytes/sec across running downloads. */
  totalSpeed: number;
  /** Combined remaining bytes across running downloads that have a known size. */
  remainingBytes: number;
  /** Number of running downloads. */
  activeCount: number;
  /** True when a running download has an unknown total size (so the ETA is a lower bound). */
  hasUnknownSize: boolean;
}

/** The subset of a download the aggregate needs — keeps this pure and testable. */
type DownloadLike = Pick<Download, "status" | "speed" | "total_size" | "downloaded">;

/**
 * Pure aggregate finish-time estimate: combined remaining bytes ÷ combined current
 * speed across all RUNNING downloads. etaSeconds is null when nothing is running or
 * the combined speed is zero (stalled) so callers can render a "calculating" state
 * instead of Infinity. Extracted from the hook so it can be unit-tested directly.
 */
export function computeAggregateEta(downloads: DownloadLike[]): AggregateEta {
  let totalSpeed = 0;
  let remainingBytes = 0;
  let activeCount = 0;
  let hasUnknownSize = false;

  for (const d of downloads) {
    if (d.status !== "running") continue;
    activeCount += 1;
    totalSpeed += Math.max(0, d.speed ?? 0);
    const total = d.total_size ?? 0;
    const done = d.downloaded ?? 0;
    if (total > 0) {
      remainingBytes += Math.max(0, total - done);
    } else {
      hasUnknownSize = true;
    }
  }

  const etaSeconds =
    totalSpeed > 0 && remainingBytes > 0 ? remainingBytes / totalSpeed : null;

  return { etaSeconds, totalSpeed, remainingBytes, activeCount, hasUnknownSize };
}

/**
 * Live combined finish-time across all running downloads. Driven by the
 * SSE-patched downloads cache (`useDownloads("all")`), so it updates in real time
 * as speeds change — meaningful now that per-download speed is sampled over a real
 * window instead of spiking.
 */
export function useAggregateEta(): AggregateEta {
  const { data } = useDownloads("all");
  return useMemo(() => computeAggregateEta(data ?? []), [data]);
}
