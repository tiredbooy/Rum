import { useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Loader2, Send } from "lucide-react";
import { expandWildcard, validateRange, type WildcardRange } from "./wildcard";
import { useCreateBatch } from "@/_lib/services/queries/download.queries";
import type { BatchOptions } from "@/_lib/types/download-types";

export function WildcardExpander({
  url,
  options,
  onDone,
}: {
  url: string;
  options?: BatchOptions;
  onDone?: () => void;
}) {
  const createBatch = useCreateBatch();
  const [range, setRange] = useState<WildcardRange>({ from: 1, to: 10, pad: 0 });

  const error = validateRange(range);
  const preview = useMemo(() => {
    if (error) return null;
    try {
      const urls = expandWildcard(url, range);
      return { count: urls.length, first: urls[0], last: urls[urls.length - 1] };
    } catch {
      return null;
    }
  }, [url, range, error]);

  const set = (k: keyof WildcardRange) => (e: React.ChangeEvent<HTMLInputElement>) =>
    setRange((r) => ({ ...r, [k]: Number(e.target.value) }));

  const handleAdd = () => {
    if (error) return;
    const urls = expandWildcard(url, range);
    createBatch.mutate({ urls, options }, { onSuccess: () => onDone?.() });
  };

  return (
    <div className="space-y-3 rounded-md border border-border p-3">
      <p className="text-sm font-medium">Wildcard detected — expand range</p>
      <p className="text-xs text-muted-foreground break-all">{url}</p>
      <div className="grid grid-cols-3 gap-2">
        <div className="space-y-1">
          <Label className="text-xs">From</Label>
          <Input type="number" min={0} value={range.from} onChange={set("from")} />
        </div>
        <div className="space-y-1">
          <Label className="text-xs">To</Label>
          <Input type="number" min={0} value={range.to} onChange={set("to")} />
        </div>
        <div className="space-y-1">
          <Label className="text-xs">Zero-pad digits</Label>
          <Input type="number" min={0} max={12} value={range.pad} onChange={set("pad")} />
        </div>
      </div>
      {error ? (
        <p className="text-xs text-destructive">{error}</p>
      ) : preview ? (
        <div className="text-xs text-muted-foreground font-mono">
          <div>{preview.first}</div>
          <div>… {preview.count} files …</div>
          <div>{preview.last}</div>
        </div>
      ) : null}
      <Button onClick={handleAdd} disabled={!!error || createBatch.isPending} className="w-full gap-2">
        {createBatch.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
        Add {preview ? preview.count : 0} download{preview?.count !== 1 ? "s" : ""}
      </Button>
    </div>
  );
}

export default WildcardExpander;
