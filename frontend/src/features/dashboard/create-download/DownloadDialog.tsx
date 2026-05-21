
import { ResponsiveDialog } from "@/features/reusable/dialog/DialogShell";
import { DownloadTabs } from "./DownloadDialogTabs";
import { useState } from "react";
import { Button } from "@/components/ui/button";

interface Props {

}

export default function DownloadDialog({}: Props) {
  const [open, setOpen] = useState(true);

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
          <Button onClick={() => setOpen(false)}>Save</Button>
        </div>
      }
    >
      <DownloadTabs />
    </ResponsiveDialog>
  );
}
