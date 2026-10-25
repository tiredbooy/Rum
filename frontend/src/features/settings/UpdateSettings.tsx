import { RefreshCw } from "lucide-react";
import {
  useSettings,
  useSettingsPatch,
} from "@/_lib/services/queries/settings.queries";
import {
  useCheckForUpdate,
  useUpdateStatus,
} from "@/_lib/services/queries/update.queries";
import { showUpdateBanner, updateSummary } from "@/_lib/update-notice";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useUpdateDownload } from "@/features/layout/useUpdateDownload";
import { SettingToggle } from "./controls";

/** Updates: automatic check switch, "Check now", and the current result. */
export function UpdateSettings() {
  const { data: settings, isLoading, isError } = useSettings();
  const { patch, savedField, errorFor } = useSettingsPatch();
  const { data: status } = useUpdateStatus();
  const check = useCheckForUpdate();
  const { start, busy } = useUpdateDownload(status);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-lg">
          <RefreshCw className="w-5 h-5" /> Updates
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-5">
        {isLoading ? (
          <p className="text-muted-foreground animate-pulse text-sm">Loading…</p>
        ) : isError ? (
          <p className="text-destructive text-sm">Could not load preferences.</p>
        ) : (
          <SettingToggle
            id="update-check-for-updates"
            label="Check for updates"
            help="Once a day, when Rum starts."
            checked={settings?.check_for_updates ?? true}
            onCheckedChange={(v) => patch("check_for_updates", { check_for_updates: v })}
            saved={savedField === "check_for_updates"}
            error={errorFor("check_for_updates")}
          />
        )}
        <div className="flex flex-wrap items-center gap-2">
          <Button
            variant="outline"
            onClick={() => check.mutate(false)}
            disabled={check.isPending}
          >
            <RefreshCw className="w-4 h-4" /> Check now
          </Button>
          {showUpdateBanner(status) && (
            <Button onClick={start} disabled={busy}>
              {busy ? "Downloading…" : "Download"}
            </Button>
          )}
          <span role="status" aria-live="polite" className="text-sm text-muted-foreground">
            {updateSummary(status, check.isPending)}
          </span>
        </div>
      </CardContent>
    </Card>
  );
}

export default UpdateSettings;
