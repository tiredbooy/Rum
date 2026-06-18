import { create } from "zustand";

export type AddTab = "single" | "bulk" | "url" | "batch";

interface AddDialogState {
  open: boolean;
  initialTab: AddTab;
  droppedUrls: string[] | null;
  openWith: (opts?: { tab?: AddTab; urls?: string[] }) => void;
  setOpen: (open: boolean) => void;
  consumeDropped: () => string[] | null;
}

export const useAddDialogStore = create<AddDialogState>((set, get) => ({
  open: false,
  initialTab: "single",
  droppedUrls: null,
  openWith: (opts) =>
    set({
      open: true,
      initialTab: opts?.tab ?? "single",
      droppedUrls: opts?.urls ?? null,
    }),
  setOpen: (open) => set({ open }),
  consumeDropped: () => {
    const urls = get().droppedUrls;
    if (urls) set({ droppedUrls: null });
    return urls;
  },
}));
