import { Button } from "@/components/ui/button";
import {
  useDownloads,
  useRetryFailedDownloads,
} from "@/_lib/services/queries/download.queries";
import { RotateCw } from "lucide-react";

interface RetryAllButtonProps {
  size?: "sm" | "default";
  variant?: "outline" | "default";
}

/** Retries every failed download. Renders nothing when none have failed. */
export function RetryAllButton({ size = "sm", variant = "outline" }: RetryAllButtonProps) {
  const { data: failed } = useDownloads("error");
  const retry = useRetryFailedDownloads();
  const count = failed?.length ?? 0;
  if (count === 0) return null;

  return (
    <Button
      variant={variant}
      size={size}
      onClick={() => retry.mutate()}
      disabled={retry.isPending}
      aria-label={`Retry all ${count} failed downloads`}
      className="gap-1.5 h-9"
    >
      <RotateCw className="w-3.5 h-3.5" aria-hidden="true" />
      <span>Retry all ({count})</span>
    </Button>
  );
}
