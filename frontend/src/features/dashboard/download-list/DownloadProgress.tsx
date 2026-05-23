interface DownloadProgressProps {
  progress: number; // 0-100
  speed: number; // e.g. "8.4 MB/s"
  eta?: string; // e.g. "2m 34s"
}

export function DownloadProgress({
  progress,
  speed,
  eta,
}: DownloadProgressProps) {
  const clamped = Math.min(100, Math.max(0, progress));
  return (
    <div className="w-full space-y-1">
      <div className="flex justify-between text-xs text-muted-foreground">
        <span>{clamped}%</span>
        <span>
          {speed} • {eta} left
        </span>
      </div>
      <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
        <div
          className="h-full rounded-full bg-primary transition-all duration-500"
          style={{ width: `${clamped}%` }}
        />
      </div>
    </div>
  );
}
