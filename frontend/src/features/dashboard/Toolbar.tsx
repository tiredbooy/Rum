import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import {
  CheckCheck,
  Pause,
  Play,
  Plus,
  Trash2,
} from "lucide-react";
import StatsCards from "./StatsCards";
import AggregateEtaBanner from "./AggregateEtaBanner";

interface Stats {
  active: number;
  completedToday: number;
  speed: string;
  dataToday: string;
  failedCount: number;
}

interface DashboardToolbarProps {
  stats: Stats;
  onAddDownload: () => void;
  onPauseAll: () => void;
  onResumeAll: () => void;
  onRetryFailed: () => void;
  onClearCompleted: () => void;
  /** Delete every download (running, queued, completed, failed). */
  onDeleteAll: () => void;
}

export function DashboardToolbar({
  stats,
  onAddDownload,
  onPauseAll,
  onResumeAll,
  onClearCompleted,
  onDeleteAll,
}: DashboardToolbarProps) {
  return (
    <div className="sticky top-0 z-10 pb-8 space-y-5">
      <div className="absolute inset-0 bg-background/70 backdrop-blur-2xl -z-10" />

      <StatsCards />

      <AggregateEtaBanner />

      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Button
            onClick={onAddDownload}
            size="default"
            className="gap-2 shadow-lg shadow-primary/20 hover:shadow-primary/30 transition-shadow"
          >
            <Plus className="w-4 h-4" />
            Add Download
          </Button>

          <Separator orientation="vertical" className="h-6" />

          <div className="flex items-center gap-1">
            <Button
              variant="ghost"
              size="icon"
              onClick={onPauseAll}
              className="h-9 w-9 text-muted-foreground hover:text-foreground hover:bg-muted/50 transition-colors"
              title="Pause all"
            >
              <Pause className="w-4 h-4" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              onClick={onResumeAll}
              className="h-9 w-9 text-muted-foreground hover:text-foreground hover:bg-muted/50 transition-colors"
              title="Resume all"
            >
              <Play className="w-4 h-4" />
            </Button>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={onClearCompleted}
            disabled={stats.completedToday === 0}
            className="gap-1.5 h-9 hover:text-foreground text-foreground/80"
          >
            <CheckCheck className="w-3.5 h-3.5" />
            <span className="hidden sm:inline">Clear completed</span>
          </Button>

          <Button
            variant="outline"
            size="sm"
            onClick={onDeleteAll}
            className="gap-1.5 h-9 border-destructive/40 text-destructive hover:bg-destructive/10 hover:text-destructive"
            title="Remove all from list"
          >
            <Trash2 className="w-3.5 h-3.5" />
            <span className="hidden sm:inline">Remove all</span>
          </Button>
        </div>
      </div>
    </div>
  );
}
