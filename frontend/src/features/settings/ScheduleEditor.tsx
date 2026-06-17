import { useEffect, useMemo, useState } from "react";
import {
  useSchedule,
  useUpdateSchedule,
} from "@/_lib/services/queries/schedule.queries";
import type { ScheduleSettings, SpeedRule } from "@/_lib/types/setting-types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import { CalendarClock, Gauge, Loader2, Plus, Trash2 } from "lucide-react";
import { cn } from "@/lib/utils";

const HOURS = Array.from({ length: 24 }, (_, h) => h);
const DAYS = [
  { value: 0, short: "S", label: "Sunday" },
  { value: 1, short: "M", label: "Monday" },
  { value: 2, short: "T", label: "Tuesday" },
  { value: 3, short: "W", label: "Wednesday" },
  { value: 4, short: "T", label: "Thursday" },
  { value: 5, short: "F", label: "Friday" },
  { value: 6, short: "S", label: "Saturday" },
];

function pad(h: number): string {
  return `${h.toString().padStart(2, "0")}:00`;
}

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

  const handleSave = () => updateMutation.mutate(draft);

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
            {/* Scheduled-start toggle */}
            <div className="flex items-start justify-between gap-4">
              <div className="space-y-0.5">
                <Label htmlFor="scheduled-start" className="text-sm">
                  Scheduled start
                </Label>
                <p className="text-xs text-muted-foreground">
                  Start downloads automatically when their scheduled time
                  arrives.
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
                <div className="rounded-md border border-dashed border-border px-4 py-6 text-center">
                  <p className="text-sm text-muted-foreground">
                    No speed-limit windows yet.
                  </p>
                  <p className="text-xs text-muted-foreground mt-1">
                    e.g. throttle to 500 kB/s during 09:00–18:00 on weekdays.
                  </p>
                </div>
              ) : (
                <ul className="space-y-3">
                  {draft.rules.map((rule, i) => (
                    <li
                      key={i}
                      className="rounded-md border border-border p-3 space-y-3"
                    >
                      <div className="flex flex-wrap items-end gap-3">
                        <div className="space-y-1">
                          <Label
                            htmlFor={`start-${i}`}
                            className="text-xs text-muted-foreground"
                          >
                            From
                          </Label>
                          <NativeSelect
                            id={`start-${i}`}
                            value={rule.start_hour}
                            onChange={(e) =>
                              setRule(i, { start_hour: Number(e.target.value) })
                            }
                            className="w-24"
                            aria-label={`Window ${i + 1} start hour`}
                          >
                            {HOURS.map((h) => (
                              <NativeSelectOption key={h} value={h}>
                                {pad(h)}
                              </NativeSelectOption>
                            ))}
                          </NativeSelect>
                        </div>

                        <div className="space-y-1">
                          <Label
                            htmlFor={`end-${i}`}
                            className="text-xs text-muted-foreground"
                          >
                            To
                          </Label>
                          <NativeSelect
                            id={`end-${i}`}
                            value={rule.end_hour}
                            onChange={(e) =>
                              setRule(i, { end_hour: Number(e.target.value) })
                            }
                            className="w-24"
                            aria-label={`Window ${i + 1} end hour`}
                          >
                            {HOURS.map((h) => (
                              <NativeSelectOption key={h} value={h}>
                                {pad(h)}
                              </NativeSelectOption>
                            ))}
                          </NativeSelect>
                        </div>

                        <div className="space-y-1 flex-1 min-w-[10rem]">
                          <Label
                            htmlFor={`limit-${i}`}
                            className="text-xs text-muted-foreground"
                          >
                            Limit (kB/s, 0 = unlimited)
                          </Label>
                          <Input
                            id={`limit-${i}`}
                            type="number"
                            min={0}
                            value={rule.limit_kbps}
                            onChange={(e) =>
                              setRule(i, {
                                limit_kbps:
                                  e.target.value === ""
                                    ? 0
                                    : Math.max(0, Number(e.target.value)),
                              })
                            }
                            placeholder="0"
                          />
                        </div>

                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          onClick={() => removeRule(i)}
                          aria-label={`Remove window ${i + 1}`}
                          title="Remove window"
                          className="text-destructive shrink-0"
                        >
                          <Trash2 className="w-4 h-4" />
                        </Button>
                      </div>

                      <div className="space-y-1.5">
                        <span className="text-xs text-muted-foreground">
                          Days (none = every day)
                        </span>
                        <div
                          className="flex flex-wrap gap-1.5"
                          role="group"
                          aria-label={`Window ${i + 1} days of week`}
                        >
                          {DAYS.map((d) => {
                            const active = (rule.days ?? []).includes(d.value);
                            return (
                              <button
                                key={d.value}
                                type="button"
                                onClick={() => toggleDay(i, d.value)}
                                aria-pressed={active}
                                aria-label={d.label}
                                title={d.label}
                                className={cn(
                                  "inline-flex size-8 items-center justify-center rounded-md border text-xs font-medium transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1",
                                  active
                                    ? "border-primary bg-primary text-primary-foreground"
                                    : "border-border bg-background text-muted-foreground hover:bg-accent hover:text-accent-foreground",
                                )}
                              >
                                {d.short}
                              </button>
                            );
                          })}
                        </div>
                      </div>

                      {rule.start_hour === rule.end_hour && (
                        <p className="text-xs text-amber-500">
                          Start and end are the same hour — this window covers
                          the full day.
                        </p>
                      )}
                    </li>
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
                onClick={handleSave}
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
