import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { getSchedule, putSchedule } from "../api/schedule-api";
import type { ScheduleSettings } from "@/_lib/types/setting-types";
import { toast } from "@/lib/toast";

export const scheduleKeys = {
  all: ["settings", "schedule"] as const,
};

export function useSchedule() {
  return useQuery<ScheduleSettings>({
    queryKey: scheduleKeys.all,
    queryFn: getSchedule,
    staleTime: 1000 * 60 * 5,
  });
}

/**
 * Optimistic schedule update with rollback — mirrors the priority mutation
 * pattern: snapshot the cache, write the new value immediately, and restore on
 * error so the editor never lies about persisted state.
 */
export function useUpdateSchedule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: ScheduleSettings) => putSchedule(data),
    onMutate: async (data) => {
      await queryClient.cancelQueries({ queryKey: scheduleKeys.all });
      const prev = queryClient.getQueryData<ScheduleSettings>(scheduleKeys.all);
      queryClient.setQueryData(scheduleKeys.all, data);
      return { prev };
    },
    onSuccess: (updated) => {
      queryClient.setQueryData(scheduleKeys.all, updated);
      toast.success("Bandwidth schedule saved");
    },
    onError: (err, _data, ctx) => {
      if (ctx?.prev) queryClient.setQueryData(scheduleKeys.all, ctx.prev);
      toast.error(
        err instanceof Error ? err.message : "Failed to save schedule",
      );
    },
  });
}
