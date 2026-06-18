import { useEffect, useState } from "react";
import { extractUrlsFromDataTransfer } from "./extract-links";
import { useAddDialogStore } from "@/stores/add-dialog-store";
import { toast } from "@/lib/toast";

// Global browser-drag capture. Listens at the window level, prevents the
// webview from navigating to a dropped URL, extracts links, and opens the Add
// dialog (Batch tab) pre-filled. Renders only a drag overlay; no layout impact.
export function DropProvider() {
  const openWith = useAddDialogStore((s) => s.openWith);
  const [dragging, setDragging] = useState(false);

  useEffect(() => {
    let depth = 0;
    const onEnter = (e: DragEvent) => {
      e.preventDefault();
      depth++;
      setDragging(true);
    };
    const onOver = (e: DragEvent) => {
      e.preventDefault();
    };
    const onLeave = (e: DragEvent) => {
      e.preventDefault();
      depth = Math.max(0, depth - 1);
      if (depth === 0) setDragging(false);
    };
    const onDrop = (e: DragEvent) => {
      e.preventDefault();
      depth = 0;
      setDragging(false);
      if (!e.dataTransfer) return;
      const urls = extractUrlsFromDataTransfer(e.dataTransfer);
      if (!urls.length) {
        toast.error("No links found in the dropped content.");
        return;
      }
      openWith({ tab: "batch", urls });
    };
    window.addEventListener("dragenter", onEnter);
    window.addEventListener("dragover", onOver);
    window.addEventListener("dragleave", onLeave);
    window.addEventListener("drop", onDrop);
    return () => {
      window.removeEventListener("dragenter", onEnter);
      window.removeEventListener("dragover", onOver);
      window.removeEventListener("dragleave", onLeave);
      window.removeEventListener("drop", onDrop);
    };
  }, [openWith]);

  if (!dragging) return null;
  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center bg-background/70 backdrop-blur-sm pointer-events-none">
      <div className="rounded-xl border-2 border-dashed border-primary px-8 py-6 text-lg font-medium text-primary">
        Drop links to add to Rum
      </div>
    </div>
  );
}

export default DropProvider;
