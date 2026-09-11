import {
  useSettings,
  useSettingsPatch,
} from "@/_lib/services/queries/settings.queries";
import type { Setting, SettingReq } from "@/_lib/types/setting-types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { MonitorCog } from "lucide-react";
import { SettingToggle } from "./controls";
import { useDesktopCapabilities } from "./useDesktopCapabilities";

type ToggleField =
  | "launch_on_startup"
  | "minimize_to_tray"
  | "close_to_tray"
  | "enable_clipboard_watch";

const TOGGLES: {
  field: ToggleField;
  label: string;
  help: string;
  /** Inert without a system tray (Windows-only in this build). */
  needsTray?: boolean;
}[] = [
  {
    field: "launch_on_startup",
    label: "Launch on startup",
    help: "Start Rum automatically when you log in.",
  },
  {
    field: "minimize_to_tray",
    label: "Minimize to tray",
    help: "Hide the window to the tray instead of the taskbar.",
    needsTray: true,
  },
  {
    field: "close_to_tray",
    label: "Close to tray",
    help: "Keep running in the tray when the window is closed.",
    needsTray: true,
  },
  {
    field: "enable_clipboard_watch",
    label: "Watch clipboard for links",
    help: "Offer to add a download link the moment you copy one.",
  },
];

/**
 * Desktop preference switches. Each toggle persists immediately via PATCH
 * /settings; the Go side picks every one of them up while running.
 *
 * The tray-dependent switches are disabled where this build has no system tray
 * (Linux/macOS — see trayAvailable in tray_others.go) instead of being offered
 * as switches that persist and then do nothing. The capability comes from the
 * App.Capabilities() Wails binding, so the UI never has to guess the platform.
 */
export function DesktopSettings() {
  const { data: settings, isLoading, isError } = useSettings();
  const { patch, savedField, errorFor } = useSettingsPatch();
  const { tray, isDesktop } = useDesktopCapabilities();

  const value = (field: ToggleField): boolean =>
    (settings as Setting | undefined)?.[field] ?? false;

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
          <>
            {!isDesktop && (
              <p className="text-xs text-muted-foreground">
                These apply to the desktop app.
              </p>
            )}
            {TOGGLES.map(({ field, label, help, needsTray }) => {
              const unavailable = !!needsTray && isDesktop && !tray;
              return (
                <SettingToggle
                  key={field}
                  id={`desktop-${field}`}
                  label={label}
                  help={unavailable ? "Not available on this platform." : help}
                  checked={value(field)}
                  disabled={unavailable}
                  onCheckedChange={(v) =>
                    patch(field, { [field]: v } as Partial<SettingReq>)
                  }
                  saved={savedField === field}
                  error={errorFor(field)}
                />
              );
            })}
          </>
        )}
      </CardContent>
    </Card>
  );
}

export default DesktopSettings;
