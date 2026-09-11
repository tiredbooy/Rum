import { useEffect, useRef } from "react";
import { onWailsEvent } from "@/_lib/wails";
import { useAddDialogStore } from "@/stores/add-dialog-store";
import { useDownloadRequestStore } from "@/stores/download-request-store";
import { toast } from "@/lib/toast";

/** Wails event emitted by the Go clipboard watcher (desktop.go). */
const CLIPBOARD_EVENT = "clipboard:url";

/**
 * Completes the "Watch clipboard for links" setting.
 *
 * The Go side polls the clipboard when the preference is on and emits a
 * `clipboard:url` event — but nothing in the frontend had ever subscribed to
 * it, so the switch persisted, said "Saved", and produced no visible behaviour
 * anywhere in the app.
 *
 * A copied link opens a toast with an Add action rather than popping the dialog
 * open by itself: copying a URL is not a request to download it, and a modal
 * that appears on every copy would be hostile.
 */
export function ClipboardLinkWatcher() {
  const openWith = useAddDialogStore((s) => s.openWith);
  // The Go watcher already de-dupes consecutive identical clipboard reads; this
  // guards against a re-mount replaying the same link.
  const lastUrl = useRef<string | null>(null);

  useEffect(() => {
    return onWailsEvent(CLIPBOARD_EVENT, (...data: unknown[]) => {
      const url = typeof data[0] === "string" ? data[0] : "";
      if (!url || url === lastUrl.current) return;
      lastUrl.current = url;

      toast("Link copied", {
        description: url,
        action: {
          label: "Add",
          onClick: () => {
            useDownloadRequestStore.getState().updateDraft({ urls: [url] });
            openWith({ tab: "single" });
          },
        },
      });
    });
  }, [openWith]);

  return null;
}

export default ClipboardLinkWatcher;
