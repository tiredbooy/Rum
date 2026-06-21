import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { getSettings, updateSettings } from "../api/settings-api";
import { SettingReq } from "@/_lib/types/setting-types";
import { toast } from "@/lib/toast";

export const settingsKeys = {
  all: ["settings"] as const,
};

export function useSettings() {
  return useQuery<SettingReq>({
    queryKey: settingsKeys.all,
    queryFn: getSettings,
    staleTime: 1000 * 60 * 5, // cache for 5 min
    // The settings cards mirror text fields into local draft state and resync
    // from this cache when it changes. A window-focus refetch swaps in a fresh
    // cache object and would stomp a field the user is actively editing, so
    // disable it — every successful save already writes the merged result back
    // via setQueryData, keeping the cache fresh without a focus refetch.
    refetchOnWindowFocus: false,
  });
}

export function useUpdateSettings() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: Partial<SettingReq>) => updateSettings(data),
    onSuccess: (updated) => {
      queryClient.setQueryData(settingsKeys.all, updated);
    },
    onError: (err) => {
      // Surface PATCH failures instead of failing silently. The backend returns
      // the full merged settings on success; on failure the cache is left
      // untouched so the control snaps back to its last-known value — the toast
      // tells the user the change did not persist (matches the categories /
      // schedule / download mutations, which all toast on error).
      toast.error(
        err instanceof Error ? err.message : "Failed to save setting",
      );
    },
  });
}
