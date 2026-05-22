import { Button } from "@/components/ui/button";
import { ResponsiveDialog } from "@/features/reusable/dialog/DialogShell";
import { DownloadTabs } from "./DownloadDialogTabs";
import { Dispatch, SetStateAction } from "react";
import { useDownloadRequestStore } from "@/stores/download-request-store";
import { useCreateDownloads } from "@/_lib/services/queries/download.queries";

interface Props {
  open: boolean;
  setOpen: Dispatch<SetStateAction<boolean>>;
}

export default function DownloadDialog({ open, setOpen }: Props) {
  const draft = useDownloadRequestStore((s) => s.draft)
  const saveDownloads = useCreateDownloads()
  
  const handleSaveDownloads = () => {
    if (draft?.urls.length  )
    saveDownloads.mutateAsync(draft)
    setOpen(false);
  };

  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={setOpen}
      title="Add Downloads"
      size="2xl"
      description="Choose a method to add downloads."
      footer={
        <div className="flex gap-2 justify-end w-full">
          <Button variant="outline" onClick={() => setOpen(false)}>
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
