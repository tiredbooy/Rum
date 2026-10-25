import { Plus, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { MAX_MIRRORS, mirrorProblem } from "@/_lib/mirrors";

interface MirrorFieldsProps {
  value: string[];
  onChange: (next: string[]) => void;
  idPrefix: string;
}

/** Rows of mirror links with an "Add mirror" button; blank rows are ignored on save. */
export function MirrorFields({ value, onChange, idPrefix }: MirrorFieldsProps) {
  const setRow = (i: number, next: string) =>
    onChange(value.map((row, j) => (j === i ? next : row)));

  return (
    <div className="space-y-2">
      {value.map((row, i) => {
        const problem = mirrorProblem(row);
        const errorId = `${idPrefix}-${i}-error`;
        return (
          <div key={i} className="space-y-1">
            <div className="flex gap-2">
              <Input
                id={`${idPrefix}-${i}`}
                type="url"
                autoComplete="off"
                placeholder="https://mirror.example.com/file.zip"
                aria-label={`Mirror ${i + 1}`}
                aria-invalid={problem ? true : undefined}
                aria-describedby={problem ? errorId : undefined}
                value={row}
                onChange={(e) => setRow(i, e.target.value)}
                className="flex-1 bg-background text-foreground"
              />
              <Button
                type="button"
                size="icon"
                variant="outline"
                className="shrink-0"
                aria-label={`Remove mirror ${i + 1}`}
                onClick={() => onChange(value.filter((_, j) => j !== i))}
              >
                <X className="h-4 w-4" aria-hidden />
              </Button>
            </div>
            {problem && (
              <p id={errorId} role="alert" className="text-xs text-destructive">
                {problem}
              </p>
            )}
          </div>
        );
      })}
      <Button
        type="button"
        variant="outline"
        size="sm"
        disabled={value.length >= MAX_MIRRORS}
        onClick={() => onChange([...value, ""])}
      >
        <Plus className="h-4 w-4" aria-hidden />
        Add mirror
      </Button>
    </div>
  );
}
