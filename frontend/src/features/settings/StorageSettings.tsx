import { useEffect, useState } from "react";
import {
  useSettings,
  useUpdateSettings,
} from "@/_lib/services/queries/settings.queries";
import type { Setting, SettingReq } from "@/_lib/types/setting-types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { HardDrive, FolderOpen, FileArchive } from "lucide-react";
import { chooseDir, hasChooseDir } from "@/_lib/wails";

/**
 * Storage settings (PATCH /settings): the temp directory for in-progress files
 * (with a native folder picker when running in the desktop shell) and whether
 * to keep partial data on a failed download so it can be resumed/repaired.
 */
export function StorageSettings() {
  const { data: settings, isLoading, isError } = useSettings();
  const updateMutation = useUpdateSettings();
  const [savedField, setSavedField] = useState<string | null>(null);
  const [tempDir, setTempDir] = useState<string>("");
  const canPick = hasChooseDir();

  useEffect(() => {
    if (!settings) return;
    setTempDir(settings.temp_dir ?? "");
  }, [settings]);

  const flash = (field: string) => {
    setSavedField(field);
    setTimeout(() => setSavedField((f) => (f === field ? null : f)), 2000);
  };

  const patch = (field: string, payload: Partial<SettingReq>) => {
    updateMutation.mutate(payload, { onSuccess: () => flash(field) });
  };

  const commitTempDir = (value: string) => {
    setTempDir(value);
    if (value !== (settings?.temp_dir ?? "")) {
      patch("temp_dir", { temp_dir: value });
    }
  };

  const pickFolder = async () => {
    const dir = await chooseDir();
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
            {/* Temp directory */}
            <div className="space-y-1.5">
              <Label
                htmlFor="storage-temp-dir"
                className="flex items-center gap-2 text-sm"
              >
                <FolderOpen className="w-4 h-4" /> Temp directory
              </Label>
              <div className="flex items-center gap-2">
                <div className="relative flex-1">
                  <Input
                    id="storage-temp-dir"
                    className="pr-16"
                    placeholder="Next to the final file"
                    value={tempDir}
                    onChange={(e) => setTempDir(e.target.value)}
                    onBlur={() => commitTempDir(tempDir)}
                  />
                  {savedField === "temp_dir" && (
                    <Badge
                      variant="outline"
                      className="absolute right-2 top-1/2 -translate-y-1/2 text-green-600 border-green-600"
                    >
                      Saved
                    </Badge>
                  )}
                </div>
                {canPick && (
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => void pickFolder()}
                  >
                    <FolderOpen className="w-4 h-4" /> Browse
                  </Button>
                )}
              </div>
              <p className="text-xs text-muted-foreground">
                Where in-progress files live until they finish. Leave empty to
                write next to the final file.
              </p>
            </div>

            {/* Keep partial on failure */}
            <div className="flex items-start justify-between gap-4">
              <div className="space-y-0.5">
                <Label
                  htmlFor="storage-keep-partial"
                  className="flex items-center gap-2 text-sm"
                >
                  <FileArchive className="w-4 h-4" /> Keep partial on failure
                </Label>
                <p className="text-xs text-muted-foreground">
                  Keep downloaded data when a download fails so it can be resumed
                  or repaired. Explicit deletes always remove partials.
                </p>
              </div>
              <div className="flex items-center gap-2 shrink-0">
                {savedField === "keep_partial_on_failure" && (
                  <Badge
                    variant="outline"
                    className="text-green-600 border-green-600"
                  >
                    Saved
                  </Badge>
                )}
                <Switch
                  id="storage-keep-partial"
                  checked={keepPartial}
                  onCheckedChange={(v) =>
                    patch("keep_partial_on_failure", {
                      keep_partial_on_failure: v,
                    })
                  }
                />
              </div>
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}

export default StorageSettings;
