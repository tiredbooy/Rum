import { Link2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { mirrorLabel } from "@/_lib/mirrors";
import type { Download } from "@/_lib/types/download-types";

/** Chip shown while a download is served by a mirror instead of its main link. */
export function MirrorBadge({
  download,
}: {
  download: Pick<Download, "active_mirror" | "status">;
}) {
  const label = mirrorLabel(download);
  if (!label) return null;
  return (
    <Badge
      variant="outline"
      className="max-w-40 gap-1 px-1.5 py-0 text-[10px] font-medium text-sky-500 border-sky-500/40 bg-sky-500/10"
      title={label}
    >
      <Link2 className="h-3 w-3 shrink-0" aria-hidden />
      <span className="truncate">{label}</span>
    </Badge>
  );
}

export default MirrorBadge;
