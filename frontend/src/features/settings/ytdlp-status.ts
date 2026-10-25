import type { YtdlpStatus } from "@/_lib/types/video-types";

const SOURCES: Record<string, string> = {
  configured: "your path",
  path: "PATH",
  managed: "Rum",
};

/** One short line: what the Video tab can rely on right now. */
export function ytdlpStatusLine(s: YtdlpStatus): string {
  const ffmpeg = s.ffmpeg ? "" : " No ffmpeg: single-file formats only.";
  if (!s.found) {
    return (s.can_install ? "yt-dlp is not installed." : "yt-dlp is not installed. Install it yourself.") + ffmpeg;
  }
  const ver = s.version ? ` ${s.version}` : "";
  return `yt-dlp${ver} ready (from ${SOURCES[s.source ?? ""] ?? "system"}).${ffmpeg}`;
}
