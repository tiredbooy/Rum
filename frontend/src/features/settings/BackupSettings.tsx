import { useId, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { ClipboardCopy, DatabaseBackup, Download, Upload } from "lucide-react";
import {
  exportDownloadList,
  getDiagnostics,
  importDownloadList,
} from "@/_lib/services/api/backup-api";
import { downloadKeys } from "@/_lib/services/queries/download.queries";
import { saveTextFile } from "@/_lib/wails";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { toast } from "@/lib/toast";
import { exportFileName, importSummary, parseDownloadList } from "./backup-file";

type Busy = "export" | "import" | "diagnostics" | null;

/** Back up / move the download list and copy a redacted support report. */
export function BackupSettings() {
  const queryClient = useQueryClient();
  const fileInput = useRef<HTMLInputElement>(null);
  const fileId = useId();
  const [busy, setBusy] = useState<Busy>(null);

  async function run(kind: Exclude<Busy, null>, job: () => Promise<void>) {
    setBusy(kind);
    try {
      await job();
    } finally {
      setBusy(null);
    }
  }

  const exportList = () =>
    run("export", async () => {
      try {
        const doc = await exportDownloadList();
        const saved = await saveTextFile(exportFileName(), JSON.stringify(doc, null, 2));
        if (saved) toast.success(`Exported ${doc.downloads.length} downloads`);
      } catch {
        toast.error("Export failed. Try again.");
      }
    });

  const importFile = (file: File) =>
    run("import", async () => {
      const doc = parseDownloadList(await file.text());
      if (!doc) {
        toast.error("Not a Rum download list.");
        return;
      }
      try {
        const result = await importDownloadList(doc);
        toast.success(importSummary(result));
        void queryClient.invalidateQueries({ queryKey: downloadKeys.all });
      } catch {
        toast.error("Import failed. Check the file and try again.");
      }
    });

  const copyDiagnostics = () =>
    run("diagnostics", async () => {
      try {
        const report = await getDiagnostics();
        await navigator.clipboard.writeText(JSON.stringify(report, null, 2));
        toast.success("Diagnostics copied");
      } catch {
        toast.error("Could not copy diagnostics.");
      }
    });

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-lg">
          <DatabaseBackup className="w-5 h-5" /> Backup
        </CardTitle>
      </CardHeader>
      <CardContent className="flex flex-wrap gap-2">
        <Button variant="outline" onClick={exportList} disabled={busy !== null}>
          <Download className="w-4 h-4" /> Export list
        </Button>
        <Button
          variant="outline"
          onClick={() => fileInput.current?.click()}
          disabled={busy !== null}
        >
          <Upload className="w-4 h-4" /> Import list
        </Button>
        <input
          ref={fileInput}
          id={fileId}
          type="file"
          accept="application/json,.json"
          aria-label="Download list file"
          className="sr-only"
          tabIndex={-1}
          onChange={(e) => {
            const file = e.target.files?.[0];
            e.target.value = "";
            if (file) void importFile(file);
          }}
        />
        <Button variant="outline" onClick={copyDiagnostics} disabled={busy !== null}>
          <ClipboardCopy className="w-4 h-4" /> Copy diagnostics
        </Button>
      </CardContent>
    </Card>
  );
}

export default BackupSettings;
