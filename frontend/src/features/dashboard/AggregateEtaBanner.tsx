import { Clock } from "lucide-react";
import { useAggregateEta } from "@/hooks/useAggregateEta";
import { formatBytes, formatETA, formatSpeed } from "@/_lib/utils/format";

/**
 * A slim, live "everything finishes in ~X" banner: combined remaining bytes over
 * the combined current speed of all running downloads. Renders nothing when no
 * download is running, so it never adds idle clutter. Uses the accent color, so it
 * follows a custom theme.
 */
export default function AggregateEtaBanner() {
  const { etaSeconds, totalSpeed, remainingBytes, activeCount, hasUnknownSize } =
    useAggregateEta();

  if (activeCount === 0) return null;

  return (
    <div
      className="flex flex-wrap items-center gap-x-2 gap-y-1 rounded-lg border border-primary/25 bg-primary/5 px-3 py-2 text-sm"
      aria-live="polite"
    >
      <Clock className="h-4 w-4 shrink-0 text-primary" />
      <span className="font-medium">
        {etaSeconds != null ? (
          <>
            All downloads finish in{" "}
            <span className="tabular-nums text-primary">
              {formatETA(etaSeconds)}
              {hasUnknownSize ? "+" : ""}
            </span>
          </>
        ) : (
          <span className="text-muted-foreground">Calculating finish time…</span>
        )}
      </span>
      <span className="text-muted-foreground">·</span>
      <span className="tabular-nums text-muted-foreground">
        {formatSpeed(totalSpeed)}
      </span>
      {remainingBytes > 0 && (
        <>
          <span className="text-muted-foreground">·</span>
          <span className="tabular-nums text-muted-foreground">
            {formatBytes(remainingBytes)} left
          </span>
        </>
      )}
      <span className="text-muted-foreground">· {activeCount} active</span>
    </div>
  );
}
