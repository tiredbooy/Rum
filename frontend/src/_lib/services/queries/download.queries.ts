import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import type { DownloadReq } from "@/_lib/types/download-types";
import {
  createDownloads,
  deleteDownload,
  getDownloads,
  getDownloadStatus,
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
    queryFn: () => getDownloadStatus(id),
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
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: downloadKeys.all });
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
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: downloadKeys.all });
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
