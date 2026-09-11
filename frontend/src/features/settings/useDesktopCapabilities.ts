import { useEffect, useState } from "react";
import {
  getCapabilities,
  hasWailsRuntime,
  type DesktopCapabilities,
} from "@/_lib/wails";

interface Result extends DesktopCapabilities {
  /** Whether we are running inside the Wails desktop shell at all. */
  isDesktop: boolean;
}

/**
 * Platform feature matrix from the App.Capabilities() Wails binding.
 *
 * The settings UI used to describe platform limits in prose ("Windows only in
 * this build") next to switches that saved happily and then did nothing. Asking
 * the shell what it can actually do lets the affected controls be disabled
 * instead.
 */
export function useDesktopCapabilities(): Result {
  const [caps, setCaps] = useState<DesktopCapabilities>({
    tray: false,
    folderPicker: false,
    platform: "web",
  });
  const [isDesktop, setIsDesktop] = useState(hasWailsRuntime);

  useEffect(() => {
    let alive = true;
    void getCapabilities().then((next) => {
      if (!alive) return;
      setCaps(next);
      setIsDesktop(hasWailsRuntime());
    });
    return () => {
      alive = false;
    };
  }, []);

  return { ...caps, isDesktop };
}
