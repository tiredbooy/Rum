import { describe, expect, it } from "vitest";
import {
  canCheckVideo,
  canDownloadVideo,
  defaultVideoFormat,
  ffmpegNote,
  optionLabel,
  videoFormState,
  videoErrorView,
} from "./video-format";
import type { VideoInfo } from "./types/video-types";

const info: VideoInfo = {
  title: "Test Video",
  ffmpeg: true,
  options: [
    { key: "best", label: "Best (1080p)", est_size: 41_500_000 },
    { key: "h720", label: "720p", est_size: 20_000_000 },
    { key: "audio", label: "Audio only" },
  ],
};

describe("videoFormState", () => {
  it("is idle before anything is checked", () => {
    expect(videoFormState({ checking: false })).toEqual({ kind: "idle" });
  });

  it("is checking while formats load, even if an old result exists", () => {
    expect(videoFormState({ checking: true, info })).toEqual({ kind: "checking" });
  });

  it("is ready with title and options", () => {
    const s = videoFormState({ checking: false, info });
    expect(s).toMatchObject({ kind: "ready", title: "Test Video" });
  });

  it("asks to install yt-dlp when it is missing", () => {
    expect(videoFormState({ checking: false, errorCode: "ytdlp_missing" })).toEqual({ kind: "missing" });
  });

  it("shows a short error for other failures", () => {
    const s = videoFormState({ checking: false, errorCode: "video_unavailable" });
    expect(s.kind).toBe("error");
    if (s.kind === "error") expect(s.message).toBe("Video unavailable. Check the link, or try a VPN.");
  });

  it("falls back to the generic message for unknown codes", () => {
    const s = videoFormState({ checking: false, errorCode: "weird_code" });
    expect(s.kind).toBe("error");
  });

  it("keeps every video error within 12 words", () => {
    for (const code of ["video_unavailable", "ytdlp_missing", "ytdlp_failed", "dns", "timeout", "proxy"]) {
      const { message } = videoErrorView(code);
      expect(message.split(/\s+/).length, code).toBeLessThanOrEqual(12);
    }
  });
});

describe("picker helpers", () => {
  it("labels options with size when known", () => {
    expect(optionLabel(info.options[0])).toBe("Best (1080p) · 39.6 MB");
    expect(optionLabel(info.options[2])).toBe("Audio only");
  });

  it("defaults to best, else the first option", () => {
    expect(defaultVideoFormat(info.options)).toBe("best");
    expect(defaultVideoFormat([{ key: "audio", label: "Audio only" }])).toBe("audio");
    expect(defaultVideoFormat([])).toBe("");
  });

  it("says why options are limited without ffmpeg", () => {
    expect(ffmpegNote(false)).toBe("No ffmpeg: single-file formats only.");
    expect(ffmpegNote(true)).toBeNull();
  });
});

describe("button gating", () => {
  it("checks only a valid http(s) link and not while busy", () => {
    expect(canCheckVideo("https://youtu.be/x", false)).toBe(true);
    expect(canCheckVideo("https://youtu.be/x", true)).toBe(false);
    expect(canCheckVideo("youtu.be/x", false)).toBe(false);
    expect(canCheckVideo("ftp://x/y", false)).toBe(false);
    expect(canCheckVideo("  ", false)).toBe(false);
  });

  it("downloads only a chosen, offered format", () => {
    const ready = videoFormState({ checking: false, info });
    expect(canDownloadVideo(ready, "h720", false)).toBe(true);
    expect(canDownloadVideo(ready, "h480", false)).toBe(false);
    expect(canDownloadVideo(ready, "h720", true)).toBe(false);
    expect(canDownloadVideo({ kind: "idle" }, "best", false)).toBe(false);
  });
});
