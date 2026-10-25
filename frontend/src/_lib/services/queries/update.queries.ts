import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { UpdateStatus } from "@/_lib/update-notice";
import {
  checkForUpdate,
  dismissUpdate,
  downloadUpdate,
  getUpdateStatus,
} from "../api/update-api";

export const updateKeys = { status: ["update", "status"] as const };

export function useUpdateStatus() {
  return useQuery<UpdateStatus>({
    queryKey: updateKeys.status,
    queryFn: getUpdateStatus,
    staleTime: Infinity,
    refetchOnWindowFocus: false,
    retry: false,
  });
}

/** `auto` = startup check (throttled by the backend); false = "Check now". */
export function useCheckForUpdate() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (auto: boolean) => checkForUpdate(auto),
    onSuccess: (status) => queryClient.setQueryData(updateKeys.status, status),
  });
}

export function useDismissUpdate() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (tag: string) => dismissUpdate(tag),
    onSuccess: (status) => queryClient.setQueryData(updateKeys.status, status),
  });
}

export function useDownloadUpdate() {
  return useMutation({ mutationFn: downloadUpdate });
}
