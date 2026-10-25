import type { Download } from "@/_lib/types/download-types";
import { openActionsFor, openDownload, showInFolder, type OpenResult } from "@/_lib/open-download";
import { Button } from "@/components/ui/button";
import { toast } from "@/lib/toast";
import { ExternalLink, FolderOpen } from "lucide-react";

interface OpenActionsProps {
  download: Pick<Download, "id" | "status" | "error_code">;
  /** Show the "Open" (default app) button. Off for disk-full cards. */
  withOpen?: boolean;
}

function report(result: OpenResult) {
  if (result.status === "failed") toast.error("Can't open it. Check the file still exists.");
}

/**
 * "Open" and "Open folder" buttons. Renders nothing outside the desktop shell
 * (no bindings), so browser mode shows no dead buttons.
 */
export function OpenActions({ download, withOpen = true }: OpenActionsProps) {
  const { folder, open } = openActionsFor(download);
  const showOpen = withOpen && open;
  if (!folder && !showOpen) return null;

  return (
    <div className="flex items-center gap-1.5">
      {showOpen && (
        <Button
          variant="outline"
          size="sm"
          className="h-7 gap-1"
          aria-label="Open"
          onClick={() => void openDownload(download.id).then(report)}
        >
          <ExternalLink className="size-3.5" aria-hidden="true" />
          Open
        </Button>
      )}
      {folder && (
        <Button
          variant="outline"
          size="sm"
          className="h-7 gap-1"
          aria-label="Open folder"
          onClick={() => void showInFolder(download).then(report)}
        >
          <FolderOpen className="size-3.5" aria-hidden="true" />
          Open folder
        </Button>
      )}
    </div>
  );
}

export default OpenActions;
