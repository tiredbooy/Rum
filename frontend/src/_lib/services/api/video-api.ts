import type { VideoDownloadReq, VideoInfo, YtdlpStatus } from "@/_lib/types/video-types";
import { request } from "./api";

const post = <T>(path: string, body?: unknown) =>
  request<T>(path, { method: "POST", body: JSON.stringify(body ?? {}) });

export const getYtdlpStatus = () => request<YtdlpStatus>("/api/v1/ytdlp");

/** Downloads the official yt-dlp build and verifies its checksum. */
export const installYtdlp = () => post<YtdlpStatus>("/api/v1/ytdlp/install");

export const updateYtdlp = () => post<YtdlpStatus>("/api/v1/ytdlp/update");

/** Lists the formats of a page. Can take up to a minute. */
export const getVideoInfo = (url: string) => post<VideoInfo>("/api/v1/video/info", { url });

export const createVideoDownload = (req: VideoDownloadReq) =>
  post<{ jobs: { id: string; url: string; status: string }[] }>("/api/v1/video/downloads", req);
