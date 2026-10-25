import { Moon } from "lucide-react";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { useDownloadRequestStore } from "@/stores/download-request-store";
import { useNightPlan } from "@/_lib/services/queries/night-plan.queries";

/** "Download at night" switch: assigns the new download to the night queue. */
export function NightToggle() {
  const checked = useDownloadRequestStore((s) => !!s.draft.night_queue);
  const updateDraft = useDownloadRequestStore((s) => s.updateDraft);
  const { data } = useNightPlan();
  const planOn = !!data?.enabled;

  return (
    <div className="flex items-center justify-between gap-3">
      <div className="space-y-0.5">
        <Label
          htmlFor="night-queue"
          className="flex items-center gap-2 text-sm font-medium"
        >
          <Moon className="h-4 w-4" aria-hidden /> Download at night
        </Label>
        <p className="text-xs text-muted-foreground">
          {planOn
            ? `Runs ${data?.start}-${data?.end}.`
            : "Turn on Night plan in Settings."}
        </p>
      </div>
      <Switch
        id="night-queue"
        checked={checked && planOn}
        disabled={!planOn}
        onCheckedChange={(v) => updateDraft({ night_queue: v || undefined })}
      />
    </div>
  );
}

export default NightToggle;
