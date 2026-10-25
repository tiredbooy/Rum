import type { Download } from "@/_lib/types/download-types";

/**
 * Normalize a raw SSE progress payload (snake_case, possibly bogus values)
 * into a partial Download patch. Progress is clamped to [0, 100] so the UI is
 * robust regardless of what the backend sends (it can emit negative or >100
 * values when the total size is unknown / -1).
 */
export function patchFromRaw(raw: any): (Partial<Download> & { id: string }) | null {
  const id: string | undefined = raw?.job_id ?? raw?.id;
  if (!id) return null;

  const patch: Partial<Download> & { id: string } = { id };

  if (raw.downloaded != null) patch.downloaded = raw.downloaded;
  if (raw.total_size != null) patch.total_size = raw.total_size;
  if (raw.speed != null) patch.speed = raw.speed;
  if (raw.status != null) {
    patch.status = raw.status;
    // A job that left the error state must not keep showing its old failure.
    if (raw.status !== "error") {
      patch.error = undefined;
      patch.error_code = undefined;
      patch.retryable = undefined;
      patch.needed_bytes = undefined;
      patch.free_bytes = undefined;
      patch.disk_path = undefined;
    }
  }
  if (raw.error != null) patch.error = raw.error;
  if (raw.error_code != null) patch.error_code = raw.error_code;
  if (raw.retryable != null) patch.retryable = raw.retryable;
  if (raw.needed_bytes != null) patch.needed_bytes = raw.needed_bytes;
  if (raw.free_bytes != null) patch.free_bytes = raw.free_bytes;
  if (raw.disk_path != null) patch.disk_path = raw.disk_path;
  if (raw.eta != null) patch.eta = raw.eta;
  if (raw.progress != null) {
    patch.progress = Math.min(100, Math.max(0, raw.progress));
  }

  return patch;
}
