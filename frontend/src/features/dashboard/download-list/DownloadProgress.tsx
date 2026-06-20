import { cn } from "@/lib/utils";
import { useSmoothProgress } from "@/hooks/useSmoothProgress";
import { useAppReducedMotion } from "@/_lib/theme";

interface DownloadProgressProps {
  /** Real progress target from SSE, 0-100. */
  progress: number;
  /** Whether the bar should animate (download actively running). */
  active?: boolean;
  /**
   * True while running but no recent byte movement (speed ~0) — shows an
   * indeterminate shimmer so the user always sees life. Ignored unless active.
   */
  stalled?: boolean;
  /**
   * Optional override forwarded to {@link useSmoothProgress}. When omitted the
   * bar honors the app `reduced_motion` setting (which itself falls back to the
   * OS `prefers-reduced-motion`). When true, motion is disabled and the bar
   * tracks the raw target via CSS only.
   */
  reducedMotion?: boolean;
  className?: string;
}

/**
 * Accessible, buttery progress bar. The visible width is driven by a
 * client-side interpolated value (so it glides between sparse SSE ticks),
 * while the ARIA value reports the *real* target for assistive tech.
 */
export function DownloadProgress({
  progress,
  active = false,
  stalled = false,
  reducedMotion,
  className,
}: DownloadProgressProps) {
  const target = Math.min(100, Math.max(0, progress));
  // Prefer an explicit prop, else honor the app `reduced_motion` setting (which
  // itself falls back to the OS `prefers-reduced-motion` preference).
  const appReducedMotion = useAppReducedMotion();
  const displayed = useSmoothProgress(target, active, {
    reducedMotion: reducedMotion ?? appReducedMotion,
  });
  const showShimmer = active && stalled;

  return (
    <div
      className={cn(
        "relative h-1.5 w-full overflow-hidden rounded-full bg-primary/20",
        className,
      )}
      role="progressbar"
      aria-valuenow={Math.round(target)}
      aria-valuemin={0}
      aria-valuemax={100}
    >
      <div
        className="h-full rounded-full bg-gradient-to-r from-primary to-accent transition-[width] duration-300 ease-out"
        style={{ width: `${displayed}%` }}
      />
      {showShimmer && (
        <div
          aria-hidden
          className="pointer-events-none absolute inset-0 -translate-x-full animate-[shimmer_1.6s_ease-in-out_infinite] bg-gradient-to-r from-transparent via-white/25 to-transparent"
        />
      )}
    </div>
  );
}
