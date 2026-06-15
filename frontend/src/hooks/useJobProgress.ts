import { useQuery } from "@tanstack/react-query";
import { downloadKeys } from "@/_lib/services/queries/download.queries";
import type { Download } from "@/_lib/types/download-types";

export function useJobProgress(
  jobId: string,
  initialDownload?: Download,
  enabled: boolean = true,
) {
  return useQuery<Download>({
    queryKey: downloadKeys.detail(jobId),
    queryFn: () => Promise.resolve(initialDownload as Download),
    initialData: initialDownload,
    staleTime: Infinity,
    gcTime: 1000 * 60 * 10,
    refetchOnMount: false,
    enabled,
  });
}
