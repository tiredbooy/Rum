import type { Download } from "@/_lib/types/download-types";
import { isDiskFull } from "@/_lib/disk-full";
import * as App from "../../wailsjs/go/main/App";

/**
 * Open-folder / open-file actions backed by the desktop shell. The bindings
 * take a download id (the shell looks the path up itself), and only exist in
 * the Wails webview, so in a plain browser every action is simply absent.
 */

export type OpenResult =
  | { status: "ok" }
  | { status: "unavailable" }
  | { status: "failed"; message: string };

type OpenBinding = "OpenDownloadFolder" | "RevealDownload" | "OpenDownload";

function hasBinding(name: OpenBinding): boolean {
  return typeof window !== "undefined" && typeof window.go?.main?.App?.[name] === "function";
}

/** Which actions to render for a download in the current environment. */
export function openActionsFor(d: Pick<Download, "status" | "error_code">): {
  folder: boolean;
  open: boolean;
} {
  const done = d.status === "completed";
  return {
    folder: (done || isDiskFull(d)) && hasBinding("OpenDownloadFolder"),
    open: done && hasBinding("OpenDownload"),
  };
}

async function run(name: OpenBinding, id: string): Promise<OpenResult> {
  if (!hasBinding(name)) return { status: "unavailable" };
  try {
    await App[name](id);
    return { status: "ok" };
  } catch (err) {
    // Wails rejects with a plain string; keep that readable.
    return {
      status: "failed",
      message: err instanceof Error ? err.message : String(err),
    };
  }
}

export const openDownloadFolder = (id: string) => run("OpenDownloadFolder", id);
export const revealDownload = (id: string) => run("RevealDownload", id);
export const openDownload = (id: string) => run("OpenDownload", id);

/** "Open folder": select the file for finished downloads, else open the folder. */
export function showInFolder(d: Pick<Download, "id" | "status">): Promise<OpenResult> {
  return d.status === "completed" && hasBinding("RevealDownload")
    ? revealDownload(d.id)
    : openDownloadFolder(d.id);
}
