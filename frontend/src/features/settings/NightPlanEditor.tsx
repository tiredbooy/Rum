import { useEffect, useMemo, useState } from "react";
import { Loader2, Moon } from "lucide-react";
import { ApiError } from "@/_lib/services/api/api";
import {
  useNightPlan,
  useUpdateNightPlan,
} from "@/_lib/services/queries/night-plan.queries";
import {
  DEFAULT_NIGHT_PLAN,
  MAX_NIGHT_CAP_GB,
  crossesMidnight,
  validateNightPlan,
} from "@/_lib/night-plan";
import type { NightPlan } from "@/_lib/types/setting-types";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { TimeSelect } from "./TimeSelect";

/** Keep only the saved plan fields (drops the live state the GET adds). */
function toPlan(s: NightPlan): NightPlan {
  return {
    enabled: !!s.enabled,
    start: s.start,
    end: s.end,
    start_at_open: !!s.start_at_open,
    pause_at_close: !!s.pause_at_close,
    cap_gb: s.cap_gb ?? 0,
  };
}

interface ToggleProps {
  id: string;
  label: string;
  checked: boolean;
  onChange: (v: boolean) => void;
  disabled?: boolean;
}

function ToggleRow({ id, label, checked, onChange, disabled }: ToggleProps) {
  return (
    <div className="flex items-center justify-between gap-4">
      <Label htmlFor={id} className="text-sm">
        {label}
      </Label>
      <Switch
        id={id}
        checked={checked}
        onCheckedChange={onChange}
        disabled={disabled}
      />
    </div>
  );
}

/**
 * Night plan (PUT /settings/night-plan): a daily window in which downloads
 * assigned to the night queue run. Edits are a draft until "Save plan".
 */
export function NightPlanEditor() {
  const { data, isLoading, isError, refetch } = useNightPlan();
  const update = useUpdateNightPlan();
  const [draft, setDraft] = useState<NightPlan>(DEFAULT_NIGHT_PLAN);

  useEffect(() => {
    if (data) setDraft(toPlan(data));
  }, [data]);

  const dirty = useMemo(
    () => !!data && JSON.stringify(toPlan(data)) !== JSON.stringify(draft),
    [data, draft],
  );
  const problems = validateNightPlan(draft);
  const serverFields =
    update.error instanceof ApiError ? (update.error.fields ?? {}) : {};
  const err = (k: string) => problems[k] ?? serverFields[k];
  const patch = (p: Partial<NightPlan>) => setDraft((d) => ({ ...d, ...p }));

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-lg">
          <Moon className="w-5 h-5" /> Night plan
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-5">
        {isLoading ? (
          <p className="text-muted-foreground animate-pulse text-sm">
            Loading night plan…
          </p>
        ) : isError ? (
          <div className="flex items-center justify-between gap-3">
            <p className="text-destructive text-sm">
              Could not load the night plan.
            </p>
            <Button type="button" variant="outline" size="sm" onClick={() => refetch()}>
              Retry
            </Button>
          </div>
        ) : (
          <>
            <ToggleRow
              id="night-enabled"
              label="Run night downloads in the window"
              checked={draft.enabled}
              onChange={(v) => patch({ enabled: v })}
            />

            <div className="flex flex-wrap items-start gap-4">
              <TimeSelect
                id="night-start"
                label="From"
                value={draft.start}
                onChange={(start) => patch({ start })}
                error={err("start")}
              />
              <TimeSelect
                id="night-end"
                label="To"
                value={draft.end}
                onChange={(end) => patch({ end })}
                error={err("end")}
              />
              <div className="space-y-1 min-w-[8rem]">
                <Label htmlFor="night-cap" className="text-xs text-muted-foreground">
                  Daily cap (GB, 0 = none)
                </Label>
                <Input
                  id="night-cap"
                  type="number"
                  min={0}
                  max={MAX_NIGHT_CAP_GB}
                  step="any"
                  value={Number.isFinite(draft.cap_gb) ? draft.cap_gb : ""}
                  onChange={(e) =>
                    patch({
                      cap_gb: e.target.value === "" ? 0 : Number(e.target.value),
                    })
                  }
                  aria-invalid={!!err("cap_gb")}
                  className="w-32"
                />
                {err("cap_gb") && (
                  <p className="text-xs text-destructive" role="alert">
                    {err("cap_gb")}
                  </p>
                )}
              </div>
            </div>
            {err("window") && (
              <p className="text-xs text-destructive" role="alert">
                {err("window")}
              </p>
            )}
            {crossesMidnight(draft) && (
              <p className="text-xs text-muted-foreground">
                Runs overnight, ending the next day.
              </p>
            )}

            <div className="space-y-3">
              <ToggleRow
                id="night-start-open"
                label="Start at window open"
                checked={draft.start_at_open}
                onChange={(v) => patch({ start_at_open: v })}
              />
              <ToggleRow
                id="night-pause-close"
                label="Pause at window end"
                checked={draft.pause_at_close}
                onChange={(v) => patch({ pause_at_close: v })}
              />
            </div>

            {data?.enabled && (
              <p className="text-xs text-muted-foreground" aria-live="polite">
                {data.cap_reached
                  ? "Cap reached. Resumes next window."
                  : data.active
                    ? "Window open now."
                    : `Next window starts ${data.start}.`}
              </p>
            )}

            <div className="flex items-center justify-end gap-2 pt-1">
              {dirty && (
                <span className="text-xs text-muted-foreground">
                  Unsaved changes
                </span>
              )}
              <Button
                type="button"
                onClick={() => update.mutate(draft)}
                disabled={
                  !dirty ||
                  update.isPending ||
                  Object.keys(problems).length > 0
                }
                className="gap-2"
              >
                {update.isPending && <Loader2 className="w-4 h-4 animate-spin" />}
                Save plan
              </Button>
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}

export default NightPlanEditor;
