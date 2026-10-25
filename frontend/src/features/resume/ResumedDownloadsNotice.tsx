import { useEffect } from "react";
import { getResumedOnLaunch } from "@/_lib/wails";
import { toast } from "@/lib/toast";

/**
 * Tells the user that downloads cut off by the last exit or crash were picked
 * up again on launch. The engine resumes them before the window exists, so this
 * asks once on mount rather than listening for an event that would already be
 * gone.
 */
export function ResumedDownloadsNotice() {
  useEffect(() => {
    let cancelled = false;
    void getResumedOnLaunch().then((n) => {
      if (cancelled || n <= 0) return;
      toast(n === 1 ? "Resumed 1 download" : `Resumed ${n} downloads`);
    });
    return () => {
      cancelled = true;
    };
  }, []);

  return null;
}

export default ResumedDownloadsNotice;
