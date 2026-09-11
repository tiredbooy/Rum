import { useCallback, useEffect, useState } from "react";
import { chooseDir, hasChooseDir } from "@/_lib/wails";
import { toast } from "@/lib/toast";

/**
 * Native folder picker for the settings forms.
 *
 * Two things this fixes over the previous inline `hasChooseDir()` calls:
 *
 * 1. Availability is state, not a value frozen at first render. CategoryManager
 *    captured it in a `useRef`, so if the component mounted before the desktop
 *    shell injected `window.go`, Browse never appeared for the rest of the
 *    session.
 * 2. A picker that fails is no longer silent. `chooseDir` used to swallow every
 *    rejection into `null`, which is indistinguishable from a cancel — so a
 *    broken dialog looked exactly like a button that does nothing.
 */
export function useFolderPicker() {
  const [canPick, setCanPick] = useState(hasChooseDir);

  useEffect(() => {
    if (canPick) return;
    // The Wails runtime injects its bindings before the app module runs, but a
    // re-check on the next frame costs nothing and removes the race entirely.
    const id = window.setTimeout(() => setCanPick(hasChooseDir()), 250);
    return () => window.clearTimeout(id);
  }, [canPick]);

  const pick = useCallback(async (): Promise<string | null> => {
    const result = await chooseDir();
    switch (result.status) {
      case "picked":
        return result.path;
      case "cancelled":
        return null;
      case "unavailable":
        setCanPick(false);
        toast.error("Folder picker unavailable. Type a path instead.");
        return null;
      case "failed":
        toast.error(result.message || "Folder picker failed. Type a path.");
        return null;
    }
  }, []);

  return { canPick, pick };
}
