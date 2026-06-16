import * as React from "react";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { WifiOff, X } from "lucide-react";
import { cn } from "@/lib/utils";

interface ConnectionBannerProps {
  /** Whether the live (SSE) connection is currently up. */
  online: boolean;
  /** Optional message override. */
  message?: string;
  className?: string;
}

/**
 * Dismissible banner shown when the live/SSE connection is lost.
 * Accessible: announced politely via `role="status"` + `aria-live`.
 * The data source is wired by F1/orchestrator; this component is presentational.
 *
 * Re-appears automatically if the connection drops again after being dismissed.
 */
export function ConnectionBanner({
  online,
  message = "Connection lost. Trying to reconnect…",
  className,
}: ConnectionBannerProps) {
  const reduceMotion = useReducedMotion();
  const [dismissed, setDismissed] = React.useState(false);

  // Reset the dismissed state whenever the connection comes back, so a future
  // disconnect shows the banner again.
  React.useEffect(() => {
    if (online) setDismissed(false);
  }, [online]);

  const visible = !online && !dismissed;

  return (
    <AnimatePresence>
      {visible && (
        <motion.div
          role="status"
          aria-live="polite"
          initial={reduceMotion ? false : { opacity: 0, y: -8 }}
          animate={{ opacity: 1, y: 0 }}
          exit={reduceMotion ? { opacity: 0 } : { opacity: 0, y: -8 }}
          transition={{ duration: reduceMotion ? 0 : 0.2 }}
          className={cn(
            "flex items-center gap-3 border-b border-warning/40 bg-warning/15 px-4 py-2 text-sm text-foreground",
            className
          )}
        >
          <WifiOff className="size-4 shrink-0 text-warning" aria-hidden="true" />
          <span className="flex-1">{message}</span>
          <button
            type="button"
            onClick={() => setDismissed(true)}
            aria-label="Dismiss connection notice"
            className="inline-flex size-8 shrink-0 cursor-pointer items-center justify-center rounded-md text-foreground/70 transition-colors hover:bg-warning/20 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1"
          >
            <X className="size-4" aria-hidden="true" />
          </button>
        </motion.div>
      )}
    </AnimatePresence>
  );
}

export default ConnectionBanner;
