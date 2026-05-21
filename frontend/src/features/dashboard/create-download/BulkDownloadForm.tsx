import { useState, useEffect } from "react";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { Clipboard, CheckSquare, Square } from "lucide-react";
import { isValidUrl } from "@/_lib/utils/validators";
import { useClipboardUrl } from "@/hooks/useClipBoardUrl";

interface ParsedLine {
  index: number;
  text: string;
  url: string | null;
  selected: boolean;
}

export function BulkDownloadForm() {
  const [rawText, setRawText] = useState("");
  const [outputFolder, setOutputFolder] = useState("");
  const [lines, setLines] = useState<ParsedLine[]>([]);
  const { readRawText } = useClipboardUrl();

  // Parse rawText on every change
  useEffect(() => {
    const textLines = rawText.split("\n");
    const newLines: ParsedLine[] = textLines.map((line, index) => {
      const trimmed = line.trim();
      const url = isValidUrl(trimmed) ? trimmed : null;
      return {
        index,
        text: trimmed,
        url,
        selected: url ? true : false,
      };
    });
    setLines(newLines);
  }, [rawText]);

  const totalUrls = lines.filter((l) => l.url).length;
  const selectedCount = lines.filter((l) => l.selected).length;

  const toggleLine = (index: number) =>
    setLines((prev) =>
      prev.map((line) =>
        line.index === index ? { ...line, selected: !line.selected } : line,
      ),
    );

  const selectAll = () =>
    setLines((prev) => prev.map((line) => ({ ...line, selected: true })));

  const deselectAll = () =>
    setLines((prev) => prev.map((line) => ({ ...line, selected: false })));

  const handlePaste = async () => {
    const text = await readRawText();
    if (text) setRawText(text);
  };

  // Auto‑paste on first render (only if empty)
  useEffect(() => {
    if (rawText === "") {
      readRawText().then((text) => {
        if (text && text.trim()) setRawText(text);
      });
    }
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div className="space-y-4">
      {/* Output folder (global) */}
      <div className="space-y-2">
        <Label htmlFor="bulk-output" className="text-sm font-medium">
          Save to folder{" "}
          <span className="text-muted-foreground">(optional)</span>
        </Label>
        <Input
          id="bulk-output"
          placeholder="downloads/videos"
          value={outputFolder}
          onChange={(e) => setOutputFolder(e.target.value)}
          className="bg-background text-foreground"
        />
      </div>

      {/* ===== SELECTABLE URL LIST – now ABOVE the textarea ===== */}
      {totalUrls > 0 && (
        <div className="space-y-3">
          {/* Header: count + select/deselect buttons */}
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2">
            <p className="text-sm text-muted-foreground">
              {selectedCount} of {totalUrls} URL{totalUrls !== 1 && "s"}{" "}
              selected
            </p>
            <div className="flex gap-2">
              <Button variant="ghost" size="sm" onClick={selectAll}>
                <CheckSquare className="h-4 w-4 mr-1" /> Select all
              </Button>
              <Button variant="ghost" size="sm" onClick={deselectAll}>
                <Square className="h-4 w-4 mr-1" /> Deselect all
              </Button>
            </div>
          </div>

          {/* Scrollable URL list with checkboxes */}
          <div className="max-h-48 overflow-y-auto overflow-x-hidden rounded-md border border-border divide-y divide-border">
            {lines
              .filter((line) => line.url)
              .map((line) => (
                <label
                  key={line.index}
                  className="flex items-center gap-3 px-3 py-2.5 hover:bg-muted/50 cursor-pointer transition-colors min-w-0"
                >
                  <Checkbox
                    checked={line.selected}
                    onCheckedChange={() => toggleLine(line.index)}
                    className="shrink-0"
                  />
                  <span className="text-sm truncate text-foreground min-w-0">
                    {line.url}
                  </span>
                </label>
              ))}
          </div>
        </div>
      )}

      {/* Validation feedback when no valid URLs */}
      {rawText && totalUrls === 0 && (
        <p className="text-sm text-destructive font-medium">
          No valid URLs found. Enter one URL per line.
        </p>
      )}

      {/* ===== RAW TEXTAREA – now BELOW the list ===== */}
      <div className="space-y-2">
        <Label htmlFor="bulk-urls" className="text-sm font-medium">
          Edit raw URLs{" "}
          <span className="text-muted-foreground">(one per line)</span>
        </Label>
        <div className="flex flex-col gap-2">
          <Textarea
            id="bulk-urls"
            placeholder="https://example.com/file1.zip\nhttps://example.com/file2.mp4"
            value={rawText}
            onChange={(e) => setRawText(e.target.value)}
            className="min-h-[100px] bg-background text-foreground resize-y break-all max-w-full"
            rows={totalUrls > 0 ? 3 : 5}
          />
          <Button
            type="button"
            variant="outline"
            onClick={handlePaste}
            className="self-start sm:self-end gap-2"
          >
            <Clipboard className="h-4 w-4" />
            <span>Paste from clipboard</span>
          </Button>
        </div>
      </div>
    </div>
  );
}
