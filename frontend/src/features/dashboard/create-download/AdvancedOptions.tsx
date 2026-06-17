import { useState } from "react";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import { ChevronDown, FileCheck2, FolderTree, Settings2 } from "lucide-react";
import { cn } from "@/lib/utils";
import { useDownloadRequestStore } from "@/stores/download-request-store";
import { useCategories } from "@/_lib/services/queries/categories.queries";
import type { ChecksumAlgo } from "@/_lib/types/download-types";
import type { CategoryRule } from "@/_lib/types/setting-types";

/** RFC3339 (ISO) string -> value for <input type="datetime-local"> (local). */
function isoToLocalInput(iso?: string): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  // Shift to local time, then trim seconds/zone to the datetime-local format.
  const off = d.getTimezoneOffset() * 60000;
  return new Date(d.getTime() - off).toISOString().slice(0, 16);
}

/** datetime-local value (local) -> RFC3339 string (UTC). */
function localInputToIso(value: string): string | undefined {
  if (!value) return undefined;
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return undefined;
  return d.toISOString();
}

/**
 * Collapsible "Advanced" block for the single-download form: checksum + algo,
 * an auto-organize category override, and an optional scheduled start time.
 * All fields are optional and write straight into the shared draft.
 */
export function AdvancedOptions() {
  const [open, setOpen] = useState(false);
  const draft = useDownloadRequestStore((s) => s.draft);
  const updateDraft = useDownloadRequestStore((s) => s.updateDraft);
  const { data: categories } = useCategories();

  const algo: ChecksumAlgo = draft.checksum_algo ?? "sha256";

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <CollapsibleTrigger className="flex w-full items-center justify-between rounded-md py-2 text-sm font-medium text-foreground transition-colors hover:text-foreground/80 cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1">
        <span className="flex items-center gap-2">
          <Settings2 className="h-4 w-4" />
          Advanced options
        </span>
        <ChevronDown
          className={cn(
            "h-4 w-4 text-muted-foreground transition-transform",
            open && "rotate-180",
          )}
          aria-hidden="true"
        />
      </CollapsibleTrigger>

      <CollapsibleContent className="space-y-4 pt-2">
        {/* Checksum + algorithm */}
        <div className="space-y-2">
          <Label
            htmlFor="adv-checksum"
            className="flex items-center gap-2 text-sm font-medium"
          >
            <FileCheck2 className="h-4 w-4" />
            Checksum{" "}
            <span className="text-muted-foreground">(optional)</span>
          </Label>
          <div className="flex gap-2">
            <Input
              id="adv-checksum"
              placeholder="Expected hash to verify after download"
              value={draft.checksum ?? ""}
              onChange={(e) =>
                updateDraft({ checksum: e.target.value || undefined })
              }
              className="flex-1 bg-background text-foreground font-mono text-xs"
            />
            <NativeSelect
              value={algo}
              onChange={(e) =>
                updateDraft({ checksum_algo: e.target.value as ChecksumAlgo })
              }
              aria-label="Checksum algorithm"
              className="w-28 shrink-0"
            >
              <NativeSelectOption value="sha256">SHA-256</NativeSelectOption>
              <NativeSelectOption value="md5">MD5</NativeSelectOption>
            </NativeSelect>
          </div>
        </div>

        {/* Category override */}
        <div className="space-y-2">
          <Label
            htmlFor="adv-category"
            className="flex items-center gap-2 text-sm font-medium"
          >
            <FolderTree className="h-4 w-4" />
            Category{" "}
            <span className="text-muted-foreground">(optional)</span>
          </Label>
          <NativeSelect
            id="adv-category"
            value={draft.category ?? ""}
            onChange={(e) =>
              updateDraft({ category: e.target.value || undefined })
            }
            className="w-full"
          >
            <NativeSelectOption value="">Auto-detect</NativeSelectOption>
            {(categories?.rules ?? []).map((rule: CategoryRule) => (
              <NativeSelectOption key={rule.name} value={rule.name}>
                {rule.name}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </div>

        {/* Scheduled start */}
        <div className="space-y-2">
          <Label htmlFor="adv-start" className="text-sm font-medium">
            Scheduled start{" "}
            <span className="text-muted-foreground">(optional)</span>
          </Label>
          <Input
            id="adv-start"
            type="datetime-local"
            value={isoToLocalInput(draft.start_at)}
            onChange={(e) =>
              updateDraft({ start_at: localInputToIso(e.target.value) })
            }
            className="bg-background text-foreground"
          />
          <p className="text-xs text-muted-foreground">
            Leave empty to start the download immediately.
          </p>
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}

export default AdvancedOptions;
