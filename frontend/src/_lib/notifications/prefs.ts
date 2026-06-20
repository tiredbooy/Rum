/**
 * Client-side notification preferences. The completion sound is a purely local
 * preference (no backend field), persisted in localStorage. Kept tiny and
 * resilient: every access is wrapped so a disabled/unavailable storage (private
 * mode, embedded webview) never throws.
 */

const SOUND_KEY = "rum.notifications.completionSound";

/** Whether the optional completion sound is enabled. Default: false (opt-in). */
export function getCompletionSoundEnabled(): boolean {
  try {
    return window.localStorage.getItem(SOUND_KEY) === "1";
  } catch {
    return false;
  }
}

/** Persist the completion-sound toggle. No-op if storage is unavailable. */
export function setCompletionSoundEnabled(enabled: boolean): void {
  try {
    window.localStorage.setItem(SOUND_KEY, enabled ? "1" : "0");
  } catch {
    // Ignore — preference simply won't persist in this environment.
  }
  // Notify same-tab listeners (the `storage` event only fires cross-tab).
  try {
    window.dispatchEvent(
      new CustomEvent(SOUND_PREF_EVENT, { detail: enabled }),
    );
  } catch {
    // Ignore in non-DOM environments.
  }
}

/** Event name dispatched on same-tab changes so the settings switch stays in sync. */
export const SOUND_PREF_EVENT = "rum:completion-sound-changed";
export const SOUND_PREF_STORAGE_KEY = SOUND_KEY;
