import { useEffect, useMemo, useState } from "react";
import {
  useCategories,
  useUpdateCategories,
} from "@/_lib/services/queries/categories.queries";
import type { CategoryRule, CategorySettings } from "@/_lib/types/setting-types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { FolderTree, Loader2, Plus, RotateCcw } from "lucide-react";
import { CategoryRuleRow } from "./CategoryRuleRow";
import { DEFAULT_CATEGORY_RULES } from "./category-defaults";
import { useFolderPicker } from "./useFolderPicker";

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

/**
 * Auto-organize rules (PUT /settings/categories). Edits are a draft until
 * "Save rules"; the master switch is part of that draft, so it saves with the
 * rules rather than needing a second action.
 */
export function CategoryManager() {
  const { data, isLoading, isError } = useCategories();
  const updateMutation = useUpdateCategories();
  const { canPick, pick } = useFolderPicker();

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
    const dir = await pick();
    if (dir) setRule(i, { dest_dir: dir });
  };

  const handleSave = () => {
    if (invalid) return;
    updateMutation.mutate(draft);
  };

  const resetToDefaults = () =>
    setDraft((d) => ({
      ...d,
      rules: DEFAULT_CATEGORY_RULES.map((r) => ({ ...r })),
    }));

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
                  Move finished downloads into a folder by file type.
                </p>
              </div>
              <Switch
                id="categories-enabled"
                checked={draft.enabled}
                onCheckedChange={(v) => setDraft((d) => ({ ...d, enabled: v }))}
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
                    <RotateCcw className="w-4 h-4" /> Use defaults
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
                <div className="rounded-md border border-dashed border-border px-4 py-6 text-center space-y-2">
                  <p className="text-sm text-muted-foreground">No rules yet.</p>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={resetToDefaults}
                  >
                    Use defaults
                  </Button>
                </div>
              ) : (
                <ul className="space-y-3">
                  {draft.rules.map((rule, i) => (
                    <CategoryRuleRow
                      key={i}
                      index={i}
                      rule={rule}
                      canBrowse={canPick}
                      onNameChange={(name) => setRule(i, { name })}
                      onDestChange={(dest_dir) => setRule(i, { dest_dir })}
                      onAddExt={(ext) => addExt(i, ext)}
                      onRemoveExt={(ext) => removeExt(i, ext)}
                      onRemove={() => removeRule(i)}
                      onBrowse={() => void handleBrowse(i)}
                    />
                  ))}
                </ul>
              )}
            </div>

            <div className="flex items-center justify-end gap-2 pt-1">
              {invalid && (
                <span role="alert" className="text-xs text-destructive">
                  Each rule needs a name and one extension.
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

export default CategoryManager;
