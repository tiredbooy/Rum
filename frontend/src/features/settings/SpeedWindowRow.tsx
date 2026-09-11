import type { SpeedRule } from "@/_lib/types/setting-types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import { Trash2 } from "lucide-react";
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

interface Props {
  index: number;
  rule: SpeedRule;
  onChange: (patch: Partial<SpeedRule>) => void;
  onToggleDay: (day: number) => void;
  onRemove: () => void;
}

/** One bandwidth window: hour range, cap and the days it applies on. */
export function SpeedWindowRow({
  index,
  rule,
  onChange,
  onToggleDay,
  onRemove,
}: Props) {
  return (
    <li className="rounded-md border border-border p-3 space-y-3">
      <div className="flex flex-wrap items-end gap-3">
        <div className="space-y-1">
          <Label
            htmlFor={`start-${index}`}
            className="text-xs text-muted-foreground"
          >
            From
          </Label>
          <NativeSelect
            id={`start-${index}`}
            value={rule.start_hour}
            onChange={(e) => onChange({ start_hour: Number(e.target.value) })}
            className="w-24"
            aria-label={`Window ${index + 1} start hour`}
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
            htmlFor={`end-${index}`}
            className="text-xs text-muted-foreground"
          >
            To
          </Label>
          <NativeSelect
            id={`end-${index}`}
            value={rule.end_hour}
            onChange={(e) => onChange({ end_hour: Number(e.target.value) })}
            className="w-24"
            aria-label={`Window ${index + 1} end hour`}
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
            htmlFor={`limit-${index}`}
            className="text-xs text-muted-foreground"
          >
            Limit (kB/s, 0 = unlimited)
          </Label>
          <Input
            id={`limit-${index}`}
            type="number"
            min={0}
            value={rule.limit_kbps}
            onChange={(e) =>
              onChange({
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
          onClick={onRemove}
          aria-label={`Remove window ${index + 1}`}
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
          aria-label={`Window ${index + 1} days of week`}
        >
          {DAYS.map((d) => {
            const active = (rule.days ?? []).includes(d.value);
            return (
              <button
                key={d.value}
                type="button"
                onClick={() => onToggleDay(d.value)}
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
          Same start and end covers the whole day.
        </p>
      )}
    </li>
  );
}

export default SpeedWindowRow;
