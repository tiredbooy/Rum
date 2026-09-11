import { useEffect, useState } from "react";
import {
  useSettings,
  useSettingsPatch,
} from "@/_lib/services/queries/settings.queries";
import type { Setting, SettingReq } from "@/_lib/types/setting-types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Hash, Power, ShieldCheck, ShieldAlert, Timer, Wifi } from "lucide-react";
import { SettingInput, SettingToggle } from "./controls";
import { NUM_BOUNDS, clampNum, rangeError, type NumField } from "./numeric-bounds";

type IntegrityToggle =
  | "verify_integrity"
  | "auto_resume_on_reconnect"
  | "auto_resume_on_launch"
  | "block_private_hosts";

const TOGGLES: {
  field: IntegrityToggle;
  label: string;
  help: string;
  icon: React.ReactNode;
  /** Toggles that are ON unless the user turned them off. */
  defaultOn: boolean;
}[] = [
  {
    field: "verify_integrity",
    label: "Verify integrity",
    help: "Hash every finished download so it can be re-checked later.",
    icon: <ShieldCheck className="w-4 h-4" />,
    defaultOn: true,
  },
  {
    field: "auto_resume_on_reconnect",
    label: "Auto-resume on reconnect",
    help: "Retry failed downloads once their host is reachable again.",
    icon: <Wifi className="w-4 h-4" />,
    defaultOn: true,
  },
  {
    field: "auto_resume_on_launch",
    label: "Auto-resume on launch",
    help: "Resume unfinished downloads the next time Rum starts.",
    icon: <Power className="w-4 h-4" />,
    defaultOn: true,
  },
  {
    field: "block_private_hosts",
    label: "Block private hosts",
    help: "Refuse downloads from LAN and loopback addresses. Off for NAS use.",
    icon: <ShieldAlert className="w-4 h-4" />,
    defaultOn: false,
  },
];

const NUMBERS: {
  field: Extract<NumField, "retry_backoff_sec" | "max_retries">;
  label: string;
  icon: React.ReactNode;
  hint: string;
}[] = [
  {
    field: "max_retries",
    label: "Retry attempts",
    icon: <Hash className="w-4 h-4" />,
    hint: "Tries before a download is marked failed.",
  },
  {
    field: "retry_backoff_sec",
    label: "Retry backoff (seconds)",
    icon: <Timer className="w-4 h-4" />,
    hint: "Delay before the first retry; doubles each attempt.",
  },
];

/**
 * Integrity & reliability settings (PATCH /settings): the corruption fix
 * (verify_integrity), the auto-retry/resume policy and the opt-in SSRF guard.
 */
export function IntegritySettings() {
  const { data: settings, isLoading, isError } = useSettings();
  const { patch, savedField, errorFor } = useSettingsPatch();

  const [nums, setNums] = useState<Record<string, string>>({
    max_retries: "3",
    retry_backoff_sec: "1",
  });

  useEffect(() => {
    if (!settings) return;
    setNums({
      max_retries: String(settings.max_retries ?? 3),
      retry_backoff_sec: String(settings.retry_backoff_sec ?? 1),
    });
  }, [settings]);

  const toggleValue = (field: IntegrityToggle, defaultOn: boolean): boolean =>
    (settings as Setting | undefined)?.[field] ?? defaultOn;

  const commitNum = (field: NumField) => {
    const { fallback } = NUM_BOUNDS[field];
    const v = clampNum(nums[field], field);
    setNums((prev) => ({ ...prev, [field]: String(v) }));
    if (v === (settings?.[field] ?? fallback)) return;
    patch(field, { [field]: v } as Partial<SettingReq>, {
      onError: () =>
        setNums((prev) => ({
          ...prev,
          [field]: String(settings?.[field] ?? fallback),
        })),
    });
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
            {TOGGLES.map(({ field, label, help, icon, defaultOn }) => (
              <SettingToggle
                key={field}
                id={`integrity-${field}`}
                icon={icon}
                label={label}
                help={help}
                checked={toggleValue(field, defaultOn)}
                onCheckedChange={(v) =>
                  patch(field, { [field]: v } as Partial<SettingReq>)
                }
                saved={savedField === field}
                error={errorFor(field)}
              />
            ))}

            {NUMBERS.map(({ field, label, icon, hint }) => {
              const { min, max } = NUM_BOUNDS[field];
              return (
                <SettingInput
                  key={field}
                  id={`integrity-${field}`}
                  label={label}
                  icon={icon}
                  type="number"
                  min={min}
                  max={max}
                  hint={hint}
                  value={nums[field]}
                  onChange={(e) =>
                    setNums((prev) => ({ ...prev, [field]: e.target.value }))
                  }
                  onBlur={() => commitNum(field)}
                  saved={savedField === field}
                  error={rangeError(nums[field], field) ?? errorFor(field)}
                />
              );
            })}
          </>
        )}
      </CardContent>
    </Card>
  );
}

export default IntegritySettings;
