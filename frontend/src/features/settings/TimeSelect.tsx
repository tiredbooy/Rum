import { useMemo } from "react";
import { Label } from "@/components/ui/label";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import { clockOptions } from "@/_lib/night-plan";

interface Props {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  error?: string;
}

/** Labelled HH:MM picker (15-minute steps) built on the repo's NativeSelect. */
export function TimeSelect({ id, label, value, onChange, error }: Props) {
  const options = useMemo(() => clockOptions(value), [value]);
  return (
    <div className="space-y-1">
      <Label htmlFor={id} className="text-xs text-muted-foreground">
        {label}
      </Label>
      <NativeSelect
        id={id}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="w-28"
        aria-invalid={!!error}
        aria-describedby={error ? `${id}-error` : undefined}
      >
        {options.map((t) => (
          <NativeSelectOption key={t} value={t}>
            {t}
          </NativeSelectOption>
        ))}
      </NativeSelect>
      {error && (
        <p id={`${id}-error`} className="text-xs text-destructive" role="alert">
          {error}
        </p>
      )}
    </div>
  );
}
