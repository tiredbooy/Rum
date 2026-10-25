import type { Download } from "@/_lib/types/download-types";
import { formatBytes } from "@/_lib/utils/format";

export const DISK_FULL = "disk_full";

/** Mirrors the engine's safety margin (max of 16 MiB or 1% of the size). */
const FLAT_MARGIN = 16 * 1024 * 1024;

type DiskFields = Pick<
  Download,
  "status" | "error_code" | "needed_bytes" | "free_bytes"
>;

export function isDiskFull(d: Pick<Download, "status" | "error_code">): boolean {
  return d.status === "error" && d.error_code === DISK_FULL;
}

/** Bytes the user must free before the download can continue (0 = unknown). */
export function bytesToFree(d: DiskFields): number {
  const needed = d.needed_bytes ?? 0;
  if (needed <= 0) return 0;
  const margin = Math.max(FLAT_MARGIN, Math.floor(needed / 100));
  return Math.max(0, needed + margin - (d.free_bytes ?? 0));
}

/** One short line saying what stopped the download and what to do. */
export function diskFullHeadline(d: DiskFields): string {
  const free = bytesToFree(d);
  return free > 0
    ? `Disk full — free ${formatBytes(free)} to continue`
    : "Disk full — free some space to continue";
}

export function countDiskFull(list: readonly DiskFields[]): number {
  return list.filter(isDiskFull).length;
}

/** Banner text for the downloads currently waiting for disk space. */
export function diskFullBannerText(count: number): string {
  return count === 1
    ? "1 download is waiting for disk space"
    : `${count} downloads are waiting for disk space`;
}
