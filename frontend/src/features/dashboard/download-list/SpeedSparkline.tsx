import { cn } from "@/lib/utils";
import { formatSpeed } from "@/_lib/utils/format";

interface SpeedSparklineProps {
  /** Recent speed samples in bytes/sec (oldest → newest). */
  samples: number[];
  width?: number;
  height?: number;
  /** Show a trailing current-speed label (latest sample). Default false. */
  showLabel?: boolean;
  className?: string;
}

/**
 * Tiny, dependency-free SVG sparkline of recent download speed. Kept cheap:
 * a single polyline + a soft area fill, normalized to the sample window's max.
 * Draws a flat baseline when idle (all-zero/too few points) so the slot keeps
 * a stable footprint instead of popping in and out.
 */
export function SpeedSparkline({
  samples,
  width = 64,
  height = 18,
  showLabel = false,
  className,
}: SpeedSparklineProps) {
  const latest = samples.length > 0 ? samples[samples.length - 1] : 0;
  const max = Math.max(...samples, 1);
  const flat = samples.length < 2 || max <= 0;

  const stepX = flat ? 0 : width / (samples.length - 1);
  const baseline = height - 1;

  const linePath = flat
    ? `M0,${baseline} L${width},${baseline}`
    : samples
        .map((s, i) => {
          const x = i * stepX;
          // Leave a 1px top/bottom margin so the stroke isn't clipped.
          const y = baseline - (s / max) * (height - 2);
          return `${i === 0 ? "M" : "L"}${x.toFixed(1)},${y.toFixed(1)}`;
        })
        .join(" ");

  const areaPath = `${linePath} L${width},${height} L0,${height} Z`;

  return (
    <span className={cn("inline-flex items-center gap-1.5", className)}>
      <svg
        width={width}
        height={height}
        viewBox={`0 0 ${width} ${height}`}
        preserveAspectRatio="none"
        className="text-cyan-400 shrink-0"
        role="img"
        aria-label="Recent speed trend"
      >
        {!flat && <path d={areaPath} fill="currentColor" opacity={0.12} />}
        <path
          d={linePath}
          fill="none"
          stroke="currentColor"
          strokeWidth={1.25}
          strokeLinejoin="round"
          strokeLinecap="round"
          opacity={flat ? 0.4 : 1}
        />
      </svg>
      {showLabel && (
        <span className="tabular-nums">{formatSpeed(latest)}</span>
      )}
    </span>
  );
}
