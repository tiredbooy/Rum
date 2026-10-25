import { useEffect } from "react";
import {
  useCheckForUpdate,
  useDismissUpdate,
  useUpdateStatus,
} from "@/_lib/services/queries/update.queries";
import { showUpdateBanner, updateBannerText } from "@/_lib/update-notice";
import { UpdateBanner } from "@/components/update-banner";
import { useUpdateDownload } from "./useUpdateDownload";

/**
 * Runs the startup update check (the backend enforces the setting and the
 * once-a-day limit) and shows the "Rum vX available" notice. A failed check is
 * silent here; Settings shows it.
 */
export function UpdateNotice() {
  const { data: status } = useUpdateStatus();
  const { mutate: check } = useCheckForUpdate();
  const dismiss = useDismissUpdate();
  const { start, busy } = useUpdateDownload(status);

  useEffect(() => {
    check(true);
  }, [check]);

  if (!status || !showUpdateBanner(status)) return null;
  return (
    <UpdateBanner
      text={updateBannerText(status)}
      busy={busy}
      onDownload={start}
      onLater={() => status.tag && dismiss.mutate(status.tag)}
    />
  );
}

export default UpdateNotice;
