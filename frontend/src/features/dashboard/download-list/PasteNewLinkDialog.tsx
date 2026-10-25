import { useState } from "react";
import type { Download } from "@/_lib/types/download-types";
import { useRefreshDownloadUrl } from "@/_lib/services/queries/download.queries";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ResponsiveDialog } from "@/features/reusable/dialog/DialogShell";
import { linkProblem, refreshFailure } from "./link-refresh";

interface PasteNewLinkDialogProps {
  download: Pick<Download, "id" | "filename">;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/**
 * Replaces an expired link. The server keeps the partial file when the new link
 * serves the same bytes; if it is a different file we ask before starting over.
 */
export function PasteNewLinkDialog({
  download,
  open,
  onOpenChange,
}: PasteNewLinkDialogProps) {
  const [link, setLink] = useState("");
  const [problem, setProblem] = useState<string | null>(null);
  const [confirmRestart, setConfirmRestart] = useState(false);
  const refresh = useRefreshDownloadUrl();

  const close = (next: boolean) => {
    if (!next) {
      setLink("");
      setProblem(null);
      setConfirmRestart(false);
    }
    onOpenChange(next);
  };

  const submit = (restart: boolean) => {
    const invalid = linkProblem(link);
    if (invalid) {
      setProblem(invalid);
      return;
    }
    setProblem(null);
    refresh.mutate(
      { id: download.id, url: link, restart },
      {
        onSuccess: () => close(false),
        onError: (err) => {
          const failure = refreshFailure(err);
          if (failure.kind === "confirm-restart") setConfirmRestart(true);
          else setProblem(failure.message);
        },
      },
    );
  };

  const footer = confirmRestart ? (
    <>
      <Button variant="outline" onClick={() => setConfirmRestart(false)}>
        Cancel
      </Button>
      <Button
        variant="destructive"
        disabled={refresh.isPending}
        onClick={() => submit(true)}
      >
        Restart
      </Button>
    </>
  ) : (
    <Button type="submit" form="paste-new-link-form" disabled={refresh.isPending}>
      Resume
    </Button>
  );

  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={close}
      size="md"
      title={confirmRestart ? "Different file" : "Paste new link"}
      description={
        confirmRestart
          ? "Partial data won't match. Restart from zero?"
          : download.filename
      }
      footer={footer}
    >
      {!confirmRestart && (
        <form
          id="paste-new-link-form"
          className="space-y-2"
          onSubmit={(e) => {
            e.preventDefault();
            submit(false);
          }}
        >
          <Label htmlFor="paste-new-link-input">New link</Label>
          <Input
            id="paste-new-link-input"
            type="url"
            autoFocus
            autoComplete="off"
            placeholder="https://"
            value={link}
            aria-invalid={problem ? true : undefined}
            aria-describedby={problem ? "paste-new-link-error" : undefined}
            onChange={(e) => setLink(e.target.value)}
          />
          {problem && (
            <p
              id="paste-new-link-error"
              role="alert"
              className="text-xs text-destructive"
            >
              {problem}
            </p>
          )}
        </form>
      )}
    </ResponsiveDialog>
  );
}
