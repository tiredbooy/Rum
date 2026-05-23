import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { getSettings, updateSettings } from "../api/settings-api";
import { SettingReq } from "@/_lib/types/setting-types";

export const settingsKeys = {
  all: ["settings"] as const,
};

export function useSettings() {
  return useQuery<SettingReq>({
    queryKey: settingsKeys.all,
    queryFn: getSettings,
    staleTime: 1000 * 60 * 5, // cache for 5 min
  });
}

export function useUpdateSettings() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: Partial<SettingReq>) => updateSettings(data),
    onSuccess: (updated) => {
      queryClient.setQueryData(settingsKeys.all, updated);
    },
  });
}
