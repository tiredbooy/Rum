import { useEffect, useState } from "react";
import {
  useSettings,
  useUpdateSettings,
} from "@/_lib/services/queries/settings.queries";
import type {
  Setting,
  SettingReq,
  UIDensity,
} from "@/_lib/types/setting-types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  ToggleGroup,
  ToggleGroupItem,
} from "@/components/ui/toggle-group";
import { Palette, Sparkles, Rows3, Rows4, Volume2, RotateCcw } from "lucide-react";
import { cn } from "@/lib/utils";
import { isValidAccent, normalizeHex } from "@/_lib/theme";
import {
  getCompletionSoundEnabled,
  setCompletionSoundEnabled,
  ensureNotificationPermission,
  SOUND_PREF_EVENT,
} from "@/_lib/notifications";

/** A few tasteful accent presets users can one-click apply. */
const PRESETS: { name: string; hex: string }[] = [
  { name: "Indigo", hex: "#6366f1" },
  { name: "Violet", hex: "#8b5cf6" },
  { name: "Sky", hex: "#0ea5e9" },
  { name: "Emerald", hex: "#10b981" },
  { name: "Amber", hex: "#f59e0b" },
  { name: "Rose", hex: "#f43f5e" },
];

/**
 * Appearance settings (PATCH /settings + a local completion-sound preference):
 * accent color (with presets + reset to app default), UI density, reduced
 * motion, and the client-side completion sound toggle. Every change applies
 * live via the ThemeProvider (which reads the settings cache).
 */
export function AppearanceSettings() {
  const { data: settings, isLoading, isError } = useSettings();
  const updateMutation = useUpdateSettings();
  const [savedField, setSavedField] = useState<string | null>(null);

  // Local accent value mirrors the color input; "" = app default.
  const [accent, setAccent] = useState<string>("");
  const [soundOn, setSoundOn] = useState<boolean>(() =>
    getCompletionSoundEnabled(),
  );

  useEffect(() => {
    if (!settings) return;
    setAccent(settings.accent_color ?? "");
  }, [settings]);

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

  const flash = (field: string) => {
    setSavedField(field);
    setTimeout(() => setSavedField((f) => (f === field ? null : f)), 2000);
  };

  const patch = (field: string, payload: Partial<SettingReq>) => {
    updateMutation.mutate(payload, { onSuccess: () => flash(field) });
  };

  const commitAccent = (raw: string) => {
    // Empty string resets to the app default; otherwise normalize/validate.
    const next = raw === "" ? "" : isValidAccent(raw) ? normalizeHex(raw) : "";
    setAccent(next);
    if (next !== (settings?.accent_color ?? "")) {
      patch("accent_color", { accent_color: next });
    }
  };

  const density: UIDensity =
    (settings as Setting | undefined)?.ui_density ?? "comfortable";
  const reducedMotion =
    (settings as Setting | undefined)?.reduced_motion ?? false;

  // The swatch needs a concrete color even when "default"; show the live
  // computed primary so the picker opens near the current accent.
  const swatchValue = isValidAccent(accent) ? normalizeHex(accent) : "#6366f1";

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
            {/* Accent color */}
            <div className="space-y-2.5">
              <div className="flex items-center justify-between">
                <Label className="flex items-center gap-2 text-sm">
                  <Palette className="w-4 h-4" /> Accent color
                </Label>
                {savedField === "accent_color" && (
                  <Badge
                    variant="outline"
                    className="text-green-600 border-green-600"
                  >
                    Saved
                  </Badge>
                )}
              </div>

              <div className="flex items-center gap-2">
                <span
                  className="relative inline-flex h-9 w-9 overflow-hidden rounded-md border shadow-xs"
                  style={{
                    backgroundColor: accent === "" ? "var(--primary)" : undefined,
                  }}
                >
                  <input
                    aria-label="Pick accent color"
                    type="color"
                    value={swatchValue}
                    className="rum-swatch h-full w-full rounded-md"
                    onChange={(e) => setAccent(e.target.value)}
                    onBlur={() => commitAccent(accent)}
                  />
                </span>
                <code className="rounded bg-muted px-2 py-1 text-xs text-muted-foreground">
                  {accent === "" ? "Default" : normalizeHex(accent)}
                </code>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="ml-auto"
                  disabled={accent === ""}
                  onClick={() => commitAccent("")}
                >
                  <RotateCcw className="w-4 h-4" /> Reset
                </Button>
              </div>

              <div className="flex flex-wrap gap-2 pt-0.5">
                {PRESETS.map((p) => {
                  const active = normalizeHex(accent) === p.hex;
                  return (
                    <button
                      key={p.hex}
                      type="button"
                      aria-label={`Accent ${p.name}`}
                      title={p.name}
                      onClick={() => commitAccent(p.hex)}
                      className={cn(
                        "h-6 w-6 rounded-full border transition-transform hover:scale-110 focus-visible:scale-110",
                        active && "ring-2 ring-ring ring-offset-2 ring-offset-background",
                      )}
                      style={{ backgroundColor: p.hex }}
                    />
                  );
                })}
              </div>
              <p className="text-xs text-muted-foreground">
                Tints buttons, links and highlights. Reset to use the app
                default.
              </p>
            </div>

            {/* UI density */}
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <Label className="flex items-center gap-2 text-sm">
                  <Rows3 className="w-4 h-4" /> Density
                </Label>
                {savedField === "ui_density" && (
                  <Badge
                    variant="outline"
                    className="text-green-600 border-green-600"
                  >
                    Saved
                  </Badge>
                )}
              </div>
              <ToggleGroup
                type="single"
                variant="outline"
                value={density}
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
            </div>

            {/* Reduced motion */}
            <div className="flex items-start justify-between gap-4">
              <div className="space-y-0.5">
                <Label
                  htmlFor="appearance-reduced-motion"
                  className="flex items-center gap-2 text-sm"
                >
                  <Sparkles className="w-4 h-4" /> Reduced motion
                </Label>
                <p className="text-xs text-muted-foreground">
                  Minimize animations and transitions throughout the app.
                </p>
              </div>
              <div className="flex items-center gap-2 shrink-0">
                {savedField === "reduced_motion" && (
                  <Badge
                    variant="outline"
                    className="text-green-600 border-green-600"
                  >
                    Saved
                  </Badge>
                )}
                <Switch
                  id="appearance-reduced-motion"
                  checked={reducedMotion}
                  onCheckedChange={(v) =>
                    patch("reduced_motion", { reduced_motion: v })
                  }
                />
              </div>
            </div>

            {/* Completion sound (local-only preference) */}
            <div className="flex items-start justify-between gap-4">
              <div className="space-y-0.5">
                <Label
                  htmlFor="appearance-completion-sound"
                  className="flex items-center gap-2 text-sm"
                >
                  <Volume2 className="w-4 h-4" /> Completion sound
                </Label>
                <p className="text-xs text-muted-foreground">
                  Play a short chime when a download finishes. Stored on this
                  device only.
                </p>
              </div>
              <Switch
                id="appearance-completion-sound"
                checked={soundOn}
                onCheckedChange={(v) => void toggleSound(v)}
              />
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}

export default AppearanceSettings;
