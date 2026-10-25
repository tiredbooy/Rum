import { describe, expect, it } from "vitest";
import { ytdlpStatusLine } from "./ytdlp-status";

describe("ytdlpStatusLine", () => {
  it("offers install when missing", () => {
    expect(ytdlpStatusLine({ found: false, ffmpeg: true, can_install: true })).toBe("yt-dlp is not installed.");
  });

  it("says to install it yourself where no build exists", () => {
    expect(ytdlpStatusLine({ found: false, ffmpeg: true, can_install: false })).toContain("Install it yourself");
  });

  it("shows version and source when found", () => {
    expect(
      ytdlpStatusLine({ found: true, source: "managed", version: "2025.01.01", ffmpeg: true, can_install: true }),
    ).toBe("yt-dlp 2025.01.01 ready (from Rum).");
  });

  it("mentions the single-file limit without ffmpeg", () => {
    expect(ytdlpStatusLine({ found: true, source: "path", ffmpeg: false, can_install: true })).toBe(
      "yt-dlp ready (from PATH). No ffmpeg: single-file formats only.",
    );
  });
});
