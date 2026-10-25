import type { BatchOptions } from "@/_lib/types/download-types";

/** Options for POST /downloads/batch, or undefined when nothing is set. */
export function buildBatchOptions(opts: {
  destPath?: string;
  nightQueue?: boolean;
}): BatchOptions | undefined {
  const out: BatchOptions = {};
  const dest = opts.destPath?.trim();
  if (dest) out.dest_path = dest;
  if (opts.nightQueue) out.night_queue = true;
  return Object.keys(out).length ? out : undefined;
}
