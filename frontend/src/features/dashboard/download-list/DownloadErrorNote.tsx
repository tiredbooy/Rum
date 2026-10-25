import { useState } from "react";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import { describeDownloadError } from "@/_lib/utils/download-errors";

interface DownloadErrorNoteProps {
  /** Backend error_code; missing for old jobs, which fall back to "unknown". */
  code?: string;
  /** Raw error text, kept for bug reports. */
  error: string;
}

/** Plain-language failure reason on a card, with the raw error behind "Details". */
export function DownloadErrorNote({ code, error }: DownloadErrorNoteProps) {
  const [open, setOpen] = useState(false);
  const { message, action } = describeDownloadError(code);

  return (
    <Collapsible open={open} onOpenChange={setOpen} className="space-y-1">
      <p className="text-xs text-destructive" title={error}>
        <span className="font-medium">{message}</span> {action}{" "}
        <CollapsibleTrigger className="cursor-pointer rounded text-muted-foreground underline underline-offset-2 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
          {open ? "Hide details" : "Details"}
        </CollapsibleTrigger>
      </p>
      <CollapsibleContent>
        <code className="block select-text break-all rounded bg-muted px-2 py-1 text-[11px] text-muted-foreground">
          {error}
        </code>
      </CollapsibleContent>
    </Collapsible>
  );
}
