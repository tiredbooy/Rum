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
  updateProgress: (id: string, data: Partial<ProgressEntry>) => void;
  setOnline: (online: boolean) => void;
  clearAll: () => void;
}

export const useProgressStore = create<ProgressState>((set) => ({
  progressMap: {},
  // Optimistic default: assume online until the stream reports otherwise, so the
  // banner does not flash on the very first render before the SSE connects.
  online: true,
  updateProgress: (id, data) =>
    set((state) => ({
      progressMap: {
        ...state.progressMap,
        [id]: { ...state.progressMap[id], ...data },
      },
    })),
  setOnline: (online) => set({ online }),
  clearAll: () => set({ progressMap: {} }),
}));
