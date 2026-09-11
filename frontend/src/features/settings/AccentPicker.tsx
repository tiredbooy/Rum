import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Palette, RotateCcw } from "lucide-react";
import { cn } from "@/lib/utils";
import { isValidAccent, normalizeHex } from "@/_lib/theme";
import { SavedBadge } from "./controls";

/** A few tasteful accent presets users can one-click apply. */
const PRESETS: { name: string; hex: string }[] = [
  { name: "Indigo", hex: "#6366f1" },
  { name: "Violet", hex: "#8b5cf6" },
  { name: "Sky", hex: "#0ea5e9" },
  { name: "Emerald", hex: "#10b981" },
  { name: "Amber", hex: "#f59e0b" },
  { name: "Rose", hex: "#f43f5e" },
];

/** Swatch shown when the accent is the app default. */
const DEFAULT_SWATCH = "#6366f1";

/**
 * How long to wait after the last change of the colour input before saving.
 * `<input type="color">` streams values while the OS picker is open and React
 * exposes no "picker closed" event, so waiting for blur alone meant a colour
 * could sit on screen looking applied while nothing had been persisted.
 */
const COMMIT_DEBOUNCE_MS = 500;

interface Props {
  /** Persisted accent: "" is the app default. */
  value: string;
  saved: boolean;
  error?: string;
  onCommit: (accent: string) => void;
}

export function AccentPicker({ value, saved, error, onCommit }: Props) {
  const [accent, setAccent] = useState(value);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  // The last colour handed to onCommit. The debounce and the blur handler can
  // both fire for the same colour before the save lands and `value` catches up,
  // which would send the identical PATCH twice.
  const committed = useRef(value);

  useEffect(() => {
    setAccent(value);
    committed.current = value;
  }, [value]);
  useEffect(() => () => { if (timer.current) clearTimeout(timer.current); }, []);

  const commit = (raw: string) => {
    if (timer.current) clearTimeout(timer.current);
    const next = raw === "" ? "" : isValidAccent(raw) ? normalizeHex(raw) : "";
    setAccent(next);
    if (next === value || next === committed.current) return;
    committed.current = next;
    onCommit(next);
  };

  const previewThenCommit = (raw: string) => {
    setAccent(raw);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => commit(raw), COMMIT_DEBOUNCE_MS);
  };

  const swatchValue = isValidAccent(accent) ? normalizeHex(accent) : DEFAULT_SWATCH;

  return (
    <div className="space-y-2.5">
      <div className="flex items-center justify-between">
        <Label htmlFor="appearance-accent" className="flex items-center gap-2 text-sm">
          <Palette className="w-4 h-4" /> Accent color
        </Label>
        {saved && <SavedBadge />}
      </div>

      <div className="flex items-center gap-2">
        <span
          className="relative inline-flex h-9 w-9 overflow-hidden rounded-md border shadow-xs"
          style={{ backgroundColor: accent === "" ? "var(--primary)" : undefined }}
        >
          <input
            id="appearance-accent"
            aria-label="Pick accent color"
            type="color"
            value={swatchValue}
            className="rum-swatch h-full w-full rounded-md"
            onChange={(e) => previewThenCommit(e.target.value)}
            onBlur={() => commit(accent)}
          />
        </span>
        <code className="rounded bg-muted px-2 py-1 text-xs text-muted-foreground">
          {accent === "" ? "Default" : normalizeHex(accent)}
        </code>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="ml-auto"
          disabled={accent === ""}
          onClick={() => commit("")}
        >
          <RotateCcw className="w-4 h-4" /> Reset
        </Button>
      </div>

      <div className="flex flex-wrap gap-2 pt-0.5">
        {PRESETS.map((p) => {
          const active = normalizeHex(accent) === p.hex;
          return (
            <button
              key={p.hex}
              type="button"
              aria-label={`Accent ${p.name}`}
              aria-pressed={active}
              title={p.name}
              onClick={() => commit(p.hex)}
              className={cn(
                "h-6 w-6 rounded-full border transition-transform hover:scale-110 focus-visible:scale-110",
                active && "ring-2 ring-ring ring-offset-2 ring-offset-background",
              )}
              style={{ backgroundColor: p.hex }}
            />
          );
        })}
      </div>
      {error ? (
        <p role="alert" className="text-xs text-destructive">
          {error}
        </p>
      ) : (
        <p className="text-xs text-muted-foreground">
          Tints buttons, links and highlights.
        </p>
      )}
    </div>
  );
}

export default AccentPicker;
