import { useEffect, useState } from "react";
import {
  useSettings,
  useSettingsPatch,
} from "@/_lib/services/queries/settings.queries";
import type { LogLevel } from "@/_lib/types/setting-types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Monitor, ScrollText } from "lucide-react";
import { SettingSelect, SettingToggle } from "./controls";
import { SettingPathInput } from "./SettingPathInput";
import { useFolderPicker } from "./useFolderPicker";

/**
 * General preferences (PATCH /settings): exit confirmation, silent
 * notifications, preferred theme and the default save location. Each control
 * reads live from the settings cache and persists the moment it changes
 * (toggles/select) or on blur (the save-location path).
 */
export function GeneralSettings() {
  const { data: settings, isLoading, isError } = useSettings();
  const { patch, savedField, errorFor } = useSettingsPatch();
  const { canPick, pick } = useFolderPicker();

  const [outDir, setOutDir] = useState<string>("");

  useEffect(() => {
    if (!settings) return;
    setOutDir(settings.out_dir ?? "");
  }, [settings]);

  const commitOutDir = (value: string) => {
    const next = value.trim();
    setOutDir(next);
    if (next === (settings?.out_dir ?? "")) return;
    patch("out_dir", { out_dir: next }, {
      // Put the field back to what the server still holds, so a rejected path
      // never lingers in the box looking saved.
      onError: () => setOutDir(settings?.out_dir ?? ""),
    });
  };

  const pickFolder = async () => {
    const dir = await pick();
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
          <p className="text-destructive text-sm">Could not load preferences.</p>
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
              error={errorFor("confirm_on_exit")}
            />

            <SettingToggle
              id="general-silent"
              label="Silent notifications"
              checked={settings?.silent ?? false}
              onCheckedChange={(v) => patch("silent", { silent: v })}
              saved={savedField === "silent"}
              error={errorFor("silent")}
            />

            <SettingSelect
              id="general-theme"
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
              error={errorFor("preferred_theme")}
            />

            <SettingSelect
              id="general-log-level"
              label="Logging"
              icon={<ScrollText className="w-4 h-4" />}
              // Only "debug" changes behaviour; a legacy warn/error value logs
              // like Normal, so show it as Normal rather than as no selection.
              value={settings?.log_level === "debug" ? "debug" : "info"}
              options={[
                { value: "info", label: "Normal" },
                { value: "debug", label: "Debug" },
              ]}
              onValueChange={(v) =>
                patch("log_level", { log_level: v as LogLevel })
              }
              saved={savedField === "log_level"}
              error={errorFor("log_level")}
              hint="Debug records a detailed download log for bug reports."
            />

            <SettingPathInput
              label="Save location"
              value={outDir}
              placeholder="~/Downloads"
              hint="Empty uses your Downloads folder."
              saved={savedField === "out_dir"}
              error={errorFor("out_dir")}
              canBrowse={canPick}
              onChange={setOutDir}
              onCommit={commitOutDir}
              onBrowse={() => void pickFolder()}
            />
          </>
        )}
      </CardContent>
    </Card>
  );
}

export default GeneralSettings;
