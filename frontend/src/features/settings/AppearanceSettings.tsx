import { useEffect, useState } from "react";
import {
  useSettings,
  useSettingsPatch,
} from "@/_lib/services/queries/settings.queries";
import type { Setting, UIDensity } from "@/_lib/types/setting-types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { Palette, Rows3, Rows4, Sparkles, Volume2 } from "lucide-react";
import {
  ensureNotificationPermission,
  getCompletionSoundEnabled,
  setCompletionSoundEnabled,
  SOUND_PREF_EVENT,
} from "@/_lib/notifications";
import { AccentPicker } from "./AccentPicker";
import { SavedBadge, SettingToggle } from "./controls";

/**
 * Appearance settings (PATCH /settings + a local completion-sound preference):
 * accent color, UI density, reduced motion and the client-side completion
 * sound. Every change applies live via the ThemeProvider, which reads the same
 * settings cache this card writes to.
 */
export function AppearanceSettings() {
  const { data: settings, isLoading, isError } = useSettings();
  const { patch, savedField, errorFor } = useSettingsPatch();

  const [soundOn, setSoundOn] = useState<boolean>(getCompletionSoundEnabled);

  // Keep the sound switch in sync if changed elsewhere (same- or cross-tab).
  useEffect(() => {
    const sync = () => setSoundOn(getCompletionSoundEnabled());
    window.addEventListener(SOUND_PREF_EVENT, sync);
    window.addEventListener("storage", sync);
    return () => {
      window.removeEventListener(SOUND_PREF_EVENT, sync);
      window.removeEventListener("storage", sync);
    };
  }, []);

  const density: UIDensity =
    (settings as Setting | undefined)?.ui_density ?? "comfortable";
  const reducedMotion =
    (settings as Setting | undefined)?.reduced_motion ?? false;

  const toggleSound = async (next: boolean) => {
    setSoundOn(next);
    setCompletionSoundEnabled(next);
    // Turning the sound on is a good moment to ask for notification permission
    // (user gesture) so completion toasts can show too.
    if (next) await ensureNotificationPermission();
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-lg">
          <Palette className="w-5 h-5" /> Appearance
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-6">
        {isLoading ? (
          <p className="text-muted-foreground animate-pulse text-sm">
            Loading appearance preferences…
          </p>
        ) : isError ? (
          <p className="text-destructive text-sm">
            Could not load appearance preferences.
          </p>
        ) : (
          <>
            <AccentPicker
              value={settings?.accent_color ?? ""}
              saved={savedField === "accent_color"}
              error={errorFor("accent_color")}
              onCommit={(accent) => patch("accent_color", { accent_color: accent })}
            />

            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <Label id="appearance-density-label" className="flex items-center gap-2 text-sm">
                  <Rows3 className="w-4 h-4" /> Density
                </Label>
                {savedField === "ui_density" && <SavedBadge />}
              </div>
              <ToggleGroup
                type="single"
                variant="outline"
                value={density}
                aria-labelledby="appearance-density-label"
                onValueChange={(v) => {
                  if (!v || v === density) return;
                  patch("ui_density", { ui_density: v as UIDensity });
                }}
                className="w-full"
              >
                <ToggleGroupItem value="comfortable" className="flex-1 gap-2">
                  <Rows3 className="w-4 h-4" /> Comfortable
                </ToggleGroupItem>
                <ToggleGroupItem value="compact" className="flex-1 gap-2">
                  <Rows4 className="w-4 h-4" /> Compact
                </ToggleGroupItem>
              </ToggleGroup>
              {errorFor("ui_density") && (
                <p role="alert" className="text-xs text-destructive">
                  {errorFor("ui_density")}
                </p>
              )}
            </div>

            <SettingToggle
              id="appearance-reduced-motion"
              icon={<Sparkles className="w-4 h-4" />}
              label="Reduced motion"
              help="Minimize animations and transitions."
              checked={reducedMotion}
              onCheckedChange={(v) => patch("reduced_motion", { reduced_motion: v })}
              saved={savedField === "reduced_motion"}
              error={errorFor("reduced_motion")}
            />

            <SettingToggle
              id="appearance-completion-sound"
              icon={<Volume2 className="w-4 h-4" />}
              label="Completion sound"
              help="Chime when a download finishes. This device only."
              checked={soundOn}
              onCheckedChange={(v) => void toggleSound(v)}
              saved={false}
            />
          </>
        )}
      </CardContent>
    </Card>
  );
}

export default AppearanceSettings;
