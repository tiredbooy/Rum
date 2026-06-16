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
import { toast } from "@/lib/toast";
import type {
  Download,
  DownloadPriority,
} from "@/_lib/types/download-types";
import { Copy, FolderOpen, Link2, MoreVertical } from "lucide-react";
import { priorityConfig } from "./PriorityBadge";

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
 * endpoint via onSetPriority), copy link/path quick actions, and an
 * "open folder" action that falls back to copying the path (no Wails binding
 * for revealing in the file manager exists yet).
 */
export function CardActionsMenu({
  download,
  onSetPriority,
}: CardActionsMenuProps) {
  const { id, url, dest_path, priority = "normal" } = download;

  const handleOpenFolder = () => {
    if (dest_path) {
      // No reveal-in-folder binding yet — copy the destination path instead.
      void copyToClipboard(dest_path, "Folder path");
    } else {
      toast.info("Destination path is not available yet");
    }
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7"
          aria-label="More actions"
          title="More actions"
        >
          <MoreVertical className="h-4 w-4" />
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
        <DropdownMenuItem className="gap-2" onClick={handleOpenFolder}>
          <FolderOpen className="h-3.5 w-3.5" />
          Open folder
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
