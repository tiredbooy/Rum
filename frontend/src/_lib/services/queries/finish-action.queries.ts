import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { cancelFinishAction, getFinishAction } from "../api/finish-action-api";
import type { FinishPending } from "@/_lib/types/setting-types";
import { toast } from "@/lib/toast";

export const finishActionKey = ["finish-action"] as const;

/** Polls the backend; faster while a countdown is running. */
export function useFinishAction() {
  return useQuery<FinishPending | null>({
    queryKey: finishActionKey,
    queryFn: getFinishAction,
    refetchInterval: (q) => (q.state.data ? 1000 : 5000),
    retry: false,
  });
}

export function useCancelFinishAction() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: cancelFinishAction,
    onMutate: () => qc.setQueryData(finishActionKey, null),
    onError: () => toast.error("Couldn't cancel. Try again."),
  });
}
