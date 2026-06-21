import { useEffect, useState } from "react";
import {
  useSettings,
  useUpdateSettings,
} from "@/_lib/services/queries/settings.queries";
import type { SettingReq } from "@/_lib/types/setting-types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { FolderOpen, Monitor } from "lucide-react";
import { chooseDir, hasChooseDir } from "@/_lib/wails";
import {
  SettingInput,
  SettingSelect,
  SettingToggle,
  useSavedFlash,
} from "./controls";

/**
 * General preferences (PATCH /settings): exit confirmation, silent
 * notifications, preferred theme and the default save location. Each control
 * reads live from the settings cache and persists the moment it changes
 * (toggles/select) or on blur (the save-location path), matching the
 * Appearance card — no shared form state, so editing here never re-renders the
 * other settings cards.
 */
export function GeneralSettings() {
  const { data: settings, isLoading, isError } = useSettings();
  const updateMutation = useUpdateSettings();
  const [savedField, flash] = useSavedFlash();

  const [outDir, setOutDir] = useState<string>("");
  const canPick = hasChooseDir();

  useEffect(() => {
    if (!settings) return;
    setOutDir(settings.out_dir ?? "");
  }, [settings]);

  const patch = (field: string, payload: Partial<SettingReq>) => {
    updateMutation.mutate(payload, { onSuccess: () => flash(field) });
  };

  const commitOutDir = (value: string) => {
    setOutDir(value);
    if (value !== (settings?.out_dir ?? "")) {
      patch("out_dir", { out_dir: value });
    }
  };

  const pickFolder = async () => {
    const dir = await chooseDir();
    if (dir) commitOutDir(dir);
  };

  return (
    <Card className="col-span-full md:col-span-1">
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-lg">
          <Monitor className="w-5 h-5" /> General
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-5">
        {isLoading ? (
          <p className="text-muted-foreground animate-pulse text-sm">
            Loading preferences…
          </p>
        ) : isError ? (
          <p className="text-destructive text-sm">
            Could not load preferences.
          </p>
        ) : (
          <>
            <SettingToggle
              id="general-confirm-on-exit"
              label="Confirm on exit"
              checked={settings?.confirm_on_exit ?? false}
              onCheckedChange={(v) =>
                patch("confirm_on_exit", { confirm_on_exit: v })
              }
              saved={savedField === "confirm_on_exit"}
            />

            <SettingToggle
              id="general-silent"
              label="Silent notifications"
              checked={settings?.silent ?? false}
              onCheckedChange={(v) => patch("silent", { silent: v })}
              saved={savedField === "silent"}
            />

            <SettingSelect
              label="Theme"
              icon={<Monitor className="w-4 h-4" />}
              value={settings?.preferred_theme ?? "system"}
              options={[
                { value: "system", label: "System" },
                { value: "light", label: "Light" },
                { value: "dark", label: "Dark" },
              ]}
              onValueChange={(v) =>
                patch("preferred_theme", {
                  preferred_theme: v as "system" | "light" | "dark",
                })
              }
              saved={savedField === "preferred_theme"}
            />

            <div className="space-y-1.5">
              <SettingInput
                label="Save location"
                icon={<FolderOpen className="w-4 h-4" />}
                value={outDir}
                placeholder="~/Downloads"
                onChange={(e) => setOutDir(e.target.value)}
                onBlur={() => commitOutDir(outDir)}
                saved={savedField === "out_dir"}
              />
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
          </>
        )}
      </CardContent>
    </Card>
  );
}

export default GeneralSettings;
