import { useState } from "react";
import type { CategoryRule } from "@/_lib/types/setting-types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { FolderOpen, Trash2, X } from "lucide-react";

interface Props {
  index: number;
  rule: CategoryRule;
  /** Hidden when the desktop folder picker is unavailable (browser/dev). */
  canBrowse: boolean;
  onNameChange: (name: string) => void;
  onDestChange: (dir: string) => void;
  onAddExt: (ext: string) => void;
  onRemoveExt: (ext: string) => void;
  onRemove: () => void;
  onBrowse: () => void;
}

/** One auto-organize rule: name, destination folder and extension chips. */
export function CategoryRuleRow({
  index,
  rule,
  canBrowse,
  onNameChange,
  onDestChange,
  onAddExt,
  onRemoveExt,
  onRemove,
  onBrowse,
}: Props) {
  const [extInput, setExtInput] = useState("");

  const commitExt = () => {
    if (extInput.trim()) {
      onAddExt(extInput);
      setExtInput("");
    }
  };

  const handleExtKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    // Enter or comma commits the current token as a chip.
    if (e.key === "Enter" || e.key === ",") {
      e.preventDefault();
      commitExt();
    } else if (e.key === "Backspace" && !extInput && rule.extensions.length) {
      // Backspace on an empty input removes the last chip.
      onRemoveExt(rule.extensions[rule.extensions.length - 1]);
    }
  };

  return (
    <li className="rounded-md border border-border p-3 space-y-3">
      <div className="flex flex-wrap items-start gap-3">
        <div className="space-y-1 w-40">
          <Label
            htmlFor={`cat-name-${index}`}
            className="text-xs text-muted-foreground"
          >
            Name
          </Label>
          <Input
            id={`cat-name-${index}`}
            value={rule.name}
            onChange={(e) => onNameChange(e.target.value)}
            placeholder="Video"
            aria-invalid={!rule.name.trim()}
          />
        </div>

        <div className="space-y-1 flex-1 min-w-[12rem]">
          <Label
            htmlFor={`cat-dest-${index}`}
            className="text-xs text-muted-foreground"
          >
            Destination folder
          </Label>
          <div className="flex gap-2">
            <Input
              id={`cat-dest-${index}`}
              value={rule.dest_dir}
              onChange={(e) => onDestChange(e.target.value)}
              placeholder="Videos (or an absolute path)"
              className="flex-1"
            />
            {canBrowse && (
              <Button
                type="button"
                variant="outline"
                size="icon"
                onClick={onBrowse}
                aria-label={`Browse for destination folder of rule ${index + 1}`}
                title="Browse"
                className="shrink-0"
              >
                <FolderOpen className="w-4 h-4" />
              </Button>
            )}
          </div>
        </div>

        <Button
          type="button"
          variant="ghost"
          size="icon"
          onClick={onRemove}
          aria-label={`Remove rule ${index + 1}`}
          title="Remove rule"
          className="text-destructive shrink-0 mt-5"
        >
          <Trash2 className="w-4 h-4" />
        </Button>
      </div>

      <div className="space-y-1.5">
        <Label
          htmlFor={`cat-ext-${index}`}
          className="text-xs text-muted-foreground"
        >
          Extensions
        </Label>
        <div className="flex flex-wrap items-center gap-1.5 rounded-md border border-input bg-transparent p-1.5">
          {rule.extensions.map((ext) => (
            <Badge
              key={ext}
              variant="secondary"
              className="gap-1 pr-1 font-mono text-[11px]"
            >
              {ext}
              <button
                type="button"
                onClick={() => onRemoveExt(ext)}
                aria-label={`Remove ${ext}`}
                className="inline-flex size-4 items-center justify-center rounded-sm cursor-pointer hover:bg-foreground/10 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
              >
                <X className="size-3" />
              </button>
            </Badge>
          ))}
          <input
            id={`cat-ext-${index}`}
            value={extInput}
            onChange={(e) => setExtInput(e.target.value)}
            onKeyDown={handleExtKeyDown}
            onBlur={commitExt}
            placeholder={
              rule.extensions.length ? "" : ".mp4, .mkv … (Enter to add)"
            }
            aria-label={`Add extension to rule ${index + 1}`}
            className="flex-1 min-w-[8rem] bg-transparent px-1 text-sm outline-none placeholder:text-muted-foreground"
          />
        </div>
        {rule.extensions.length === 0 && (
          <p role="alert" className="text-xs text-destructive">
            Add at least one extension.
          </p>
        )}
      </div>
    </li>
  );
}

export default CategoryRuleRow;
