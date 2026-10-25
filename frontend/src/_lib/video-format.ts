import type { VideoInfo, VideoOption } from "@/_lib/types/video-types";
import { describeDownloadError } from "@/_lib/utils/download-errors";
import { formatBytes } from "@/_lib/utils/format";

/** What the Video tab shows. */
export type VideoFormState =
  | { kind: "idle" }
  | { kind: "checking" }
  | { kind: "missing" }
  | { kind: "error"; message: string }
  | { kind: "ready"; title: string; options: VideoOption[]; ffmpeg: boolean };

export function videoErrorView(code?: string | null): { message: string } {
  const { message, action } = describeDownloadError(code);
  return { message: `${message} ${action}` };
}

export function videoFormState(args: {
  checking: boolean;
  info?: VideoInfo | null;
  errorCode?: string | null;
}): VideoFormState {
  if (args.checking) return { kind: "checking" };
  if (args.errorCode === "ytdlp_missing") return { kind: "missing" };
  if (args.errorCode) return { kind: "error", ...videoErrorView(args.errorCode) };
  if (args.info) {
    return { kind: "ready", title: args.info.title, options: args.info.options, ffmpeg: args.info.ffmpeg };
  }
  return { kind: "idle" };
}

export function optionLabel(o: VideoOption): string {
  return o.est_size && o.est_size > 0 ? `${o.label} · ${formatBytes(o.est_size)}` : o.label;
}

export function defaultVideoFormat(options: VideoOption[]): string {
  if (options.some((o) => o.key === "best")) return "best";
  return options[0]?.key ?? "";
}

export function ffmpegNote(ffmpeg: boolean): string | null {
  return ffmpeg ? null : "No ffmpeg: single-file formats only.";
}

export function canCheckVideo(url: string, busy: boolean): boolean {
  return !busy && /^https?:\/\/\S+$/i.test(url.trim());
}

export function canDownloadVideo(state: VideoFormState, format: string, busy: boolean): boolean {
  return !busy && state.kind === "ready" && state.options.some((o) => o.key === format);
}
