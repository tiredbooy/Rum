import { Button } from "@/components/ui/button";
import { ResponsiveDialog } from "@/features/reusable/dialog/DialogShell";
import { DownloadTabs } from "./DownloadDialogTabs";
import { useDownloadRequestStore } from "@/stores/download-request-store";
import { useCreateDownloads } from "@/_lib/services/queries/download.queries";
import { useAddDialogStore } from "@/stores/add-dialog-store";

export default function DownloadDialog() {
  const open = useAddDialogStore((s) => s.open);
  const setOpen = useAddDialogStore((s) => s.setOpen);
  const draft = useDownloadRequestStore((s) => s.draft);
  const resetDraft = useDownloadRequestStore((s) => s.resetDraft);
  const saveDownloads = useCreateDownloads();

  // Clear the draft whenever the dialog closes so the next open starts fresh
  // instead of re-showing the previous download's URLs/folder.
  const handleOpenChange = (next: boolean) => {
    setOpen(next);
    if (!next) resetDraft();
  };

  const handleSaveDownloads = () => {
    if (draft?.urls?.length) saveDownloads.mutateAsync(draft);
    setOpen(false);
    resetDraft();
  };

  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={handleOpenChange}
      title="Add Downloads"
      size="2xl"
      description="Choose a method to add downloads."
      footer={
        <div className="flex gap-2 justify-end w-full">
          <Button variant="outline" onClick={() => handleOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleSaveDownloads}>Save</Button>
        </div>
      }
    >
      <DownloadTabs />
    </ResponsiveDialog>
  );
}
