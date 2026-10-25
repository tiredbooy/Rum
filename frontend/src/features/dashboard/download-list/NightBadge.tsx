import { Moon } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { nightChip, type NightChip } from "@/_lib/night-plan";
import { useNightPlan } from "@/_lib/services/queries/night-plan.queries";
import type { Download } from "@/_lib/types/download-types";

const TONE: Record<NightChip["tone"], string> = {
  waiting: "text-indigo-400 border-indigo-400/40 bg-indigo-400/10",
  active: "text-emerald-400 border-emerald-400/40 bg-emerald-400/10",
  capped: "text-amber-500 border-amber-500/40 bg-amber-500/10",
};

/** Status chip for a night-queue download ("Starts 02:00", "Night"). */
export function NightBadge({
  download,
  className,
}: {
  download: Pick<Download, "night_queue" | "status" | "error_code">;
  className?: string;
}) {
  const { data } = useNightPlan();
  const chip = nightChip(download, data);
  if (!chip) return null;
  return (
    <Badge
      variant="outline"
      className={cn(
        "gap-1 px-1.5 py-0 text-[10px] font-medium",
        TONE[chip.tone],
        className,
      )}
    >
      <Moon className="h-3 w-3" aria-hidden />
      {chip.label}
    </Badge>
  );
}

export default NightBadge;
