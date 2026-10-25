import { Download, FileVideo, RefreshCw } from "lucide-react";
import { useSettings, useSettingsPatch } from "@/_lib/services/queries/settings.queries";
import {
  useInstallYtdlp,
  useUpdateYtdlp,
  useYtdlpStatus,
} from "@/_lib/services/queries/video.queries";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Spinner } from "@/components/ui/spinner";
import { SettingInput } from "./controls";
import { useSyncedDrafts } from "./draft-sync";
import { ytdlpStatusLine } from "./ytdlp-status";

/**
 * Video downloads (yt-dlp): where the tool is, install or update it, and an
 * optional path to use instead of the one on PATH.
 */
export function YtdlpSettings() {
  const { data: settings } = useSettings();
  const { patch, savedField, errorFor } = useSettingsPatch();
  const { data: status, isLoading, isError } = useYtdlpStatus();
  const install = useInstallYtdlp();
  const update = useUpdateYtdlp();

  const [drafts, setDrafts] = useSyncedDrafts(
    settings ? { path: settings.ytdlp_path ?? "" } : null,
    { path: "" },
  );

  const commitPath = (value: string) => {
    const next = value.trim();
    setDrafts({ path: next });
    if (next === (settings?.ytdlp_path ?? "")) return;
    patch("ytdlp_path", { ytdlp_path: next }, {
      onError: () => setDrafts({ path: settings?.ytdlp_path ?? "" }),
    });
  };

  const busy = install.isPending || update.isPending;

  return (
    <Card className="col-span-full md:col-span-1">
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-lg">
          <FileVideo className="w-5 h-5" /> Video downloads
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <p className="text-sm" role="status">
          {isLoading ? "Checking yt-dlp…" : isError || !status ? "Could not check yt-dlp." : ytdlpStatusLine(status)}
        </p>

        <div className="flex flex-wrap gap-2">
          {status && !status.found && status.can_install && (
            <Button size="sm" onClick={() => install.mutate()} disabled={busy}>
              {install.isPending ? <Spinner /> : <Download className="h-4 w-4" aria-hidden />} Install yt-dlp
            </Button>
          )}
          {status?.found && (
            <Button size="sm" variant="outline" onClick={() => update.mutate()} disabled={busy}>
              {update.isPending ? <Spinner /> : <RefreshCw className="h-4 w-4" aria-hidden />} Update yt-dlp
            </Button>
          )}
        </div>

        <SettingInput
          id="ytdlp-path"
          label="yt-dlp path"
          value={drafts.path}
          placeholder="Auto-detect"
          onChange={(e) => setDrafts({ path: e.target.value })}
          onBlur={() => commitPath(drafts.path)}
          saved={savedField === "ytdlp_path"}
          error={errorFor("ytdlp_path")}
        />
      </CardContent>
    </Card>
  );
}

export default YtdlpSettings;
