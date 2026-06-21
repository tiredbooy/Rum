import { useEffect, useState } from "react";
import {
  useSettings,
  useUpdateSettings,
} from "@/_lib/services/queries/settings.queries";
import type { SettingReq } from "@/_lib/types/setting-types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Download, FileWarning, Gauge, Repeat, Zap } from "lucide-react";
import { SettingInput, SettingSelect, useSavedFlash } from "./controls";

/** Clamp a typed numeric string into [min, max], falling back to `fallback`. */
function clampNum(raw: string, min: number, max: number, fallback: number) {
  const n = raw === "" ? fallback : Math.round(Number(raw));
  if (Number.isNaN(n)) return fallback;
  return Math.min(max, Math.max(min, n));
}

type NumField = "speed_limit_kb" | "max_parallel" | "connections" | "max_retries";

const NUM_BOUNDS: Record<NumField, { min: number; max: number; fallback: number }> = {
  speed_limit_kb: { min: 0, max: 1_000_000, fallback: 0 },
  max_parallel: { min: 1, max: 64, fallback: 3 },
  connections: { min: 1, max: 16, fallback: 8 },
  max_retries: { min: 0, max: 10, fallback: 3 },
};

/**
 * Download limits (PATCH /settings): global speed cap, parallel-download count,
 * connections-per-download (segments) and retry attempts, plus the file-name
 * conflict policy. Numeric fields keep a local draft and commit (clamped) on
 * blur; the conflict select persists instantly. Self-contained so typing here
 * never re-renders the rest of the settings page.
 */
export function DownloadSettings() {
  const { data: settings, isLoading, isError } = useSettings();
  const updateMutation = useUpdateSettings();
  const [savedField, flash] = useSavedFlash();

  const [nums, setNums] = useState<Record<NumField, string>>({
    speed_limit_kb: "0",
    max_parallel: "3",
    connections: "8",
    max_retries: "3",
  });

  useEffect(() => {
    if (!settings) return;
    setNums({
      speed_limit_kb: String(settings.speed_limit_kb ?? 0),
      max_parallel: String(settings.max_parallel ?? 3),
      connections: String(settings.connections ?? 8),
      max_retries: String(settings.max_retries ?? 3),
    });
  }, [settings]);

  const patch = (field: string, payload: Partial<SettingReq>) => {
    updateMutation.mutate(payload, { onSuccess: () => flash(field) });
  };

  const setNum = (field: NumField, value: string) =>
    setNums((prev) => ({ ...prev, [field]: value }));

  const commitNum = (field: NumField) => {
    const { min, max, fallback } = NUM_BOUNDS[field];
    const v = clampNum(nums[field], min, max, fallback);
    setNum(field, String(v));
    if (v !== (settings?.[field] ?? fallback)) {
      patch(field, { [field]: v } as Partial<SettingReq>);
    }
  };

  return (
    <Card className="col-span-full md:col-span-1">
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-lg">
          <Download className="w-5 h-5" /> Downloads
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-5">
        {isLoading ? (
          <p className="text-muted-foreground animate-pulse text-sm">
            Loading download preferences…
          </p>
        ) : isError ? (
          <p className="text-destructive text-sm">
            Could not load download preferences.
          </p>
        ) : (
          <>
            <SettingInput
              label="Speed limit (kB/s)"
              icon={<Gauge className="w-4 h-4" />}
              type="number"
              min={0}
              placeholder="Unlimited"
              value={nums.speed_limit_kb}
              onChange={(e) => setNum("speed_limit_kb", e.target.value)}
              onBlur={() => commitNum("speed_limit_kb")}
              saved={savedField === "speed_limit_kb"}
            />

            <SettingInput
              label="Parallel downloads"
              icon={<Zap className="w-4 h-4" />}
              type="number"
              min={1}
              max={64}
              value={nums.max_parallel}
              onChange={(e) => setNum("max_parallel", e.target.value)}
              onBlur={() => commitNum("max_parallel")}
              saved={savedField === "max_parallel"}
            />

            <SettingInput
              label="Connections per download"
              icon={<Zap className="w-4 h-4" />}
              type="number"
              min={1}
              max={16}
              placeholder="8"
              value={nums.connections}
              onChange={(e) => setNum("connections", e.target.value)}
              onBlur={() => commitNum("connections")}
              saved={savedField === "connections"}
            />

            <SettingInput
              label="Retry attempts"
              icon={<Repeat className="w-4 h-4" />}
              type="number"
              min={0}
              max={10}
              value={nums.max_retries}
              onChange={(e) => setNum("max_retries", e.target.value)}
              onBlur={() => commitNum("max_retries")}
              saved={savedField === "max_retries"}
            />

            <SettingSelect
              label="If file exists"
              icon={<FileWarning className="w-4 h-4" />}
              value={settings?.file_confilict ?? "rename"}
              options={[
                { value: "rename", label: "Rename" },
                { value: "overwrite", label: "Overwrite" },
                { value: "skip", label: "Skip" },
              ]}
              onValueChange={(v) =>
                patch("file_confilict", {
                  file_confilict: v as "rename" | "overwrite" | "skip",
                })
              }
              saved={savedField === "file_confilict"}
            />
          </>
        )}
      </CardContent>
    </Card>
  );
}

export default DownloadSettings;
