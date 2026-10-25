import { request } from "./api";

/** The exported download list (see backend ExportDoc). */
export interface DownloadListFile {
  version: number;
  exported_at?: string;
  downloads: Record<string, unknown>[];
}

export interface ImportResult {
  added: number;
  skipped: number;
  invalid: number;
}

export function exportDownloadList(): Promise<DownloadListFile> {
  return request<DownloadListFile>("/api/v1/downloads/export");
}

export function importDownloadList(doc: DownloadListFile): Promise<ImportResult> {
  return request<ImportResult>("/api/v1/downloads/import", {
    method: "POST",
    body: JSON.stringify(doc),
  });
}

/** Redacted support report; the caller copies it as text. */
export function getDiagnostics(): Promise<Record<string, unknown>> {
  return request<Record<string, unknown>>("/api/v1/diagnostics");
}
