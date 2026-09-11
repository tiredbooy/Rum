// Shared, presentational setting controls used by the self-contained settings
// cards. They render the row, the "Saved" pill and any inline error, and
// delegate persistence to the caller (see useSettingsPatch). Toggles and selects
// persist immediately in their change handler; free-text / numeric inputs commit
// on blur.
import { useId } from "react";
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

/** Small "Saved" pill shown briefly after a successful persist. */
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

/** Inline field error. Rendered under the control and wired via aria-describedby. */
export function FieldError({ id, children }: { id: string; children: string }) {
  return (
    <p id={id} role="alert" className="text-xs text-destructive">
      {children}
    </p>
  );
}

export function SettingToggle({
  label,
  help,
  checked,
  onCheckedChange,
  saved,
  error,
  id,
  disabled,
  icon,
}: {
  label: string;
  help?: string;
  checked?: boolean;
  onCheckedChange: (v: boolean) => void;
  saved: boolean;
  error?: string;
  id?: string;
  disabled?: boolean;
  icon?: React.ReactNode;
}) {
  const generated = useId();
  const switchId = id ?? generated;
  const errorId = `${switchId}-error`;

  return (
    <div className="space-y-1">
      <div className="flex items-start justify-between gap-4">
        <div className="space-y-0.5">
          <Label
            htmlFor={switchId}
            className={cn("flex items-center gap-2 text-sm", disabled && "opacity-60")}
          >
            {icon}
            {label}
          </Label>
          {help && <p className="text-xs text-muted-foreground">{help}</p>}
        </div>
        <div className="flex items-center gap-2 shrink-0">
          {saved && <SavedBadge />}
          <Switch
            id={switchId}
            checked={checked ?? false}
            onCheckedChange={onCheckedChange}
            disabled={disabled}
            aria-describedby={error ? errorId : undefined}
          />
        </div>
      </div>
      {error && <FieldError id={errorId}>{error}</FieldError>}
    </div>
  );
}

export function SettingInput({
  label,
  icon,
  saved,
  error,
  hint,
  id,
  className,
  ...props
}: React.InputHTMLAttributes<HTMLInputElement> & {
  label: string;
  icon?: React.ReactNode;
  saved: boolean;
  error?: string;
  hint?: string;
}) {
  const generated = useId();
  const inputId = id ?? generated;
  const errorId = `${inputId}-error`;
  const hintId = `${inputId}-hint`;

  return (
    <div className="space-y-1.5">
      <Label htmlFor={inputId} className="flex items-center gap-2 text-sm">
        {icon}
        {label}
      </Label>
      <div className="relative">
        <Input
          {...props}
          id={inputId}
          aria-invalid={!!error}
          aria-describedby={error ? errorId : hint ? hintId : undefined}
          className={cn("pr-16", className)}
        />
        {saved && (
          <SavedBadge className="absolute right-2 top-1/2 -translate-y-1/2" />
        )}
      </div>
      {error ? (
        <FieldError id={errorId}>{error}</FieldError>
      ) : (
        hint && (
          <p id={hintId} className="text-xs text-muted-foreground">
            {hint}
          </p>
        )
      )}
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
  error,
  hint,
  id,
}: {
  label: string;
  icon?: React.ReactNode;
  value?: string;
  options: { value: string; label: string }[];
  onValueChange: (value: string) => void;
  saved: boolean;
  error?: string;
  hint?: string;
  id?: string;
}) {
  const generated = useId();
  const triggerId = id ?? generated;
  const errorId = `${triggerId}-error`;
  const hintId = `${triggerId}-hint`;

  return (
    <div className="space-y-1.5">
      <Label htmlFor={triggerId} className="flex items-center gap-2 text-sm">
        {icon}
        {label}
      </Label>
      <div className="flex items-center gap-2">
        <Select value={value ?? ""} onValueChange={onValueChange}>
          <SelectTrigger
            id={triggerId}
            className="w-full"
            aria-invalid={!!error}
            aria-describedby={error ? errorId : hint ? hintId : undefined}
          >
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
      {error ? (
        <FieldError id={errorId}>{error}</FieldError>
      ) : (
        hint && (
          <p id={hintId} className="text-xs text-muted-foreground">
            {hint}
          </p>
        )
      )}
    </div>
  );
}
