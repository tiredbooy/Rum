import { useState } from "react";
import {
  useSettings,
  useUpdateSettings,
} from "@/_lib/services/queries/settings.queries";
import type { Setting, SettingReq } from "@/_lib/types/setting-types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Badge } from "@/components/ui/badge";
import { MonitorCog } from "lucide-react";

type ToggleField =
  | "launch_on_startup"
  | "minimize_to_tray"
  | "close_to_tray"
  | "enable_clipboard_watch";

const TOGGLES: {
  field: ToggleField;
  label: string;
  help: string;
}[] = [
  {
    field: "launch_on_startup",
    label: "Launch on startup",
    help: "Start Rum automatically when you log in.",
  },
  {
    field: "minimize_to_tray",
    label: "Minimize to tray",
    help: "Hide the window to the system tray instead of the taskbar when minimized. Windows only in this build; on Linux/macOS the window minimizes normally.",
  },
  {
    field: "close_to_tray",
    label: "Close to tray",
    help: "Keep running in the tray when the window is closed. Windows only in this build; on Linux/macOS closing quits the app.",
  },
  {
    field: "enable_clipboard_watch",
    label: "Watch clipboard for links",
    help: "Detect copied download links automatically. Reads clipboard text only while Rum is running.",
  },
];

/**
 * Desktop preference switches. Each toggle persists immediately via the existing
 * partial PATCH /settings mutation. These fields are only meaningful in the
 * desktop (Wails) build, but they persist harmlessly in browser/dev too. Note
 * minimize_to_tray / close_to_tray are effective only on Windows (the system
 * tray is Windows-only — see the Go side's trayAvailable in tray_others.go);
 * elsewhere they persist but have no effect.
 */
export function DesktopSettings() {
  const { data: settings, isLoading, isError } = useSettings();
  const updateMutation = useUpdateSettings();
  const [savedField, setSavedField] = useState<ToggleField | null>(null);

  const value = (field: ToggleField): boolean =>
    (settings as Setting | undefined)?.[field] ?? false;

  const handleToggle = (field: ToggleField, next: boolean) => {
    updateMutation.mutate({ [field]: next } as Partial<SettingReq>, {
      onSuccess: () => {
        setSavedField(field);
        setTimeout(
          () => setSavedField((f) => (f === field ? null : f)),
          2000,
        );
      },
    });
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-lg">
          <MonitorCog className="w-5 h-5" /> Desktop
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-5">
        {isLoading ? (
          <p className="text-muted-foreground animate-pulse text-sm">
            Loading desktop preferences…
          </p>
        ) : isError ? (
          <p className="text-destructive text-sm">
            Could not load desktop preferences.
          </p>
        ) : (
          TOGGLES.map(({ field, label, help }) => (
            <div
              key={field}
              className="flex items-start justify-between gap-4"
            >
              <div className="space-y-0.5">
                <Label htmlFor={`desktop-${field}`} className="text-sm">
                  {label}
                </Label>
                <p className="text-xs text-muted-foreground">{help}</p>
              </div>
              <div className="flex items-center gap-2 shrink-0">
                {savedField === field && (
                  <Badge
                    variant="outline"
                    className="text-green-600 border-green-600"
                  >
                    Saved
                  </Badge>
                )}
                <Switch
                  id={`desktop-${field}`}
                  checked={value(field)}
                  onCheckedChange={(v) => handleToggle(field, v)}
                />
              </div>
            </div>
          ))
        )}
      </CardContent>
    </Card>
  );
}

export default DesktopSettings;
