/** One entry of the format picker (POST /api/v1/video/info). */
export interface VideoOption {
  /** best | h1080 | h720 | h480 | audio */
  key: string;
  label: string;
  /** Best-effort bytes; absent when unknown. */
  est_size?: number;
}

export interface VideoInfo {
  title: string;
  uploader?: string;
  /** Seconds. */
  duration?: number;
  /** False: only single-file formats are offered. */
  ffmpeg: boolean;
  options: VideoOption[];
}

/** GET /api/v1/ytdlp */
export interface YtdlpStatus {
  found: boolean;
  /** configured | path | managed */
  source?: string;
  path?: string;
  version?: string;
  ffmpeg: boolean;
  can_install: boolean;
}

export interface VideoDownloadReq {
  url: string;
  format: string;
  dest_path?: string;
  auto_start?: boolean;
}
