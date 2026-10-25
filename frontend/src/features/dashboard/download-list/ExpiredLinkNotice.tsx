import { useState } from "react";
import type { Download } from "@/_lib/types/download-types";
import { Button } from "@/components/ui/button";
import { PasteNewLinkDialog } from "./PasteNewLinkDialog";

/** Shown on a failed card whose link expired: say so, and offer the fix. */
export function ExpiredLinkNotice({ download }: { download: Download }) {
  const [open, setOpen] = useState(false);
  return (
    <div className="flex items-center gap-2">
      <p className="text-[10px] text-destructive">Link expired</p>
      <Button
        variant="outline"
        size="sm"
        className="h-6 px-2 text-[11px]"
        onClick={() => setOpen(true)}
      >
        Paste new link
      </Button>
      <PasteNewLinkDialog
        download={download}
        open={open}
        onOpenChange={setOpen}
      />
    </div>
  );
}
