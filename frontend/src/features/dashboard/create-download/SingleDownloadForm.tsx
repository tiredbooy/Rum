import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Clipboard } from "lucide-react";
import { useState, useCallback, useEffect } from "react";
import { useClipboardUrl } from "@/hooks/useClipBoardUrl";
import { useDownloadRequestStore } from "@/stores/download-request-store";
import { isValidUrl } from "@/_lib/utils/validators";
import { AdvancedOptions } from "./AdvancedOptions";

export function SingleDownloadForm() {
  const { clipboardUrl, pasteValidUrl } = useClipboardUrl();

  const draft = useDownloadRequestStore((s) => s.draft);
  const updateDraft = useDownloadRequestStore((s) => s.updateDraft);

  const url = draft.urls[0] ?? "";
  const filename = draft.filename ?? "";
  const destPath = draft.dest_path ?? "";

  const [urlError, setUrlError] = useState("");

  useEffect(() => {
    if (clipboardUrl && !draft.urls[0]) {
      updateDraft({ urls: [clipboardUrl] });
    }
  }, [clipboardUrl]);

  const validateUrl = useCallback((value: string) => {
    if (!value.trim()) {
      setUrlError("URL is required");
      return false;
    }
    return isValidUrl(value?.trim());
  }, []);

  const handleUrlChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value;
    updateDraft({ urls: [value] });
    if (urlError) validateUrl(value);
  };

  const handleUrlBlur = () => {
    if (url.trim()) validateUrl(url);
  };

  const handleManualPaste = async () => {
    const pasted = await pasteValidUrl();
    if (pasted) {
      updateDraft({ urls: [pasted] });
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

      <div className="space-y-2">
        <Label htmlFor="filename" className="text-sm font-medium">
          Filename <span className="text-muted-foreground">(optional)</span>
        </Label>
        <Input
          id="filename"
          placeholder="my-video.mp4"
          value={filename}
          onChange={(e) => updateDraft({ filename: e.target.value })}
          className="bg-background text-foreground"
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="output-path" className="text-sm font-medium">
          Save to folder{" "}
          <span className="text-muted-foreground">(optional)</span>
        </Label>
        <Input
          id="output-path"
          placeholder="videos/tutorials"
          value={destPath}
          onChange={(e) => updateDraft({ dest_path: e.target.value })}
          className="bg-background text-foreground"
        />
      </div>

      <AdvancedOptions />
    </div>
  );
}
