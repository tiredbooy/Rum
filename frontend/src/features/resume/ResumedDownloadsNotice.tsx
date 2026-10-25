import { useEffect } from "react";
import { getJobsRestored, getResumedOnLaunch } from "@/_lib/wails";
import { toast } from "@/lib/toast";

/**
 * Tells the user that downloads cut off by the last exit or crash were picked
 * up again on launch. The engine resumes them before the window exists, so this
 * asks once on mount rather than listening for an event that would already be
 * gone.
 */
// The engine hands the count out once, so the request must be made once per
// page load: StrictMode mounts twice in dev and the second ask would get 0.
let resumedCount: Promise<number> | null = null;
let jobsRestored: Promise<boolean> | null = null;

export function ResumedDownloadsNotice() {
  useEffect(() => {
    let cancelled = false;
    resumedCount ??= getResumedOnLaunch();
    void resumedCount.then((n) => {
      if (cancelled || n <= 0) return;
      toast(n === 1 ? "Resumed 1 download" : `Resumed ${n} downloads`);
    });
    // Same once-per-launch hand-out: the list was rebuilt from its backup.
    jobsRestored ??= getJobsRestored();
    void jobsRestored.then((restored) => {
      if (cancelled || !restored) return;
      toast.warning("Download list restored from backup");
    });
    return () => {
      cancelled = true;
    };
  }, []);

  return null;
}

export default ResumedDownloadsNotice;
