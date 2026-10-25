import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiError } from "../api/api";
import {
  createVideoDownload,
  getVideoInfo,
  getYtdlpStatus,
  installYtdlp,
  updateYtdlp,
} from "../api/video-api";
import { downloadKeys } from "./download.queries";
import { toast } from "@/lib/toast";
import type { VideoDownloadReq, YtdlpStatus } from "@/_lib/types/video-types";
import { videoErrorView } from "@/_lib/video-format";

export const ytdlpKeys = { status: ["ytdlp", "status"] as const };

/** The machine error code of a failed video call, if the API sent one. */
export function videoErrorCode(err: unknown): string | undefined {
  return err instanceof ApiError ? err.code : undefined;
}

export function useYtdlpStatus() {
  return useQuery<YtdlpStatus>({ queryKey: ytdlpKeys.status, queryFn: getYtdlpStatus, staleTime: 15_000 });
}

function useToolMutation(fn: () => Promise<YtdlpStatus>, done: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: fn,
    onSuccess: (status) => {
      queryClient.setQueryData(ytdlpKeys.status, status);
      toast.success(done);
    },
    onError: (err) => toast.error(videoErrorView(videoErrorCode(err) ?? "ytdlp_failed").message),
  });
}

export const useInstallYtdlp = () => useToolMutation(installYtdlp, "yt-dlp installed");
export const useUpdateYtdlp = () => useToolMutation(updateYtdlp, "yt-dlp updated");

/** Lists a page's formats. The caller reads `error` for the code. */
export function useVideoInfo() {
  return useMutation({ mutationFn: (url: string) => getVideoInfo(url) });
}

export function useCreateVideoDownload() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (req: VideoDownloadReq) => createVideoDownload(req),
    onSuccess: () => {
      toast.success("Video added");
      queryClient.invalidateQueries({ queryKey: downloadKeys.all });
    },
    onError: (err) => toast.error(videoErrorView(videoErrorCode(err) ?? "unknown").message),
  });
}
