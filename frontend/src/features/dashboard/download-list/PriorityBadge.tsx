import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import type { DownloadPriority } from "@/_lib/types/download-types";
import { ChevronsDown, ChevronsUp, Equal } from "lucide-react";
import type { ElementType } from "react";

const config: Record<
  DownloadPriority,
  { label: string; icon: ElementType; className: string }
> = {
  high: {
    label: "High",
    icon: ChevronsUp,
    className: "text-rose-400 border-rose-400/40 bg-rose-400/10",
  },
  normal: {
    label: "Normal",
    icon: Equal,
    className: "text-muted-foreground border-border bg-background/50",
  },
  low: {
    label: "Low",
    icon: ChevronsDown,
    className: "text-sky-400 border-sky-400/40 bg-sky-400/10",
  },
};

interface PriorityBadgeProps {
  priority?: DownloadPriority;
  className?: string;
}

/** Small read-only badge that surfaces a download's current priority. */
export function PriorityBadge({
  priority = "normal",
  className,
}: PriorityBadgeProps) {
  const { label, icon: Icon, className: tone } = config[priority];
  return (
    <Badge
      variant="outline"
      className={cn("gap-1 px-1.5 py-0 text-[10px] font-medium", tone, className)}
    >
      <Icon className="h-3 w-3" />
      {label}
    </Badge>
  );
}

export const priorityConfig = config;
