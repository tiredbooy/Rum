import { DownloadStatus } from "@/_lib/types/download-types";
import { create } from "zustand";

export interface ProgressEntry {
  id: string;
  progress?: number;
  speed?: number;
  status?: DownloadStatus;
  totalSize?: number;
  remaining?: number;
}

interface ProgressState {
  progressMap: Record<string, ProgressEntry>;
  /** Whether the live (SSE) progress connection is currently up. */
  online: boolean;
  /**
   * Whether any tracked download is running/pending. The SSE stream is paused
   * (closed) while this is false to keep idle RAM/CPU low, and reopened when it
   * turns true. Defaults to true so the stream connects on first load.
   */
  hasActivity: boolean;
  updateProgress: (id: string, data: Partial<ProgressEntry>) => void;
  setOnline: (online: boolean) => void;
  setHasActivity: (hasActivity: boolean) => void;
  clearAll: () => void;
}

export const useProgressStore = create<ProgressState>((set) => ({
  progressMap: {},
  // Optimistic default: assume online until the stream reports otherwise, so the
  // banner does not flash on the very first render before the SSE connects.
  online: true,
  // Optimistic default: connect on first render; the controller recomputes this
  // from the download cache and pauses the stream once everything is idle.
  hasActivity: true,
  updateProgress: (id, data) =>
    set((state) => ({
      progressMap: {
        ...state.progressMap,
        [id]: { ...state.progressMap[id], ...data },
      },
    })),
  setOnline: (online) => set({ online }),
  setHasActivity: (hasActivity) => set({ hasActivity }),
  clearAll: () => set({ progressMap: {} }),
}));
