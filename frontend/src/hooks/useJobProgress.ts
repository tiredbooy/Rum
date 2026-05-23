import { useQuery } from "@tanstack/react-query";
import { downloadKeys } from "@/_lib/services/queries/download.queries";
import type { Download } from "@/_lib/types/download-types";

export function useJobProgress(
  jobId: string,                // always required
  initialDownload?: Download,
  enabled: boolean = true
) {
  return useQuery<Download>({
    queryKey: downloadKeys.detail(jobId),
    queryFn: () => Promise.resolve(initialDownload as Download),
    initialData: initialDownload,
    staleTime: Infinity,
    refetchOnMount: false,
    enabled,
  });
}