export function formatBytes(bytes: number): string {
  if (bytes <= 0) return "0 B";
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${sizes[i]}`;
}

export function formatSpeed(bytesPerSec: number): string {
  return `${formatBytes(bytesPerSec)}/s`;
}

export function formatETA(seconds: number): string {
  if (!seconds || seconds <= 0 || !isFinite(seconds)) return "—";

  const hrs = Math.floor(seconds / 3600);
  const mins = Math.floor((seconds % 3600) / 60);
  const secs = Math.floor(seconds % 60);

  const parts: string[] = [];
  if (hrs > 0) parts.push(`${hrs}h`);
  if (mins > 0) parts.push(`${mins}m`);
  if (secs > 0 || parts.length === 0) parts.push(`${secs}s`);

  return parts.join(" ");
}

export function formatDataSize(gb: number): { value: string; unit: string } {
  const bytes = gb * 1024 * 1024 * 1024; // convert GB to bytes

  if (bytes === 0) return { value: "0", unit: "B" };

  const units = ["B", "KB", "MB", "GB", "TB"];
  let size = bytes;
  let unitIndex = 0;
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex++;
  }

  // Format with up to 2 decimal places, remove trailing .00
  const formatted = size % 1 === 0 ? size.toString() : size.toFixed(2);
  return { value: formatted, unit: units[unitIndex] };
}
