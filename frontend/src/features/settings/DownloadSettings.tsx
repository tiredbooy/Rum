import { useEffect, useState } from "react";
import {
  useSettings,
  useSettingsPatch,
} from "@/_lib/services/queries/settings.queries";
import type { SettingReq } from "@/_lib/types/setting-types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Download, FileWarning, Gauge, Zap } from "lucide-react";
import { SettingInput, SettingSelect } from "./controls";
import { NUM_BOUNDS, clampNum, rangeError, type NumField } from "./numeric-bounds";

/** The numeric fields this card owns (retries/backoff live in Integrity). */
type DownloadNumField = Extract<
  NumField,
  "speed_limit_kb" | "max_parallel" | "connections"
>;

const FIELDS: {
  field: DownloadNumField;
  label: string;
  icon: React.ReactNode;
  hint: string;
  placeholder?: string;
}[] = [
  {
    field: "speed_limit_kb",
    label: "Speed limit (kB/s)",
    icon: <Gauge className="w-4 h-4" />,
    hint: "0 is unlimited.",
    placeholder: "Unlimited",
  },
  {
    field: "max_parallel",
    label: "Parallel downloads",
    icon: <Zap className="w-4 h-4" />,
    hint: "How many downloads run at once.",
  },
  {
    field: "connections",
    label: "Connections per download",
    icon: <Zap className="w-4 h-4" />,
    hint: "Segments per file. More beats CDN throttling.",
    placeholder: "8",
  },
];

/**
 * Download limits (PATCH /settings): global speed cap, parallel-download count
 * and connections-per-download (segments), plus the file-name conflict policy.
 * Numeric fields keep a local draft, show an inline range message while the
 * typed value is out of bounds, and commit (clamped) on blur.
 */
export function DownloadSettings() {
  const { data: settings, isLoading, isError } = useSettings();
  const { patch, savedField, errorFor } = useSettingsPatch();

  const [nums, setNums] = useState<Record<DownloadNumField, string>>({
    speed_limit_kb: "0",
    max_parallel: "3",
    connections: "8",
  });

  useEffect(() => {
    if (!settings) return;
    setNums({
      speed_limit_kb: String(settings.speed_limit_kb ?? 0),
      max_parallel: String(settings.max_parallel ?? 3),
      connections: String(settings.connections ?? 8),
    });
  }, [settings]);

  const setNum = (field: DownloadNumField, value: string) =>
    setNums((prev) => ({ ...prev, [field]: value }));

  const commitNum = (field: DownloadNumField) => {
    const { fallback } = NUM_BOUNDS[field];
    const v = clampNum(nums[field], field);
    setNum(field, String(v));
    if (v === (settings?.[field] ?? fallback)) return;
    patch(field, { [field]: v } as Partial<SettingReq>, {
      onError: () => setNum(field, String(settings?.[field] ?? fallback)),
    });
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
            {FIELDS.map(({ field, label, icon, hint, placeholder }) => {
              const { min, max } = NUM_BOUNDS[field];
              return (
                <SettingInput
                  key={field}
                  id={`download-${field}`}
                  label={label}
                  icon={icon}
                  type="number"
                  min={min}
                  max={Number.isFinite(max) ? max : undefined}
                  placeholder={placeholder}
                  hint={hint}
                  value={nums[field]}
                  onChange={(e) => setNum(field, e.target.value)}
                  onBlur={() => commitNum(field)}
                  saved={savedField === field}
                  error={rangeError(nums[field], field) ?? errorFor(field)}
                />
              );
            })}

            <SettingSelect
              id="download-file-conflict"
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
              error={errorFor("file_confilict")}
            />
          </>
        )}
      </CardContent>
    </Card>
  );
}

export default DownloadSettings;
