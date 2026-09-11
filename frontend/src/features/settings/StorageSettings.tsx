import { useEffect, useState } from "react";
import {
  useSettings,
  useSettingsPatch,
} from "@/_lib/services/queries/settings.queries";
import type { Setting } from "@/_lib/types/setting-types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { FileArchive, HardDrive } from "lucide-react";
import { SettingToggle } from "./controls";
import { SettingPathInput } from "./SettingPathInput";
import { useFolderPicker } from "./useFolderPicker";

/**
 * Storage settings (PATCH /settings): the temp directory for in-progress files
 * (with a native folder picker when running in the desktop shell) and whether
 * to keep partial data on a failed download so it can be resumed/repaired.
 */
export function StorageSettings() {
  const { data: settings, isLoading, isError } = useSettings();
  const { patch, savedField, errorFor } = useSettingsPatch();
  const { canPick, pick } = useFolderPicker();

  const [tempDir, setTempDir] = useState<string>("");

  useEffect(() => {
    if (!settings) return;
    setTempDir(settings.temp_dir ?? "");
  }, [settings]);

  const commitTempDir = (value: string) => {
    const next = value.trim();
    setTempDir(next);
    if (next === (settings?.temp_dir ?? "")) return;
    patch("temp_dir", { temp_dir: next }, {
      onError: () => setTempDir(settings?.temp_dir ?? ""),
    });
  };

  const pickFolder = async () => {
    const dir = await pick();
    if (dir) commitTempDir(dir);
  };

  const keepPartial =
    (settings as Setting | undefined)?.keep_partial_on_failure ?? true;

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-lg">
          <HardDrive className="w-5 h-5" /> Storage
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-5">
        {isLoading ? (
          <p className="text-muted-foreground animate-pulse text-sm">
            Loading storage preferences…
          </p>
        ) : isError ? (
          <p className="text-destructive text-sm">
            Could not load storage preferences.
          </p>
        ) : (
          <>
            <SettingPathInput
              label="Temp directory"
              value={tempDir}
              placeholder="Next to the final file"
              hint="Where in-progress files live until they finish."
              saved={savedField === "temp_dir"}
              error={errorFor("temp_dir")}
              canBrowse={canPick}
              onChange={setTempDir}
              onCommit={commitTempDir}
              onBrowse={() => void pickFolder()}
            />

            <SettingToggle
              id="storage-keep-partial"
              icon={<FileArchive className="w-4 h-4" />}
              label="Keep partial on failure"
              help="Keep downloaded data when a download fails so it can be resumed."
              checked={keepPartial}
              onCheckedChange={(v) =>
                patch("keep_partial_on_failure", {
                  keep_partial_on_failure: v,
                })
              }
              saved={savedField === "keep_partial_on_failure"}
              error={errorFor("keep_partial_on_failure")}
            />
          </>
        )}
      </CardContent>
    </Card>
  );
}

export default StorageSettings;
