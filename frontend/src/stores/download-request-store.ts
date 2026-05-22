import { create } from "zustand";
import type { DownloadReq } from "@/_lib/types/download-types";

interface DownloadRequestState {
  requests: DownloadReq[];

  draft: DownloadReq;
  updateDraft: (partial: Partial<DownloadReq>) => void;
  resetDraft: () => void;

  // Actions
  addRequest: (req: DownloadReq) => void;
  clearRequests: () => void;
}

const emptyDraft: DownloadReq = { urls: [] };

export const useDownloadRequestStore = create<DownloadRequestState>((set) => ({
  requests: [],
  draft: emptyDraft,

  updateDraft: (partial) =>
    set((state) => ({ draft: { ...state.draft, ...partial } })),

  resetDraft: () => set({ draft: emptyDraft }),

  addRequest: (req) => set((state) => ({ requests: [...state.requests, req] })),

  clearRequests: () => set({ requests: [] }),
}));
