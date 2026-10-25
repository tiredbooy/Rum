import { HardDrive } from "lucide-react";
import { cn } from "@/lib/utils";
import { diskFullBannerText } from "@/_lib/disk-full";

interface DiskFullBannerProps {
  /** Downloads currently waiting for disk space. */
  count: number;
  className?: string;
}

/**
 * App-level notice for downloads stopped by a full disk. Not dismissible: it
 * disappears by itself once space is freed and the downloads resume.
 */
export function DiskFullBanner({ count, className }: DiskFullBannerProps) {
  if (count <= 0) return null;
  return (
    <div
      role="alert"
      className={cn(
        "flex items-center gap-3 border-b border-warning/40 bg-warning/15 px-4 py-2 text-sm text-foreground",
        className,
      )}
    >
      <HardDrive className="size-4 shrink-0 text-warning" aria-hidden="true" />
      <span className="flex-1">
        {diskFullBannerText(count)}. Free some space and they resume.
      </span>
    </div>
  );
}

export default DiskFullBanner;
