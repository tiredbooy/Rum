import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Spinner } from "@/components/ui/spinner";
import { toast } from "@/lib/toast";
import type {
  Download,
  DownloadPriority,
} from "@/_lib/types/download-types";
import {
  useRepairDownload,
  useVerifyChecksum,
  useVerifyDownload,
} from "@/_lib/services/queries/download.queries";
import {
  Copy,
  FileCheck2,
  FolderOpen,
  Link2,
  Layers,
  MoreVertical,
  Trash2,
  ShieldCheck,
  Wrench,
} from "lucide-react";
import { useUndoableDelete } from "@/_lib/services/queries/undo-delete.queries";
import { priorityConfig } from "./PriorityBadge";
import { openActionsFor, showInFolder } from "@/_lib/open-download";
import { EditMirrorsDialog } from "@/features/dashboard/mirrors/EditMirrorsDialog";

interface CardActionsMenuProps {
  download: Download;
  onSetPriority?: (id: string, priority: DownloadPriority) => void;
}

const PRIORITIES: DownloadPriority[] = ["high", "normal", "low"];

async function copyToClipboard(text: string, label: string) {
  try {
    await navigator.clipboard.writeText(text);
    toast.success(`${label} copied to clipboard`);
  } catch {
    toast.error(`Could not copy ${label.toLowerCase()}`);
  }
}

/**
 * Per-card overflow menu: priority control (radio group → calls the priority
 * endpoint via onSetPriority), copy link/path quick actions, an "open folder"
 * action that falls back to copying the path (no Wails binding for revealing in
 * the file manager exists yet), and — for completed downloads — integrity
 * Verify / Repair actions.
 *
 * Verify is read-only and always safe; if it reports the file is *not* ok we
 * surface a Repair affordance (the toast handlers in the mutations summarize the
 * VerifyResult). Repair re-fetches bad segments and re-verifies.
 */
export function CardActionsMenu({
  download,
  onSetPriority,
}: CardActionsMenuProps) {
  const { id, url, dest_path, priority = "normal", status } = download;

  const [mirrorsOpen, setMirrorsOpen] = useState(false);
  const verify = useVerifyDownload();
  const repair = useRepairDownload();
  const checksum = useVerifyChecksum();
  const { removeIds } = useUndoableDelete();
  const isCompleted = status === "completed";
  const busy = verify.isPending || repair.isPending || checksum.isPending;

  const handleOpenFolder = () => {
    if (openActionsFor(download).folder) {
      void showInFolder(download).then((r) => {
        if (r.status === "failed") toast.error("Can't open it. Check the file still exists.");
      });
    } else if (dest_path) {
      // No reveal-in-folder binding yet — copy the destination path instead.
      void copyToClipboard(dest_path, "Folder path");
    } else {
      toast.info("Destination path is not available yet");
    }
  };

  const handleVerify = () => {
    // The mutation's onSuccess already toasts the VerifyResult. If verify
    // reports not-ok, offer Repair as a one-click follow-up action.
    verify.mutate(
      { id },
      {
        onSuccess: (res) => {
          if (!res.ok) {
            toast.error("File failed verification", {
              action: {
                label: "Repair",
                onClick: () => repair.mutate({ id }),
              },
            });
          }
        },
      },
    );
  };

  return (
    <>
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7"
          aria-label="More actions"
          title="More actions"
        >
          {busy ? (
            <Spinner className="h-4 w-4" />
          ) : (
            <MoreVertical className="h-4 w-4" />
          )}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-44">
        <DropdownMenuLabel className="text-[11px] text-muted-foreground">
          Priority
        </DropdownMenuLabel>
        <DropdownMenuRadioGroup
          value={priority}
          onValueChange={(v) =>
            onSetPriority?.(id, v as DownloadPriority)
          }
        >
          {PRIORITIES.map((p) => {
            const { label, icon: Icon } = priorityConfig[p];
            return (
              <DropdownMenuRadioItem key={p} value={p} className="gap-2">
                <Icon className="h-3.5 w-3.5" />
                {label}
              </DropdownMenuRadioItem>
            );
          })}
        </DropdownMenuRadioGroup>

        {isCompleted && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuLabel className="text-[11px] text-muted-foreground">
              Integrity
            </DropdownMenuLabel>
            <DropdownMenuItem
              className="gap-2"
              disabled={busy}
              onSelect={(e) => {
                e.preventDefault();
                handleVerify();
              }}
            >
              {verify.isPending ? (
                <Spinner className="h-3.5 w-3.5" />
              ) : (
                <ShieldCheck className="h-3.5 w-3.5" />
              )}
              Verify integrity
            </DropdownMenuItem>
            <DropdownMenuItem
              className="gap-2"
              disabled={busy}
              onSelect={(e) => {
                e.preventDefault();
                checksum.mutate(id);
              }}
            >
              {checksum.isPending ? (
                <Spinner className="h-3.5 w-3.5" />
              ) : (
                <FileCheck2 className="h-3.5 w-3.5" />
              )}
              Verify checksum
            </DropdownMenuItem>
            <DropdownMenuItem
              className="gap-2"
              disabled={busy}
              onSelect={(e) => {
                e.preventDefault();
                repair.mutate({ id });
              }}
            >
              {repair.isPending ? (
                <Spinner className="h-3.5 w-3.5" />
              ) : (
                <Wrench className="h-3.5 w-3.5" />
              )}
              Repair
            </DropdownMenuItem>
          </>
        )}

        <DropdownMenuSeparator />

        <DropdownMenuItem
          className="gap-2"
          onClick={() => copyToClipboard(url, "Link")}
        >
          <Link2 className="h-3.5 w-3.5" />
          Copy link
        </DropdownMenuItem>
        {dest_path && (
          <DropdownMenuItem
            className="gap-2"
            onClick={() => copyToClipboard(dest_path, "Path")}
          >
            <Copy className="h-3.5 w-3.5" />
            Copy path
          </DropdownMenuItem>
        )}
        {!isCompleted && (
          <DropdownMenuItem className="gap-2" onClick={() => setMirrorsOpen(true)}>
            <Layers className="h-3.5 w-3.5" />
            Mirrors
          </DropdownMenuItem>
        )}
        <DropdownMenuItem className="gap-2" onClick={handleOpenFolder}>
          <FolderOpen className="h-3.5 w-3.5" />
          Open folder
        </DropdownMenuItem>

        <DropdownMenuSeparator />
        <DropdownMenuItem
          className="gap-2 text-destructive focus:text-destructive"
          onClick={() => void removeIds([id], true)}
        >
          <Trash2 className="h-3.5 w-3.5" />
          Delete file
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
    <EditMirrorsDialog
      download={download}
      open={mirrorsOpen}
      onOpenChange={setMirrorsOpen}
    />
    </>
  );
}
