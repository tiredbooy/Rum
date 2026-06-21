// Shared, presentational setting controls used by the self-contained settings
// cards (General / Downloads / Post-download). They render the row + a "Saved"
// badge and delegate persistence to the caller. Toggles and selects persist
// immediately in their change handler (no fragile onBlur dependency); free-text
// / numeric inputs commit on blur. This mirrors the AppearanceSettings pattern
// so every card saves live and reliably.
import { useCallback, useRef, useState } from "react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

/** Small "Saved ✓" pill shown briefly after a successful persist. */
export function SavedBadge({ className }: { className?: string }) {
  return (
    <Badge
      variant="outline"
      className={cn("text-green-600 border-green-600", className)}
    >
      Saved
    </Badge>
  );
}

/**
 * Per-card "which field just saved" flash state. Returns the currently-flashing
 * field name and a `flash(field)` callback that clears itself after 2s. The
 * timeout is tracked in a ref so rapid edits don't leak overlapping timers.
 */
export function useSavedFlash(): [string | null, (field: string) => void] {
  const [savedField, setSavedField] = useState<string | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const flash = useCallback((field: string) => {
    setSavedField(field);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(
      () => setSavedField((f) => (f === field ? null : f)),
      2000,
    );
  }, []);

  return [savedField, flash];
}

export function SettingToggle({
  label,
  help,
  checked,
  onCheckedChange,
  saved,
  id,
}: {
  label: string;
  help?: string;
  checked?: boolean;
  onCheckedChange: (v: boolean) => void;
  saved: boolean;
  id?: string;
}) {
  return (
    <div className="flex items-start justify-between gap-4">
      <div className="space-y-0.5">
        <Label htmlFor={id} className="text-sm">
          {label}
        </Label>
        {help && <p className="text-xs text-muted-foreground">{help}</p>}
      </div>
      <div className="flex items-center gap-2 shrink-0">
        {saved && <SavedBadge />}
        <Switch
          id={id}
          checked={checked ?? false}
          onCheckedChange={onCheckedChange}
        />
      </div>
    </div>
  );
}

export function SettingInput({
  label,
  icon,
  saved,
  ...props
}: React.InputHTMLAttributes<HTMLInputElement> & {
  label: string;
  icon?: React.ReactNode;
  saved: boolean;
}) {
  return (
    <div className="space-y-1.5">
      <Label className="flex items-center gap-2 text-sm">
        {icon}
        {label}
      </Label>
      <div className="relative">
        <Input {...props} className={cn("pr-16", props.className)} />
        {saved && (
          <SavedBadge className="absolute right-2 top-1/2 -translate-y-1/2" />
        )}
      </div>
    </div>
  );
}

export function SettingSelect({
  label,
  icon,
  value,
  options,
  onValueChange,
  saved,
}: {
  label: string;
  icon?: React.ReactNode;
  value?: string;
  options: { value: string; label: string }[];
  onValueChange: (value: string) => void;
  saved: boolean;
}) {
  return (
    <div className="space-y-1.5">
      <Label className="flex items-center gap-2 text-sm">
        {icon}
        {label}
      </Label>
      <div className="flex items-center gap-2">
        <Select value={value ?? ""} onValueChange={onValueChange}>
          <SelectTrigger className="w-full">
            <SelectValue placeholder="Select..." />
          </SelectTrigger>
          <SelectContent>
            {options.map((opt) => (
              <SelectItem key={opt.value} value={opt.value}>
                {opt.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {saved && <SavedBadge className="shrink-0" />}
      </div>
    </div>
  );
}
