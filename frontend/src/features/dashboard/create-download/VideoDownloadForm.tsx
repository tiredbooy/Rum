import { useEffect, useState } from "react";
import { Download, Search } from "lucide-react";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import { useAddDialogStore } from "@/stores/add-dialog-store";
import {
  useCreateVideoDownload,
  useInstallYtdlp,
  useVideoInfo,
  videoErrorCode,
} from "@/_lib/services/queries/video.queries";
import {
  canCheckVideo,
  canDownloadVideo,
  defaultVideoFormat,
  ffmpegNote,
  videoFormState,
} from "@/_lib/video-format";
import { VideoFormatPicker } from "./VideoFormatPicker";

/**
 * Add a video by its page URL (YouTube, Aparat, ...): Check lists the formats,
 * pick one, Download. Has its own primary button, so the dialog's Save is hidden
 * on this tab.
 */
export function VideoDownloadForm() {
  const [url, setUrl] = useState("");
  const [format, setFormat] = useState("");
  const [destPath, setDestPath] = useState("");
  const info = useVideoInfo();
  const create = useCreateVideoDownload();
  const install = useInstallYtdlp();

  const droppedUrl = useAddDialogStore((s) => s.droppedUrls?.[0]);
  useEffect(() => {
    if (droppedUrl) setUrl(droppedUrl);
  }, [droppedUrl]);

  const state = videoFormState({
    checking: info.isPending,
    info: info.data,
    errorCode: info.error ? (videoErrorCode(info.error) ?? "unknown") : undefined,
  });

  const check = () => {
    setFormat("");
    info.mutate(url.trim(), { onSuccess: (data) => setFormat(defaultVideoFormat(data.options)) });
  };

  const download = () => {
    create.mutate(
      { url: url.trim(), format, dest_path: destPath.trim() || undefined },
      { onSuccess: () => useAddDialogStore.getState().setOpen(false) },
    );
  };

  const installAndRetry = () => install.mutate(undefined, { onSuccess: check });

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="video-url" className="text-sm font-medium">
          Video page link
        </Label>
        <div className="flex gap-2">
          <Input
            id="video-url"
            placeholder="https://www.youtube.com/watch?v=..."
            value={url}
            onChange={(e) => {
              setUrl(e.target.value);
              if (info.data || info.error) info.reset(); // formats belong to the old link
            }}
            onKeyDown={(e) => e.key === "Enter" && canCheckVideo(url, info.isPending) && check()}
            className="flex-1 bg-background text-foreground"
          />
          <Button type="button" variant="outline" onClick={check} disabled={!canCheckVideo(url, info.isPending)}>
            <Search className="h-4 w-4" aria-hidden /> Check
          </Button>
        </div>
      </div>

      <div aria-live="polite">
        {state.kind === "checking" && (
          <p className="flex items-center gap-2 text-sm text-muted-foreground">
            <Spinner /> Checking…
          </p>
        )}

        {state.kind === "missing" && (
          <Alert className="flex items-center justify-between gap-3">
            <span>yt-dlp is not installed.</span>
            <Button type="button" size="sm" onClick={installAndRetry} disabled={install.isPending}>
              {install.isPending ? <Spinner /> : <Download className="h-4 w-4" aria-hidden />} Install yt-dlp
            </Button>
          </Alert>
        )}

        {state.kind === "error" && <Alert variant="destructive">{state.message}</Alert>}

        {state.kind === "ready" && (
          <div className="space-y-3">
            <p className="text-sm font-medium break-words">{state.title}</p>
            <VideoFormatPicker options={state.options} value={format} onChange={setFormat} />
            {ffmpegNote(state.ffmpeg) && (
              <p className="text-xs text-muted-foreground">{ffmpegNote(state.ffmpeg)}</p>
            )}
          </div>
        )}
      </div>

      {state.kind === "ready" && (
        <>
          <div className="space-y-2">
            <Label htmlFor="video-folder" className="text-sm font-medium">
              Save to folder <span className="text-muted-foreground">(optional)</span>
            </Label>
            <Input
              id="video-folder"
              placeholder="videos"
              value={destPath}
              onChange={(e) => setDestPath(e.target.value)}
              className="bg-background text-foreground"
            />
          </div>
          <div className="flex justify-end">
            <Button type="button" onClick={download} disabled={!canDownloadVideo(state, format, create.isPending)}>
              {create.isPending ? <Spinner /> : <Download className="h-4 w-4" aria-hidden />} Download
            </Button>
          </div>
        </>
      )}
    </div>
  );
}
