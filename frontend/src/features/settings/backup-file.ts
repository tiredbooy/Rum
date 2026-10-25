import type { DownloadListFile, ImportResult } from "@/_lib/services/api/backup-api";

/** Parse an exported list; null when the text is not a Rum download list. */
export function parseDownloadList(text: string): DownloadListFile | null {
  try {
    const doc = JSON.parse(text) as Partial<DownloadListFile> | null;
    if (!doc || doc.version !== 1 || !Array.isArray(doc.downloads)) return null;
    return doc as DownloadListFile;
  } catch {
    return null;
  }
}

/** One short line for the toast after an import. */
export function importSummary(r: ImportResult): string {
  if (r.added === 0 && r.skipped === 0 && r.invalid === 0) return "Nothing to import";
  const parts = [`Added ${r.added} (paused)`];
  if (r.skipped > 0) parts.push(`${r.skipped} skipped`);
  if (r.invalid > 0) parts.push(`${r.invalid} invalid`);
  return parts.join(", ");
}

/** Dated file name, e.g. rum-downloads-2026-01-31.json. */
export function exportFileName(now: Date = new Date()): string {
  return `rum-downloads-${now.toISOString().slice(0, 10)}.json`;
}
