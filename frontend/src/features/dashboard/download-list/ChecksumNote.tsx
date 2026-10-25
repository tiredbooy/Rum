import { ShieldCheck, ShieldAlert } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { useRedownload } from "@/_lib/services/queries/download.queries";
import type { Download } from "@/_lib/types/download-types";

/** True when the file was compared with a checksum and did not match. */
export function isChecksumMismatch(d: Pick<Download, "checksum_status">) {
  return d.checksum_status === "mismatch";
}

const ALGO_LABEL = { sha256: "SHA-256", md5: "MD5" } as const;

/** Card line: "Verified (SHA-256)" or the damaged-file prompt. Nothing otherwise. */
export function ChecksumNote({ download }: { download: Download }) {
  const redownload = useRedownload();
  const { checksum_status: status, checksum_result_algo: algo } = download;
  if (status === "verified") {
    return (
      <p className="flex items-center gap-1 text-[11px] text-emerald-400">
        <ShieldCheck className="h-3 w-3" aria-hidden />
        Verified{algo ? ` (${ALGO_LABEL[algo] ?? algo})` : ""}
      </p>
    );
  }
  if (status === "mismatch") {
    return (
      <div className="flex items-center gap-2 text-[11px] text-destructive">
        <ShieldAlert className="h-3 w-3 shrink-0" aria-hidden />
        <span>File is damaged. Redownload?</span>
        <Button
          size="sm"
          variant="outline"
          className="h-6 px-2 text-[11px]"
          disabled={redownload.isPending}
          onClick={() => redownload.mutate(download.id)}
        >
          {redownload.isPending && <Spinner className="h-3 w-3" />}
          Redownload
        </Button>
      </div>
    );
  }
  return null;
}
