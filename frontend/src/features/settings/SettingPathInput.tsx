import { useId } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { FolderOpen } from "lucide-react";
import { cn } from "@/lib/utils";
import { FieldError, SavedBadge } from "./controls";

interface Props {
  label: string;
  value: string;
  placeholder?: string;
  hint?: string;
  error?: string;
  saved: boolean;
  /** Hidden when the desktop folder picker is unavailable (browser/dev). */
  canBrowse: boolean;
  onChange: (value: string) => void;
  /** Persist the current text — wired to blur. */
  onCommit: (value: string) => void;
  onBrowse: () => void;
}

/**
 * Folder path field with the native "Browse" picker beside it. Shared by the
 * download and temp directory settings, which had two near-identical copies of
 * this markup and of the picker plumbing.
 *
 * The text input is always editable: it is the fallback whenever the native
 * picker is missing (browser/dev) or fails, so the setting is never a dead end.
 */
export function SettingPathInput({
  label,
  value,
  placeholder,
  hint,
  error,
  saved,
  canBrowse,
  onChange,
  onCommit,
  onBrowse,
}: Props) {
  const id = useId();
  const errorId = `${id}-error`;
  const hintId = `${id}-hint`;

  return (
    <div className="space-y-1.5">
      <Label htmlFor={id} className="flex items-center gap-2 text-sm">
        <FolderOpen className="w-4 h-4" />
        {label}
      </Label>
      <div className="flex items-center gap-2">
        <div className="relative flex-1">
          <Input
            id={id}
            className={cn("pr-16")}
            placeholder={placeholder}
            value={value}
            aria-invalid={!!error}
            aria-describedby={error ? errorId : hint ? hintId : undefined}
            onChange={(e) => onChange(e.target.value)}
            onBlur={() => onCommit(value)}
          />
          {saved && (
            <SavedBadge className="absolute right-2 top-1/2 -translate-y-1/2" />
          )}
        </div>
        {canBrowse && (
          <Button type="button" variant="outline" size="sm" onClick={onBrowse}>
            <FolderOpen className="w-4 h-4" /> Browse
          </Button>
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

export default SettingPathInput;
