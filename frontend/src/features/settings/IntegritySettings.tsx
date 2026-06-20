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
import { ShieldCheck, Wifi, Power, Timer, Hash } from "lucide-react";

type IntegrityToggle =
  | "verify_integrity"
  | "auto_resume_on_reconnect"
  | "auto_resume_on_launch";

const TOGGLES: { field: IntegrityToggle; label: string; help: string }[] = [
  {
    field: "verify_integrity",
    label: "Verify integrity",
    help: "Check every finished download for corruption and store a hash for server-free re-verification. Recommended.",
  },
  {
    field: "auto_resume_on_reconnect",
    label: "Auto-resume on reconnect",
    help: "Automatically continue interrupted downloads when the network comes back.",
  },
  {
    field: "auto_resume_on_launch",
    label: "Auto-resume on launch",
    help: "Resume unfinished downloads the next time Rum starts.",
  },
];

const RETRY_ICONS: Record<IntegrityToggle, React.ReactNode> = {
  verify_integrity: <ShieldCheck className="w-4 h-4" />,
  auto_resume_on_reconnect: <Wifi className="w-4 h-4" />,
  auto_resume_on_launch: <Power className="w-4 h-4" />,
};

function clampBackoff(n: number): number {
  if (Number.isNaN(n)) return 1;
  return Math.min(60, Math.max(1, Math.round(n)));
}

/**
 * Integrity & reliability settings (PATCH /settings). Surfaces the corruption
 * fix (verify_integrity) plus the auto-retry/resume policy: reconnect/launch
 * resume, retry backoff, and the existing max_retries for context.
 */
export function IntegritySettings() {
  const { data: settings, isLoading, isError } = useSettings();
  const updateMutation = useUpdateSettings();
  const [savedField, setSavedField] = useState<string | null>(null);

  // Local controlled values for the numeric inputs (commit on blur).
  const [backoff, setBackoff] = useState<string>("1");
  const [retries, setRetries] = useState<string>("3");

  useEffect(() => {
    if (!settings) return;
    setBackoff(String(settings.retry_backoff_sec ?? 1));
    setRetries(String(settings.max_retries ?? 3));
  }, [settings]);

  const value = (field: IntegrityToggle): boolean => {
    const s = settings as Setting | undefined;
    // verify_integrity / auto_resume_* default true.
    return s?.[field] ?? true;
  };

  const flash = (field: string) => {
    setSavedField(field);
    setTimeout(() => setSavedField((f) => (f === field ? null : f)), 2000);
  };

  const patch = (field: string, payload: Partial<SettingReq>) => {
    updateMutation.mutate(payload, { onSuccess: () => flash(field) });
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-lg">
          <ShieldCheck className="w-5 h-5" /> Integrity &amp; reliability
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-5">
        {isLoading ? (
          <p className="text-muted-foreground animate-pulse text-sm">
            Loading reliability preferences…
          </p>
        ) : isError ? (
          <p className="text-destructive text-sm">
            Could not load reliability preferences.
          </p>
        ) : (
          <>
            {TOGGLES.map(({ field, label, help }) => (
              <div
                key={field}
                className="flex items-start justify-between gap-4"
              >
                <div className="space-y-0.5">
                  <Label
                    htmlFor={`integrity-${field}`}
                    className="flex items-center gap-2 text-sm"
                  >
                    {RETRY_ICONS[field]}
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
                    id={`integrity-${field}`}
                    checked={value(field)}
                    onCheckedChange={(v) =>
                      patch(field, { [field]: v } as Partial<SettingReq>)
                    }
                  />
                </div>
              </div>
            ))}

            {/* Retry backoff (1–60s) */}
            <div className="space-y-1.5">
              <Label
                htmlFor="integrity-backoff"
                className="flex items-center gap-2 text-sm"
              >
                <Timer className="w-4 h-4" /> Retry backoff (seconds)
              </Label>
              <div className="relative">
                <Input
                  id="integrity-backoff"
                  type="number"
                  min={1}
                  max={60}
                  className="pr-16"
                  value={backoff}
                  onChange={(e) => setBackoff(e.target.value)}
                  onBlur={() => {
                    const v = clampBackoff(Number(backoff));
                    setBackoff(String(v));
                    if (v !== (settings?.retry_backoff_sec ?? 1)) {
                      patch("retry_backoff_sec", { retry_backoff_sec: v });
                    }
                  }}
                />
                {savedField === "retry_backoff_sec" && (
                  <Badge
                    variant="outline"
                    className="absolute right-2 top-1/2 -translate-y-1/2 text-green-600 border-green-600"
                  >
                    Saved
                  </Badge>
                )}
              </div>
              <p className="text-xs text-muted-foreground">
                Base delay before the first retry; doubles each attempt
                (1–60&nbsp;s).
              </p>
            </div>

            {/* Max retries (existing field, surfaced here) */}
            <div className="space-y-1.5">
              <Label
                htmlFor="integrity-retries"
                className="flex items-center gap-2 text-sm"
              >
                <Hash className="w-4 h-4" /> Retry attempts
              </Label>
              <div className="relative">
                <Input
                  id="integrity-retries"
                  type="number"
                  min={0}
                  max={10}
                  className="pr-16"
                  value={retries}
                  onChange={(e) => setRetries(e.target.value)}
                  onBlur={() => {
                    const v = Math.min(
                      10,
                      Math.max(0, Math.round(Number(retries) || 0)),
                    );
                    setRetries(String(v));
                    if (v !== (settings?.max_retries ?? 3)) {
                      patch("max_retries", { max_retries: v });
                    }
                  }}
                />
                {savedField === "max_retries" && (
                  <Badge
                    variant="outline"
                    className="absolute right-2 top-1/2 -translate-y-1/2 text-green-600 border-green-600"
                  >
                    Saved
                  </Badge>
                )}
              </div>
              <p className="text-xs text-muted-foreground">
                How many times to retry a failed download (0–10).
              </p>
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}

export default IntegritySettings;
