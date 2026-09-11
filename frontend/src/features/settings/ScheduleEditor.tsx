import { useEffect, useMemo, useState } from "react";
import {
  useSchedule,
  useUpdateSchedule,
} from "@/_lib/services/queries/schedule.queries";
import type { ScheduleSettings, SpeedRule } from "@/_lib/types/setting-types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { CalendarClock, Gauge, Loader2, Plus } from "lucide-react";
import { SpeedWindowRow } from "./SpeedWindowRow";

const emptyRule: SpeedRule = {
  start_hour: 9,
  end_hour: 18,
  limit_kbps: 500,
  days: [],
};

function normalize(s: ScheduleSettings): ScheduleSettings {
  return {
    scheduled_start_enabled: !!s.scheduled_start_enabled,
    rules: (s.rules ?? []).map((r) => ({
      start_hour: r.start_hour ?? 0,
      end_hour: r.end_hour ?? 0,
      limit_kbps: r.limit_kbps ?? 0,
      days: r.days ?? [],
    })),
  };
}

/**
 * Bandwidth schedule + scheduled-start toggle (PUT /settings/schedule). Edits
 * are a draft until "Save schedule"; saving applies the newly-active window to
 * the running engine immediately rather than at the next controller tick.
 */
export function ScheduleEditor() {
  const { data, isLoading, isError } = useSchedule();
  const updateMutation = useUpdateSchedule();

  const [draft, setDraft] = useState<ScheduleSettings>({
    scheduled_start_enabled: false,
    rules: [],
  });

  useEffect(() => {
    if (data) setDraft(normalize(data));
  }, [data]);

  const dirty = useMemo(() => {
    if (!data) return false;
    return JSON.stringify(normalize(data)) !== JSON.stringify(draft);
  }, [data, draft]);

  const setRule = (i: number, patch: Partial<SpeedRule>) =>
    setDraft((d) => ({
      ...d,
      rules: d.rules.map((r, idx) => (idx === i ? { ...r, ...patch } : r)),
    }));

  const addRule = () =>
    setDraft((d) => ({ ...d, rules: [...d.rules, { ...emptyRule }] }));

  const removeRule = (i: number) =>
    setDraft((d) => ({ ...d, rules: d.rules.filter((_, idx) => idx !== i) }));

  const toggleDay = (i: number, day: number) =>
    setDraft((d) => ({
      ...d,
      rules: d.rules.map((r, idx) => {
        if (idx !== i) return r;
        const days = r.days ?? [];
        return {
          ...r,
          days: days.includes(day)
            ? days.filter((x) => x !== day)
            : [...days, day].sort((a, b) => a - b),
        };
      }),
    }));

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-lg">
          <CalendarClock className="w-5 h-5" /> Bandwidth schedule
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-5">
        {isLoading ? (
          <p className="text-muted-foreground animate-pulse text-sm">
            Loading schedule…
          </p>
        ) : isError ? (
          <p className="text-destructive text-sm">
            Could not load the bandwidth schedule.
          </p>
        ) : (
          <>
            <div className="flex items-start justify-between gap-4">
              <div className="space-y-0.5">
                <Label htmlFor="scheduled-start" className="text-sm">
                  Scheduled start
                </Label>
                <p className="text-xs text-muted-foreground">
                  Off: scheduled downloads wait for you to start them.
                </p>
              </div>
              <Switch
                id="scheduled-start"
                checked={draft.scheduled_start_enabled}
                onCheckedChange={(v) =>
                  setDraft((d) => ({ ...d, scheduled_start_enabled: v }))
                }
              />
            </div>

            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <Label className="flex items-center gap-2 text-sm">
                  <Gauge className="w-4 h-4" /> Speed-limit windows
                </Label>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={addRule}
                  className="gap-1"
                >
                  <Plus className="w-4 h-4" /> Add window
                </Button>
              </div>

              {draft.rules.length === 0 ? (
                <div className="rounded-md border border-dashed border-border px-4 py-6 text-center space-y-2">
                  <p className="text-sm text-muted-foreground">
                    No speed limits by time of day.
                  </p>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={addRule}
                  >
                    Add window
                  </Button>
                </div>
              ) : (
                <ul className="space-y-3">
                  {draft.rules.map((rule, i) => (
                    <SpeedWindowRow
                      key={i}
                      index={i}
                      rule={rule}
                      onChange={(patch) => setRule(i, patch)}
                      onToggleDay={(day) => toggleDay(i, day)}
                      onRemove={() => removeRule(i)}
                    />
                  ))}
                </ul>
              )}
            </div>

            <div className="flex items-center justify-end gap-2 pt-1">
              {dirty && (
                <span className="text-xs text-muted-foreground">
                  Unsaved changes
                </span>
              )}
              <Button
                type="button"
                onClick={() => updateMutation.mutate(draft)}
                disabled={!dirty || updateMutation.isPending}
                className="gap-2"
              >
                {updateMutation.isPending && (
                  <Loader2 className="w-4 h-4 animate-spin" />
                )}
                Save schedule
              </Button>
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}

export default ScheduleEditor;
