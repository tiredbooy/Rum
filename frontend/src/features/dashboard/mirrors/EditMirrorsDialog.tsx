import { useEffect, useState } from "react";
import type { Download } from "@/_lib/types/download-types";
import { useSetDownloadMirrors } from "@/_lib/services/queries/download.queries";
import { Button } from "@/components/ui/button";
import { ResponsiveDialog } from "@/features/reusable/dialog/DialogShell";
import { cleanMirrors, mirrorProblem } from "@/_lib/mirrors";
import { MirrorFields } from "./MirrorFields";

interface EditMirrorsDialogProps {
  download: Pick<Download, "id" | "filename" | "mirrors">;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/** Edit the extra links a download falls back to when its main link fails. */
export function EditMirrorsDialog({
  download,
  open,
  onOpenChange,
}: EditMirrorsDialogProps) {
  const [rows, setRows] = useState<string[]>(download.mirrors ?? []);
  const [failed, setFailed] = useState(false);
  const save = useSetDownloadMirrors();

  useEffect(() => {
    if (open) {
      setRows(download.mirrors ?? []);
      setFailed(false);
    }
  }, [open, download.mirrors]);

  const invalid = rows.some((row) => mirrorProblem(row) !== null);

  const submit = () => {
    setFailed(false);
    save.mutate(
      { id: download.id, mirrors: cleanMirrors(rows) },
      { onSuccess: () => onOpenChange(false), onError: () => setFailed(true) },
    );
  };

  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={onOpenChange}
      size="md"
      title="Mirrors"
      description={download.filename}
      footer={
        <Button disabled={invalid || save.isPending} onClick={submit}>
          Save
        </Button>
      }
    >
      <div className="space-y-2">
        <MirrorFields value={rows} onChange={setRows} idPrefix="edit-mirror" />
        {failed && (
          <p role="alert" className="text-xs text-destructive">
            Could not save mirrors. Check the links.
          </p>
        )}
      </div>
    </ResponsiveDialog>
  );
}

export default EditMirrorsDialog;
