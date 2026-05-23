import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import type { DownloadReq, DownloadStatus } from "@/_lib/types/download-types";
import {
  createDownloads,
  deleteDownload,
  deleteDownloads,
  getDownloads,
  fetchDownloadProgress,
  pauseAllDownloads,
  pauseDownload,
  resumeDownload,
  startAllDownloads,
  startDownload,
} from "../api/download-api";

export const downloadKeys = {
  all: ["downloads"] as const,
  list: (status?: string) => [...downloadKeys.all, "list", status] as const,
  detail: (id: string) => [...downloadKeys.all, "detail", id] as const,
};

export function useDownloads(status?: Parameters<typeof getDownloads>[0]) {
  return useQuery({
    queryKey: downloadKeys.list(status),
    queryFn: () => getDownloads(status),
  });
}

export function useDownload(id: string) {
  return useQuery({
    queryKey: downloadKeys.detail(id),
    queryFn: () => fetchDownloadProgress(id),
    enabled: !!id,
  });
}

export function useCreateDownloads() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: DownloadReq) => createDownloads(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: downloadKeys.all });
    },
  });
}

export function useStartDownload() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => startDownload(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: downloadKeys.all });
    },
  });
}

export function usePauseDownload() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => pauseDownload(id),
    onSuccess: async (_, id) => {
      const downloads = await queryClient.fetchQuery({
        queryKey: downloadKeys.list("all"),
        queryFn: () => getDownloads("all"),
      });

      const pausedJob = downloads.find((d) => d.id === id);
      if (pausedJob) {
        queryClient.setQueryData(downloadKeys.detail(id), pausedJob);
      }
    },
  });
}

export function usePauseAllDownloads() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => pauseAllDownloads(),
    onSuccess: async () => {
      const downloads = await queryClient.fetchQuery({
        queryKey: downloadKeys.list("all"),
        queryFn: () => getDownloads("all"),
      });

      downloads.forEach((download) => {
        queryClient.setQueryData(downloadKeys.detail(download.id), download);
      });
    },
  });
}

export function useResumeDownload() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => resumeDownload(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: downloadKeys.all });
    },
  });
}

export function useDeleteDownload() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deleteDownload(id),
    onSuccess: async (_, id) => {
      const downloads = await queryClient.fetchQuery({
        queryKey: downloadKeys.list("all"),
        queryFn: () => getDownloads("all"),
      });

      const deletedDownloads = downloads.find((d) => d.id === id);
      if (deletedDownloads) {
        queryClient.setQueryData(downloadKeys.detail(id), deletedDownloads);
      }
    },
    onError: async (_, id) => {
      const downloads = await queryClient.fetchQuery({
        queryKey: downloadKeys.list("all"),
        queryFn: () => getDownloads("all"),
      });

      const deletedDownloads = downloads.find((d) => d.id === id);
      if (deletedDownloads) {
        queryClient.setQueryData(downloadKeys.detail(id), deletedDownloads);
      }
    },
  });
}

export function useDeleteDownloads() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (status: DownloadStatus) => deleteDownloads(status),
    onSuccess: async () => {
      const downloads = await queryClient.fetchQuery({
        queryKey: downloadKeys.list("all"),
        queryFn: () => getDownloads("all"),
      });

      downloads.forEach((download) => {
        queryClient.setQueryData(downloadKeys.detail(download.id), download);
      });
    },
  });
}

export function useStartAllDownloads() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => startAllDownloads(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: downloadKeys.all });
    },
  });
}
