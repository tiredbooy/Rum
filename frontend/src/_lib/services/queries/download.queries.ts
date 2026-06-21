import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import type {
  BatchOptions,
  Download,
  DownloadPriority,
  DownloadReq,
  DownloadStatus,
} from "@/_lib/types/download-types";
import {
  createBatch,
  createDownloads,
  deleteDownload,
  deleteDownloads,
  getDownloads,
  fetchDownloadProgress,
  pauseAllDownloads,
  pauseDownload,
  repairDownload,
  resumeDownload,
  retryDownload,
  setDownloadPriority,
  startAllDownloads,
  startDownload,
  verifyDownload,
  fetchDashboardStats,
} from "../api/download-api";
import { useVisibilityChange } from "@/hooks/useVisibilityChange";
import { toast } from "@/lib/toast";

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
      toast.success("Download added");
      queryClient.invalidateQueries({ queryKey: downloadKeys.all });
    },
    onError: (err) => {
      toast.error(
        err instanceof Error ? err.message : "Failed to add download",
      );
    },
  });
}

export function useCreateBatch() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      urls,
      options,
    }: {
      urls: string[];
      options?: BatchOptions;
    }) => createBatch(urls, options),
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
    onError: (err) => {
      toast.error(
        err instanceof Error ? err.message : "Failed to start download",
      );
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
    onError: (err) => {
      toast.error(
        err instanceof Error ? err.message : "Failed to pause download",
      );
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
    onError: (err) => {
      toast.error(
        err instanceof Error ? err.message : "Failed to resume download",
      );
    },
  });
}

/**
 * Patch a single download across every cached query (detail + any list) so the
 * UI reflects an optimistic change immediately, mirroring the SSE applyPatch.
 */
function patchDownloadInCaches(
  queryClient: ReturnType<typeof useQueryClient>,
  id: string,
  patch: Partial<Download>,
) {
  queryClient.setQueryData<Download>(
    downloadKeys.detail(id),
    (old: Download | undefined) => (old ? { ...old, ...patch } : old),
  );
  queryClient
    .getQueriesData<Download[]>({ queryKey: downloadKeys.all })
    .forEach(([key, list]) => {
      if (!Array.isArray(list)) return;
      let changed = false;
      const next = list.map((dl) => {
        if (dl.id !== id) return dl;
        changed = true;
        return { ...dl, ...patch };
      });
      if (changed) queryClient.setQueryData(key, next);
    });
}

export function useRetryDownload() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => retryDownload(id),
    onMutate: (id) => {
      // Optimistic: clear the error and flip to pending so the card stops
      // showing the failed state right away.
      patchDownloadInCaches(queryClient, id, {
        status: "pending",
        error: undefined,
      });
    },
    onSuccess: () => {
      toast.success("Download retry started");
      queryClient.invalidateQueries({ queryKey: downloadKeys.all });
    },
    onError: (err, id) => {
      // Roll back to the errored state on failure.
      patchDownloadInCaches(queryClient, id, { status: "error" });
      toast.error(
        err instanceof Error ? err.message : "Failed to retry download",
      );
      queryClient.invalidateQueries({ queryKey: downloadKeys.all });
    },
  });
}

/**
 * Verify a download's integrity. Read-only on the server (never mutates the
 * file), so this does not patch the list cache; it just surfaces the result + a
 * toast. Pass `{ id, deep }`; deep forces a network re-validation.
 */
export function useVerifyDownload() {
  return useMutation({
    mutationFn: ({ id, deep }: { id: string; deep?: boolean }) =>
      verifyDownload(id, deep),
    onSuccess: (res) => {
      if (res.ok) {
        toast.success("File verified — integrity OK");
      } else {
        const n = res.bad_segments?.length ?? 0;
        toast.error(
          res.detail ||
            (n > 0
              ? `Verification failed — ${n} bad segment(s)`
              : "Verification failed"),
        );
      }
    },
    onError: (err) => {
      toast.error(
        err instanceof Error ? err.message : "Failed to verify download",
      );
    },
  });
}

/**
 * Repair a download by re-fetching its bad/missing segments, then re-verifying.
 * Mirrors the retry mutation: it changes job state, so on success it invalidates
 * the list cache to reflect the repaired/completed status. Pass `{ id, segments? }`.
 */
export function useRepairDownload() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, segments }: { id: string; segments?: number[] }) =>
      repairDownload(id, segments),
    onMutate: ({ id }) => {
      // Optimistically flip to running while the repair re-fetches segments.
      patchDownloadInCaches(queryClient, id, { status: "running" });
    },
    onSuccess: (res) => {
      if (res.ok) {
        toast.success("Repair complete — file is intact");
      } else {
        const n = res.bad_segments?.length ?? 0;
        toast.error(
          res.detail ||
            (n > 0
              ? `Repair incomplete — ${n} segment(s) still bad`
              : "Repair incomplete"),
        );
      }
      queryClient.invalidateQueries({ queryKey: downloadKeys.all });
    },
    onError: (err, { id }) => {
      patchDownloadInCaches(queryClient, id, { status: "error" });
      toast.error(
        err instanceof Error ? err.message : "Failed to repair download",
      );
      queryClient.invalidateQueries({ queryKey: downloadKeys.all });
    },
  });
}

export function useSetDownloadPriority() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, priority }: { id: string; priority: DownloadPriority }) =>
      setDownloadPriority(id, priority),
    onMutate: ({ id, priority }) => {
      const prev = queryClient.getQueryData<Download>(downloadKeys.detail(id));
      patchDownloadInCaches(queryClient, id, { priority });
      return { prevPriority: prev?.priority };
    },
    onSuccess: (_data, { priority }) => {
      const label = priority.charAt(0).toUpperCase() + priority.slice(1);
      toast.success(`Priority set to ${label}`);
    },
    onError: (err, { id }, ctx) => {
      // Roll back to the previous priority on failure.
      patchDownloadInCaches(queryClient, id, {
        priority: ctx?.prevPriority,
      });
      toast.error(
        err instanceof Error ? err.message : "Failed to update priority",
      );
    },
  });
}

export function useDeleteDownload() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deleteDownload(id),
    onSuccess: async (_, id) => {
      toast.success("Download deleted");
      const downloads = await queryClient.fetchQuery({
        queryKey: downloadKeys.list("all"),
        queryFn: () => getDownloads("all"),
      });

      const deletedDownloads = downloads.find((d) => d.id === id);
      if (deletedDownloads) {
        queryClient.setQueryData(downloadKeys.detail(id), deletedDownloads);
      }
    },
    onError: async (err, id) => {
      toast.error(
        err instanceof Error ? err.message : "Failed to delete download",
      );
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

export const useDashboardStats = (enabled: boolean = true) => {
  const isVisible = useVisibilityChange();
  return useQuery({
    queryKey: ["dashboardStats"],
    queryFn: fetchDashboardStats,
    refetchInterval: isVisible ? 5000 : false,
    enabled,
  });
};
