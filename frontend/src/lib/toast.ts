import { toast as sonnerToast } from "sonner";

/**
 * Thin wrapper around `sonner`'s toast so the rest of the app imports a single,
 * stable surface from `@/lib/toast` instead of reaching into `sonner` directly.
 *
 * Frozen contract (consumed by F1): at minimum `.success`, `.error`, `.info`,
 * `.loading`, and `.dismiss`. We re-export the full sonner `toast` so callers
 * may also use `.warning`, `.message`, `.promise`, `.custom`, etc.
 */
export const toast = sonnerToast;

/** Convenience standalone re-export of the dismiss helper. */
export const dismiss = sonnerToast.dismiss;

export type { ExternalToast } from "sonner";
