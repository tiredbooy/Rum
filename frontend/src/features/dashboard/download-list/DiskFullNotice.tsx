import type { Download } from "@/_lib/types/download-types";
import { diskFullHeadline } from "@/_lib/disk-full";
import { Button } from "@/components/ui/button";
import { HardDrive, RotateCcw } from "lucide-react";

interface DiskFullNoticeProps {
  download: Download;
  onRetry?: (id: string) => void;
}

/**
 * Shown on a card whose download stopped because the disk is full. The partial
 * file is kept and the app resumes by itself once space is free, so the only
 * action is an immediate Retry.
 */
export function DiskFullNotice({ download, onRetry }: DiskFullNoticeProps) {
  const { id, disk_path } = download;
  return (
    <div
      role="alert"
      className="flex items-start gap-2 rounded-md border border-amber-400/50 bg-amber-400/10 px-2 py-1.5"
    >
      <HardDrive className="mt-0.5 size-4 shrink-0 text-amber-400" aria-hidden="true" />
      <div className="min-w-0 flex-1">
        <p className="text-xs font-medium">{diskFullHeadline(download)}</p>
        <p className="truncate text-[10px] text-muted-foreground" title={disk_path}>
          {disk_path ? `${disk_path} · ` : ""}Resumes by itself
        </p>
      </div>
      <Button
        variant="outline"
        size="sm"
        className="h-7 shrink-0 gap-1"
        onClick={() => onRetry?.(id)}
        aria-label="Retry"
      >
        <RotateCcw className="size-3.5" aria-hidden="true" />
        Retry
      </Button>
    </div>
  );
}

export default DiskFullNotice;
