import { ArrowUpCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

interface UpdateBannerProps {
  /** e.g. "Rum v1.2.0 available". */
  text: string;
  /** A download is in progress. */
  busy: boolean;
  onDownload: () => void;
  onLater: () => void;
  className?: string;
}

/** Non-blocking notice that a newer release exists. Never installs anything. */
export function UpdateBanner({ text, busy, onDownload, onLater, className }: UpdateBannerProps) {
  return (
    <div
      role="status"
      aria-live="polite"
      className={cn(
        "flex items-center gap-3 border-b border-primary/30 bg-primary/10 px-4 py-2 text-sm text-foreground",
        className,
      )}
    >
      <ArrowUpCircle className="size-4 shrink-0 text-primary" aria-hidden="true" />
      <span className="flex-1">{text}</span>
      <Button size="sm" onClick={onDownload} disabled={busy}>
        {busy ? "Downloading…" : "Download"}
      </Button>
      <Button size="sm" variant="ghost" onClick={onLater} disabled={busy}>
        Later
      </Button>
    </div>
  );
}

export default UpdateBanner;
