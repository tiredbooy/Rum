import { useEffect, useMemo, useState } from "react";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  CheckCircle2,
  Clipboard,
  Loader2,
  Send,
  XCircle,
} from "lucide-react";
import { isValidUrl } from "@/_lib/utils/validators";
import { useClipboardUrl } from "@/hooks/useClipBoardUrl";
import { useCreateBatch } from "@/_lib/services/queries/download.queries";
import type { BatchResult } from "@/_lib/types/download-types";
import { useAddDialogStore } from "@/stores/add-dialog-store";
import { classifyLinks } from "@/features/drag-drop/extract-links";

/**
 * Self-contained batch add: paste/type one URL per line, submit straight to
 * POST /downloads/batch, and show a per-URL created/failed summary. Distinct
 * from the draft-based Save flow in the dialog footer.
 */
export function BatchDownloadForm() {
  const { readRawText } = useClipboardUrl();
  const createBatch = useCreateBatch();

  const consumeDropped = useAddDialogStore((s) => s.consumeDropped);
  const open = useAddDialogStore((s) => s.open);

  const [rawText, setRawText] = useState("");
  const [destPath, setDestPath] = useState("");
  const [result, setResult] = useState<BatchResult | null>(null);
  const [showAll, setShowAll] = useState(false);
  const [dropped, setDropped] = useState<{
    all: string[];
    downloadable: string[];
  } | null>(null);

  // When the dialog opens carrying dropped URLs, classify them once and seed the
  // textarea (downloadable-only by default, falling back to all links).
  useEffect(() => {
    if (!open) return;
    const urls = consumeDropped();
    if (urls && urls.length) {
      const classified = classifyLinks(urls);
      setDropped(classified);
      setRawText(
        (classified.downloadable.length
          ? classified.downloadable
          : classified.all
        ).join("\n"),
      );
      setShowAll(classified.downloadable.length === 0);
    }
  }, [open, consumeDropped]);

  // When the toggle flips and we have a drop set, swap the textarea contents.
  useEffect(() => {
    if (!dropped) return;
    setRawText((showAll ? dropped.all : dropped.downloadable).join("\n"));
  }, [showAll, dropped]);

  const urls = useMemo(
    () =>
      rawText
        .split("\n")
        .map((l) => l.trim())
        .filter((l) => isValidUrl(l)),
    [rawText],
  );

  const totalLines = rawText
    .split("\n")
    .map((l) => l.trim())
    .filter(Boolean).length;
  const invalidCount = totalLines - urls.length;

  const handlePaste = async () => {
    const text = await readRawText();
    if (text) setRawText((prev) => (prev ? `${prev}\n${text}` : text));
  };

  const handleSubmit = () => {
    if (urls.length === 0) return;
    createBatch.mutate(
      {
        urls,
        options: destPath.trim() ? { dest_path: destPath.trim() } : undefined,
      },
      {
        onSuccess: (res) => {
          setResult(res);
          // Keep only the lines that failed so the user can fix + resubmit.
          if (res.errors.length) {
            setRawText(res.errors.map((e) => e.url).join("\n"));
          } else {
            setRawText("");
          }
        },
      },
    );
  };

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="batch-output" className="text-sm font-medium">
          Save to folder{" "}
          <span className="text-muted-foreground">(optional)</span>
        </Label>
        <Input
          id="batch-output"
          placeholder="downloads/batch"
          value={destPath}
          onChange={(e) => setDestPath(e.target.value)}
          className="bg-background text-foreground"
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="batch-urls" className="text-sm font-medium">
          URLs <span className="text-muted-foreground">(one per line)</span>
        </Label>
        {dropped && (
          <label className="flex items-center gap-2 text-xs text-muted-foreground">
            <input
              type="checkbox"
              checked={showAll}
              onChange={(e) => setShowAll(e.target.checked)}
            />
            Show all links ({dropped.all.length}) · downloadable only (
            {dropped.downloadable.length})
          </label>
        )}
        <Textarea
          id="batch-urls"
          placeholder={"https://example.com/file1.zip\nhttps://example.com/file2.mp4"}
          value={rawText}
          onChange={(e) => setRawText(e.target.value)}
          className="min-h-[120px] bg-background text-foreground resize-y break-all max-w-full"
          rows={6}
        />
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2">
          <p className="text-xs text-muted-foreground">
            {urls.length} valid URL{urls.length !== 1 && "s"}
            {invalidCount > 0 && (
              <span className="text-destructive">
                {" "}
                · {invalidCount} invalid line{invalidCount !== 1 && "s"}
              </span>
            )}
          </p>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={handlePaste}
            className="gap-2 self-start sm:self-auto"
          >
            <Clipboard className="h-4 w-4" />
            Paste from clipboard
          </Button>
        </div>
      </div>

      {createBatch.isError && (
        <p className="text-sm text-destructive" role="alert">
          {createBatch.error instanceof Error
            ? createBatch.error.message
            : "Batch request failed."}
        </p>
      )}

      {result && (
        <div className="space-y-2 rounded-md border border-border p-3">
          <div className="flex items-center gap-4 text-sm">
            <span className="flex items-center gap-1.5 text-emerald-500">
              <CheckCircle2 className="h-4 w-4" />
              {result.created.length} added
            </span>
            {result.errors.length > 0 && (
              <span className="flex items-center gap-1.5 text-destructive">
                <XCircle className="h-4 w-4" />
                {result.errors.length} failed
              </span>
            )}
          </div>
          {result.errors.length > 0 && (
            <ul className="max-h-40 overflow-y-auto space-y-1 text-xs">
              {result.errors.map((e, idx) => (
                <li
                  key={`${e.url}-${idx}`}
                  className="flex flex-col rounded-sm bg-destructive/5 px-2 py-1"
                >
                  <span className="truncate font-mono text-foreground" title={e.url}>
                    {e.url}
                  </span>
                  <span className="text-destructive">{e.error}</span>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}

      <Button
        type="button"
        onClick={handleSubmit}
        disabled={urls.length === 0 || createBatch.isPending}
        className="w-full gap-2"
      >
        {createBatch.isPending ? (
          <Loader2 className="h-4 w-4 animate-spin" />
        ) : (
          <Send className="h-4 w-4" />
        )}
        Add {urls.length > 0 ? urls.length : ""} download
        {urls.length !== 1 && "s"}
      </Button>
    </div>
  );
}

export default BatchDownloadForm;
