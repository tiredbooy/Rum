import { Badge } from "@/components/ui/badge";
import {
  Loader2,
  CheckCircle,
  XCircle,
  PauseCircle,
  Clock,
} from "lucide-react";

type DownloadStatus =
  | "active"
  | "paused"
  | "completed"
  | "failed"
  | "pending"
  | "queued";

const statusMap: Record<
  DownloadStatus,
  { icon: React.ElementType; label: string; className: string }
> = {
  active: {
    icon: Loader2,
    label: "Active",
    className: "border-blue-500/30 bg-blue-500/10 text-blue-400",
  },
  paused: {
    icon: PauseCircle,
    label: "Paused",
    className: "border-amber-500/30 bg-amber-500/10 text-amber-400",
  },
  completed: {
    icon: CheckCircle,
    label: "Done",
    className: "border-emerald-500/30 bg-emerald-500/10 text-emerald-400",
  },
  failed: {
    icon: XCircle,
    label: "Failed",
    className: "border-red-500/30 bg-red-500/10 text-red-400",
  },
  pending: {
    icon: Clock,
    label: "Pending",
    className: "border-muted-foreground/30 bg-muted/50 text-muted-foreground",
  },
  queued: {
    icon: Clock,
    label: "Queued",
    className: "border-muted-foreground/30 bg-muted/50 text-muted-foreground",
  },
};

export function StatusBadge({ status }: { status: DownloadStatus }) {
  const config = statusMap[status] ?? statusMap.queued;
  const Icon = config.icon;
  return (
    <Badge
      variant="outline"
      className={`gap-1.5 px-2 py-0.5 ${config.className}`}
    >
      <Icon className="h-3.5 w-3.5" />
      <span className="text-xs">{config.label}</span>
    </Badge>
  );
}
