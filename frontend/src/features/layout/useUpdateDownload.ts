import { ApiError } from "@/_lib/services/api/api";
import { useDownloadUpdate } from "@/_lib/services/queries/update.queries";
import type { UpdateStatus } from "@/_lib/update-notice";
import { describeDownloadError } from "@/_lib/utils/download-errors";
import { openInBrowser } from "@/_lib/wails";
import { toast } from "@/lib/toast";

/** Opens the release page: system browser in the desktop shell, a tab otherwise. */
export function openReleasePage(url?: string) {
  if (!url) return;
  if (!openInBrowser(url)) window.open(url, "_blank", "noopener,noreferrer");
}

/**
 * "Download": saves + verifies the installer for this system. When that is not
 * possible (no build for this OS, server unreachable, failed check) it says why
 * and opens the release page so the user is never stuck.
 */
export function useUpdateDownload(status: UpdateStatus | undefined) {
  const mutation = useDownloadUpdate();

  const start = () =>
    mutation.mutate(undefined, {
      onSuccess: () => toast.success("Update saved and verified"),
      onError: (err) => {
        const code = err instanceof ApiError ? err.code : undefined;
        const { message, action } = describeDownloadError(
          code === "update_verify_failed" || code === "update_check_failed" ? code : "update_check_failed",
        );
        toast.error(`${message} ${action}`);
        openReleasePage(status?.release_url);
      },
    });

  return { start, busy: mutation.isPending };
}
