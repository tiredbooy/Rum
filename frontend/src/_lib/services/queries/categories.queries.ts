import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { getCategories, putCategories } from "../api/categories-api";
import type { CategorySettings } from "@/_lib/types/setting-types";
import { toast } from "@/lib/toast";

export const categoriesKeys = {
  all: ["settings", "categories"] as const,
};

export function useCategories() {
  return useQuery<CategorySettings>({
    queryKey: categoriesKeys.all,
    queryFn: getCategories,
    staleTime: 1000 * 60 * 5,
  });
}

/**
 * Optimistic category update with rollback — same pattern as the priority and
 * schedule mutations.
 */
export function useUpdateCategories() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: CategorySettings) => putCategories(data),
    onMutate: async (data) => {
      await queryClient.cancelQueries({ queryKey: categoriesKeys.all });
      const prev = queryClient.getQueryData<CategorySettings>(
        categoriesKeys.all,
      );
      queryClient.setQueryData(categoriesKeys.all, data);
      return { prev };
    },
    onSuccess: (updated) => {
      queryClient.setQueryData(categoriesKeys.all, updated);
      toast.success("Category rules saved");
    },
    onError: (err, _data, ctx) => {
      if (ctx?.prev) queryClient.setQueryData(categoriesKeys.all, ctx.prev);
      toast.error(
        err instanceof Error ? err.message : "Failed to save categories",
      );
    },
  });
}
