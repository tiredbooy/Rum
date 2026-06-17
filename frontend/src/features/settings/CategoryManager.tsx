import { useEffect, useMemo, useRef, useState } from "react";
import {
  useCategories,
  useUpdateCategories,
} from "@/_lib/services/queries/categories.queries";
import type {
  CategoryRule,
  CategorySettings,
} from "@/_lib/types/setting-types";
import { chooseDir, hasChooseDir } from "@/_lib/wails";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Badge } from "@/components/ui/badge";
import {
  FolderOpen,
  FolderTree,
  Loader2,
  Plus,
  RotateCcw,
  Trash2,
  X,
} from "lucide-react";

/** Default rule set offered via "Reset to defaults". */
const DEFAULT_RULES: CategoryRule[] = [
  {
    name: "Video",
    extensions: [".mp4", ".mkv", ".avi", ".mov", ".webm"],
    dest_dir: "Videos",
  },
  {
    name: "Audio",
    extensions: [".mp3", ".flac", ".wav", ".aac", ".ogg"],
    dest_dir: "Music",
  },
  {
    name: "Documents",
    extensions: [".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".txt"],
    dest_dir: "Documents",
  },
  {
    name: "Archives",
    extensions: [".zip", ".rar", ".7z", ".tar", ".gz"],
    dest_dir: "Archives",
  },
  {
    name: "Images",
    extensions: [".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg"],
    dest_dir: "Images",
  },
];

/** Normalize a typed extension token to lower-case, dot-prefixed form. */
function normalizeExt(raw: string): string {
  const t = raw.trim().toLowerCase().replace(/^\.+/, "");
  return t ? `.${t}` : "";
}

const emptyRule: CategoryRule = { name: "", extensions: [], dest_dir: "" };

function normalize(s: CategorySettings): CategorySettings {
  return {
    enabled: !!s.enabled,
    rules: (s.rules ?? []).map((r) => ({
      name: r.name ?? "",
      extensions: r.extensions ?? [],
      dest_dir: r.dest_dir ?? "",
    })),
  };
}

export function CategoryManager() {
  const { data, isLoading, isError } = useCategories();
  const updateMutation = useUpdateCategories();

  const [draft, setDraft] = useState<CategorySettings>({
    enabled: false,
    rules: [],
  });

  useEffect(() => {
    if (data) setDraft(normalize(data));
  }, [data]);

  const dirty = useMemo(() => {
    if (!data) return false;
    return JSON.stringify(normalize(data)) !== JSON.stringify(draft);
  }, [data, draft]);

  // A rule is valid when it has a name and at least one extension.
  const invalid = draft.rules.some(
    (r) => !r.name.trim() || r.extensions.length === 0,
  );

  const setRule = (i: number, patch: Partial<CategoryRule>) =>
    setDraft((d) => ({
      ...d,
      rules: d.rules.map((r, idx) => (idx === i ? { ...r, ...patch } : r)),
    }));

  const addRule = () =>
    setDraft((d) => ({ ...d, rules: [...d.rules, { ...emptyRule }] }));

  const removeRule = (i: number) =>
    setDraft((d) => ({ ...d, rules: d.rules.filter((_, idx) => idx !== i) }));

  const addExt = (i: number, raw: string) => {
    const ext = normalizeExt(raw);
    if (!ext) return;
    setDraft((d) => ({
      ...d,
      rules: d.rules.map((r, idx) =>
        idx === i && !r.extensions.includes(ext)
          ? { ...r, extensions: [...r.extensions, ext] }
          : r,
      ),
    }));
  };

  const removeExt = (i: number, ext: string) =>
    setRule(i, {
      extensions: draft.rules[i].extensions.filter((e) => e !== ext),
    });

  const handleBrowse = async (i: number) => {
    const dir = await chooseDir();
    if (dir) setRule(i, { dest_dir: dir });
  };

  const handleSave = () => {
    if (invalid) return;
    updateMutation.mutate(draft);
  };

  const resetToDefaults = () =>
    setDraft((d) => ({ ...d, rules: DEFAULT_RULES.map((r) => ({ ...r })) }));

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-lg">
          <FolderTree className="w-5 h-5" /> Auto-organize by category
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-5">
        {isLoading ? (
          <p className="text-muted-foreground animate-pulse text-sm">
            Loading categories…
          </p>
        ) : isError ? (
          <p className="text-destructive text-sm">
            Could not load category rules.
          </p>
        ) : (
          <>
            <div className="flex items-start justify-between gap-4">
              <div className="space-y-0.5">
                <Label htmlFor="categories-enabled" className="text-sm">
                  Enable auto-organize
                </Label>
                <p className="text-xs text-muted-foreground">
                  Move finished downloads into a folder based on their file
                  type.
                </p>
              </div>
              <Switch
                id="categories-enabled"
                checked={draft.enabled}
                onCheckedChange={(v) =>
                  setDraft((d) => ({ ...d, enabled: v }))
                }
              />
            </div>

            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <Label className="text-sm">Rules</Label>
                <div className="flex gap-2">
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    onClick={resetToDefaults}
                    className="gap-1"
                  >
                    <RotateCcw className="w-4 h-4" /> Reset to defaults
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={addRule}
                    className="gap-1"
                  >
                    <Plus className="w-4 h-4" /> Add rule
                  </Button>
                </div>
              </div>

              {draft.rules.length === 0 ? (
                <div className="rounded-md border border-dashed border-border px-4 py-6 text-center">
                  <p className="text-sm text-muted-foreground">
                    No category rules yet.
                  </p>
                  <p className="text-xs text-muted-foreground mt-1">
                    Add a rule, or load a sensible Video / Audio / Documents
                    starter set.
                  </p>
                </div>
              ) : (
                <ul className="space-y-3">
                  {draft.rules.map((rule, i) => (
                    <CategoryRuleRow
                      key={i}
                      index={i}
                      rule={rule}
                      onNameChange={(name) => setRule(i, { name })}
                      onDestChange={(dest_dir) => setRule(i, { dest_dir })}
                      onAddExt={(ext) => addExt(i, ext)}
                      onRemoveExt={(ext) => removeExt(i, ext)}
                      onRemove={() => removeRule(i)}
                      onBrowse={() => handleBrowse(i)}
                    />
                  ))}
                </ul>
              )}
            </div>

            <div className="flex items-center justify-end gap-2 pt-1">
              {invalid && (
                <span className="text-xs text-destructive">
                  Each rule needs a name and at least one extension.
                </span>
              )}
              {!invalid && dirty && (
                <span className="text-xs text-muted-foreground">
                  Unsaved changes
                </span>
              )}
              <Button
                type="button"
                onClick={handleSave}
                disabled={!dirty || invalid || updateMutation.isPending}
                className="gap-2"
              >
                {updateMutation.isPending && (
                  <Loader2 className="w-4 h-4 animate-spin" />
                )}
                Save rules
              </Button>
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}

interface RuleRowProps {
  index: number;
  rule: CategoryRule;
  onNameChange: (name: string) => void;
  onDestChange: (dir: string) => void;
  onAddExt: (ext: string) => void;
  onRemoveExt: (ext: string) => void;
  onRemove: () => void;
  onBrowse: () => void;
}

function CategoryRuleRow({
  index,
  rule,
  onNameChange,
  onDestChange,
  onAddExt,
  onRemoveExt,
  onRemove,
  onBrowse,
}: RuleRowProps) {
  const [extInput, setExtInput] = useState("");
  const browseAvailable = useRef(hasChooseDir()).current;

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
            {browseAvailable && (
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
          <p className="text-xs text-destructive">
            Add at least one extension.
          </p>
        )}
      </div>
    </li>
  );
}

export default CategoryManager;
