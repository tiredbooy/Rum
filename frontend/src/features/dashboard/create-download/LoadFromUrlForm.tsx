import { useState, useEffect } from "react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Loader2, Globe, CheckSquare, Square } from "lucide-react";
import { isValidUrl } from "@/_lib/utils/validators";
import { useClipboardUrl } from "@/hooks/useClipBoardUrl";

interface ParsedLine {
  index: number;
  url: string;
  selected: boolean;
}

export function LoadFromUrlForm() {
  const { readRawText } = useClipboardUrl();

  const [sourceUrl, setSourceUrl] = useState("");
  const [outputFolder, setOutputFolder] = useState("");
  const [isFetching, setIsFetching] = useState(false);
  const [fetchError, setFetchError] = useState("");
  const [parsedLines, setParsedLines] = useState<ParsedLine[]>([]);
  const [sourceUrlError, setSourceUrlError] = useState("");

  const totalUrls = parsedLines.length;
  const selectedCount = parsedLines.filter((l) => l.selected).length;

  const validateSource = (url: string) => {
    if (!url.trim()) {
      setSourceUrlError("URL is required");
      return false;
    }
    try {
      new URL(url.trim());
      setSourceUrlError("");
      return true;
    } catch {
      setSourceUrlError("Please enter a valid URL");
      return false;
    }
  };

  const handleSourceChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value;
    setSourceUrl(value);
    if (sourceUrlError) validateSource(value);
  };

  const handleFetchLinks = async () => {
    if (!validateSource(sourceUrl)) return;

    setIsFetching(true);
    setFetchError("");
    setParsedLines([]);

    try {
      const response = await fetch(sourceUrl.trim());
      if (!response.ok) {
        throw new Error(`Failed to fetch (${response.status})`);
      }
      const text = await response.text();
      const rawLines = text.split("\n");
      const validLines: ParsedLine[] = rawLines
        .map((line, index) => ({ index, url: line.trim() }))
        .filter((line) => isValidUrl(line.url))
        .map((line) => ({ ...line, selected: true }));

      if (validLines.length === 0) {
        setFetchError("No valid download URLs found in the response.");
      } else {
        setParsedLines(validLines);
      }
    } catch (err: any) {
      setFetchError(err.message || "Something went wrong while fetching.");
    } finally {
      setIsFetching(false);
    }
  };

  const toggleLine = (index: number) => {
    setParsedLines((prev) =>
      prev.map((line) =>
        line.index === index ? { ...line, selected: !line.selected } : line,
      ),
    );
  };

  const selectAll = () => {
    setParsedLines((prev) => prev.map((line) => ({ ...line, selected: true })));
  };

  const deselectAll = () => {
    setParsedLines((prev) =>
      prev.map((line) => ({ ...line, selected: false })),
    );
  };

  // Auto‑paste a URL from clipboard into the source field on mount (if empty)
  useEffect(() => {
    if (sourceUrl === "") {
      readRawText().then((text) => {
        if (isValidUrl(text.trim())) {
          setSourceUrl(text.trim());
        }
      });
    }
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div className="space-y-4">
      {/* Output folder */}
      <div className="space-y-2">
        <Label htmlFor="url-output" className="text-sm font-medium">
          Save to folder{" "}
          <span className="text-muted-foreground">(optional)</span>
        </Label>
        <Input
          id="url-output"
          placeholder="downloads/from-url"
          value={outputFolder}
          onChange={(e) => setOutputFolder(e.target.value)}
          className="bg-background text-foreground"
        />
      </div>

      {/* Source URL input */}
      <div className="space-y-2">
        <Label htmlFor="source-url" className="text-sm font-medium">
          URL of the link list <span className="text-destructive">*</span>
        </Label>
        <div className="flex flex-col sm:flex-row gap-2">
          <Input
            id="source-url"
            placeholder="https://example.com/links.txt"
            value={sourceUrl}
            onChange={handleSourceChange}
            onBlur={() => sourceUrl.trim() && validateSource(sourceUrl)}
            className="flex-1 bg-background text-foreground"
            aria-invalid={!!sourceUrlError}
          />
          <Button
            onClick={handleFetchLinks}
            disabled={isFetching || !sourceUrl.trim()}
            className="gap-2 shrink-0"
          >
            {isFetching ? (
              <>
                <Loader2 className="h-4 w-4 animate-spin" />
                Fetching...
              </>
            ) : (
              <>
                <Globe className="h-4 w-4" />
                Fetch links
              </>
            )}
          </Button>
        </div>
        {sourceUrlError && (
          <p className="text-sm text-destructive">{sourceUrlError}</p>
        )}
      </div>

      {fetchError && (
        <p className="text-sm text-destructive font-medium">{fetchError}</p>
      )}

      {totalUrls > 0 && (
        <div className="space-y-3">
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

          <div className="max-h-48 overflow-y-auto overflow-x-hidden rounded-md border border-border divide-y divide-border">
            {parsedLines.map((line) => (
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
    </div>
  );
}
