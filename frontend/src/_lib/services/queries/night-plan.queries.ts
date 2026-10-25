import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  getNightPlan,
  moveToNightQueue,
  putNightPlan,
} from "../api/night-plan-api";
import { downloadKeys } from "./download.queries";
import type { NightPlan, NightPlanState } from "@/_lib/types/setting-types";
import { toast } from "@/lib/toast";

export const nightPlanKeys = {
  all: ["settings", "night-plan"] as const,
};

/** The night plan with live window state; re-polled so chips stay current. */
export function useNightPlan() {
  return useQuery<NightPlanState>({
    queryKey: nightPlanKeys.all,
    queryFn: getNightPlan,
    staleTime: 30_000,
    refetchInterval: 60_000,
  });
}

export function useUpdateNightPlan() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: NightPlan) => putNightPlan(data),
    onSuccess: (updated) => {
      queryClient.setQueryData(nightPlanKeys.all, updated);
      queryClient.invalidateQueries({ queryKey: downloadKeys.all });
      toast.success("Night plan saved");
    },
    // Field errors are shown inline by the editor; no toast for those.
    onError: (err) => {
      toast.error(
        err instanceof Error ? err.message : "Could not save night plan",
      );
    },
  });
}

export function useMoveToNightQueue() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ ids, enabled }: { ids: string[]; enabled: boolean }) =>
      moveToNightQueue(ids, enabled),
    onSuccess: ({ updated }, { enabled }) => {
      queryClient.invalidateQueries({ queryKey: downloadKeys.all });
      toast.success(
        updated === 0
          ? "Nothing to move"
          : enabled
            ? `${updated} moved to night queue`
            : `${updated} removed from night queue`,
      );
    },
    onError: () => toast.error("Could not move downloads. Try again."),
  });
}
