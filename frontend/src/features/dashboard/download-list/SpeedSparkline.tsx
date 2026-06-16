import { cn } from "@/lib/utils";

interface SpeedSparklineProps {
  /** Recent speed samples in bytes/sec (oldest → newest). */
  samples: number[];
  width?: number;
  height?: number;
  className?: string;
}

/**
 * Tiny, dependency-free SVG sparkline of recent download speed. Kept cheap:
 * a single polyline + a soft area fill, normalized to the sample window's max.
 * Render nothing until there are at least two points to draw a line.
 */
export function SpeedSparkline({
  samples,
  width = 64,
  height = 18,
  className,
}: SpeedSparklineProps) {
  if (samples.length < 2) {
    return <div style={{ width, height }} className={className} aria-hidden />;
  }

  const max = Math.max(...samples, 1);
  const stepX = width / (samples.length - 1);

  const points = samples.map((s, i) => {
    const x = i * stepX;
    // Leave a 1px top/bottom margin so the stroke isn't clipped.
    const y = height - 1 - (s / max) * (height - 2);
    return [x, y] as const;
  });

  const linePath = points
    .map(([x, y], i) => `${i === 0 ? "M" : "L"}${x.toFixed(1)},${y.toFixed(1)}`)
    .join(" ");
  const areaPath = `${linePath} L${width},${height} L0,${height} Z`;

  return (
    <svg
      width={width}
      height={height}
      viewBox={`0 0 ${width} ${height}`}
      preserveAspectRatio="none"
      className={cn("text-cyan-400 shrink-0", className)}
      role="img"
      aria-label="Recent speed trend"
    >
      <path d={areaPath} fill="currentColor" opacity={0.12} />
      <path
        d={linePath}
        fill="none"
        stroke="currentColor"
        strokeWidth={1.25}
        strokeLinejoin="round"
        strokeLinecap="round"
      />
    </svg>
  );
}
