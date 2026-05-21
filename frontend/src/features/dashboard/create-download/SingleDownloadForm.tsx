import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Clipboard } from "lucide-react";
import { useState, useCallback, useEffect } from "react";
import { useClipboardUrl } from "@/hooks/useClipBoardUrl";

export function SingleDownloadForm() {
  const { clipboardUrl, pasteValidUrl } = useClipboardUrl();
  const [url, setUrl] = useState("");
  const [filename, setFilename] = useState("");
  const [outputPath, setOutputPath] = useState("");
  const [urlError, setUrlError] = useState("");

  // Pre‑fill URL if clipboard had a valid one on mount
  useEffect(() => {
    if (clipboardUrl) {
      setUrl(clipboardUrl);
    }
  }, [clipboardUrl]);

  const validateUrl = useCallback((value: string) => {
    if (!value.trim()) {
      setUrlError("URL is required");
      return false;
    }
    try {
      new URL(value.trim());
      setUrlError("");
      return true;
    } catch {
      setUrlError("Please enter a valid URL");
      return false;
    }
  }, []);

  const handleUrlChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value;
    setUrl(value);
    if (urlError) validateUrl(value); // clear error as user types
  };

  const handleUrlBlur = () => {
    if (url.trim()) validateUrl(url);
  };

  const handleManualPaste = async () => {
    const pasted = await pasteValidUrl();
    if (pasted) {
      setUrl(pasted);
      validateUrl(pasted);
    }
  };

  return (
    <div className="space-y-4">
      {/* URL field */}
      <div className="space-y-2">
        <Label htmlFor="download-url" className="text-sm font-medium">
          Download URL <span className="text-destructive">*</span>
        </Label>
        <div className="flex gap-2">
          <Input
            id="download-url"
            placeholder="https://example.com/file.zip"
            value={url}
            onChange={handleUrlChange}
            onBlur={handleUrlBlur}
            className="flex-1 bg-background text-foreground"
            aria-invalid={!!urlError}
          />
          <Button
            type="button"
            size="icon"
            variant="outline"
            onClick={handleManualPaste}
            title="Paste from clipboard"
            className="shrink-0"
          >
            <Clipboard className="h-4 w-4" />
          </Button>
        </div>
        {urlError && (
          <p className="text-sm text-destructive" role="alert">
            {urlError}
          </p>
        )}
      </div>

      {/* Optional filename */}
      <div className="space-y-2">
        <Label htmlFor="filename" className="text-sm font-medium">
          Filename <span className="text-muted-foreground">(optional)</span>
        </Label>
        <Input
          id="filename"
          placeholder="my-video.mp4"
          value={filename}
          onChange={(e) => setFilename(e.target.value)}
          className="bg-background text-foreground"
        />
      </div>

      {/* Optional output path */}
      <div className="space-y-2">
        <Label htmlFor="output-path" className="text-sm font-medium">
          Save to folder{" "}
          <span className="text-muted-foreground">(optional)</span>
        </Label>
        <Input
          id="output-path"
          placeholder="videos/tutorials"
          value={outputPath}
          onChange={(e) => setOutputPath(e.target.value)}
          className="bg-background text-foreground"
        />
      </div>
    </div>
  );
}
