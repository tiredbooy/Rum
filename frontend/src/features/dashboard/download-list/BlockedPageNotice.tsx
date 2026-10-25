import { useDownloadAnyway } from "@/_lib/services/queries/download.queries";
import type { Download } from "@/_lib/types/download-types";
import { hasOpenInBrowser, openInBrowser } from "@/_lib/wails";
import { Button } from "@/components/ui/button";
import { ExternalLink, ShieldAlert } from "lucide-react";

export const BLOCKED_PAGE_CODE = "blocked_page";

/** True when the download stopped because the server sent a web page. */
export function isBlockedPage(d: Pick<Download, "status" | "error_code">) {
  return d.status === "error" && d.error_code === BLOCKED_PAGE_CODE;
}

/**
 * Error row for a captive portal / block page. The file was left untouched;
 * "Download anyway" accepts the page once, "Open in browser" lets the user log
 * in to the network (desktop only).
 */
export function BlockedPageNotice({ download }: { download: Download }) {
  const anyway = useDownloadAnyway();
  const host = download.final_host;

  return (
    <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-[10px]">
      <span className="inline-flex items-center gap-1 text-destructive">
        <ShieldAlert className="h-3 w-3" aria-hidden />
        Got a web page{host ? ` from ${host}` : ""}. Blocked or login needed.
      </span>
      <Button
        variant="outline"
        size="sm"
        className="h-6 px-2 text-[10px]"
        disabled={anyway.isPending}
        onClick={() => anyway.mutate(download.id)}
      >
        Download anyway
      </Button>
      {hasOpenInBrowser() && (
        <Button
          variant="ghost"
          size="sm"
          className="h-6 px-2 text-[10px]"
          onClick={() => openInBrowser(download.url)}
        >
          <ExternalLink className="h-3 w-3" aria-hidden />
          Open in browser
        </Button>
      )}
    </div>
  );
}
