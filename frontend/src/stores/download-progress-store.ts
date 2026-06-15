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
  updateProgress: (id: string, data: Partial<ProgressEntry>) => void;
  clearAll: () => void;
}

export const useProgressStore = create<ProgressState>((set) => ({
  progressMap: {},
  updateProgress: (id, data) =>
    set((state) => ({
      progressMap: {
        ...state.progressMap,
        [id]: { ...state.progressMap[id], ...data },
      },
    })),
  clearAll: () => set({ progressMap: {} }),
}));
