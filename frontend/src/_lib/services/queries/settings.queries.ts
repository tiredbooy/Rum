import { useCallback, useRef, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { getSettings, updateSettings } from "../api/settings-api";
import { ApiError } from "../api/api";
import { SettingReq } from "@/_lib/types/setting-types";
import { toast } from "@/lib/toast";

export const settingsKeys = {
  all: ["settings"] as const,
};

export function useSettings() {
  return useQuery<SettingReq>({
    queryKey: settingsKeys.all,
    queryFn: getSettings,
    staleTime: 1000 * 60 * 5, // cache for 5 min
    // The settings cards mirror text fields into local draft state and resync
    // from this cache when it changes. A window-focus refetch swaps in a fresh
    // cache object and would stomp a field the user is actively editing, so
    // disable it — every successful save already writes the merged result back
    // via setQueryData, keeping the cache fresh without a focus refetch.
    refetchOnWindowFocus: false,
  });
}

export function useUpdateSettings() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: Partial<SettingReq>) => updateSettings(data),
    onSuccess: (updated) => {
      queryClient.setQueryData(settingsKeys.all, updated);
    },
  });
}

/** How long the green "Saved" pill stays up after a successful write. */
const SAVED_FLASH_MS = 2000;

interface PatchOptions {
  /**
   * Called when the save fails, so a card can put its local draft back to the
   * value the server still holds. Without this the input kept showing a value
   * that was never persisted.
   */
  onError?: () => void;
}

/**
 * One place for the save-a-single-setting flow every settings card repeats:
 * fire the PATCH, flash "Saved" on success, and surface the failure — as an
 * inline message on the offending field when the backend named one, otherwise
 * as a toast.
 *
 * Returns `errorFor(field)` so a control can render its own message rather than
 * relying on a toast the user may have already dismissed.
 */
export function useSettingsPatch() {
  const mutation = useUpdateSettings();
  const [savedField, setSavedField] = useState<string | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const flash = useCallback((field: string) => {
    setSavedField(field);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(
      () => setSavedField((f) => (f === field ? null : f)),
      SAVED_FLASH_MS,
    );
  }, []);

  const clearError = useCallback((field: string) => {
    setErrors((prev) => {
      if (!(field in prev)) return prev;
      const next = { ...prev };
      delete next[field];
      return next;
    });
  }, []);

  // `mutation` is a fresh object every render, so depending on it would make
  // `patch` a new function on every render — harmless in an event handler, but a
  // render loop the moment someone puts it in a dependency array. `mutate` is
  // stable for the observer's lifetime, so depend on that instead.
  const { mutate } = mutation;

  const patch = useCallback(
    (field: string, payload: Partial<SettingReq>, opts?: PatchOptions) => {
      clearError(field);
      mutate(payload, {
        onSuccess: () => flash(field),
        onError: (err) => {
          const fieldMessage =
            err instanceof ApiError ? err.fieldError(field) : undefined;
          if (fieldMessage) {
            setErrors((prev) => ({ ...prev, [field]: fieldMessage }));
          } else {
            toast.error(
              err instanceof Error ? err.message : "Failed to save setting",
            );
          }
          opts?.onError?.();
        },
      });
    },
    [clearError, flash, mutate],
  );

  const errorFor = useCallback(
    (field: string): string | undefined => errors[field],
    [errors],
  );

  return { patch, savedField, errorFor, clearError, isSaving: mutation.isPending };
}
