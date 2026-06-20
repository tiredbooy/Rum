import { create } from "zustand";

/**
 * Selection store for bulk operations on the download list.
 *
 * Framework-light by design: a single Zustand store holding a `Set<string>` of
 * selected ids plus the last-clicked id (the anchor for shift-range selection).
 * Range selection takes the *ordered* id list from the rendered list so the
 * store never has to know about sorting/filtering — the view owns order.
 *
 * Consumers should read `selectedIds` (a Set) for membership checks and the
 * stable action callbacks (Zustand keeps action identities stable across
 * renders, so destructuring them does not cause re-render storms).
 */
interface SelectionState {
  /** Currently selected download ids. */
  selectedIds: Set<string>;
  /** Anchor id for shift-range selection (the last plain/ctrl click). */
  lastClickedId: string | null;

  /** Toggle a single id; updates the range anchor to this id. */
  toggle: (id: string) => void;
  /**
   * Select the contiguous range between the current anchor and `id`, using the
   * caller-supplied ordered id list (the order as rendered). If there is no
   * anchor yet, behaves like a single select. Adds to the existing selection.
   */
  selectRange: (id: string, orderedIds: string[]) => void;
  /** Replace the selection with exactly these ids. */
  selectAll: (ids: string[]) => void;
  /**
   * Add the ids matching a predicate over the provided items to the selection.
   * Pragmatic helper for "select all failed", "select all paused", etc.
   */
  selectByFilter: <T extends { id: string }>(
    items: T[],
    predicate: (item: T) => boolean,
  ) => void;
  /** Set membership of a single id explicitly (used by the checkbox). */
  setSelected: (id: string, selected: boolean) => void;
  /** Clear the entire selection. */
  clear: () => void;
}

export const useSelection = create<SelectionState>((set) => ({
  selectedIds: new Set<string>(),
  lastClickedId: null,

  toggle: (id) =>
    set((state) => {
      const next = new Set(state.selectedIds);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return { selectedIds: next, lastClickedId: id };
    }),

  selectRange: (id, orderedIds) =>
    set((state) => {
      const anchor = state.lastClickedId;
      // No anchor (or anchor no longer present) → treat as a single add.
      const anchorIdx = anchor ? orderedIds.indexOf(anchor) : -1;
      const targetIdx = orderedIds.indexOf(id);
      if (targetIdx === -1) return state;
      const next = new Set(state.selectedIds);
      if (anchorIdx === -1) {
        next.add(id);
        return { selectedIds: next, lastClickedId: id };
      }
      const [start, end] =
        anchorIdx < targetIdx
          ? [anchorIdx, targetIdx]
          : [targetIdx, anchorIdx];
      for (let i = start; i <= end; i++) next.add(orderedIds[i]);
      // Keep the original anchor so further shift-clicks extend from it.
      return { selectedIds: next, lastClickedId: anchor };
    }),

  selectAll: (ids) =>
    set({ selectedIds: new Set(ids), lastClickedId: ids.at(-1) ?? null }),

  selectByFilter: (items, predicate) =>
    set((state) => {
      const next = new Set(state.selectedIds);
      for (const item of items) if (predicate(item)) next.add(item.id);
      return { selectedIds: next };
    }),

  setSelected: (id, selected) =>
    set((state) => {
      const next = new Set(state.selectedIds);
      if (selected) next.add(id);
      else next.delete(id);
      return { selectedIds: next, lastClickedId: id };
    }),

  clear: () => set({ selectedIds: new Set<string>(), lastClickedId: null }),
}));
