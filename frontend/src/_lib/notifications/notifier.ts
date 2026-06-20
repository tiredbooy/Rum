/**
 * Desktop-notification + completion-sound primitives. Fully defensive: a missing
 * Notification API, a denied permission, an absent native binding, or a blocked
 * audio play() must never throw into the caller (the download watcher).
 *
 * Native path: if the Wails shell ever injects a `Notify(title, body)` binding
 * on `window.go.main.App`, we prefer it (true OS toast, app-icon, no web
 * permission prompt). Otherwise we fall back to the browser Notification API.
 */
import completeSoundUrl from "@/assets/complete.wav";
import { getCompletionSoundEnabled } from "./prefs";

interface WailsNotifyApp {
  Notify?: (title: string, body: string) => void | Promise<void>;
}

function nativeNotify(): WailsNotifyApp["Notify"] | undefined {
  const fn = (window.go?.main?.App as WailsNotifyApp | undefined)?.Notify;
  return typeof fn === "function" ? fn : undefined;
}

/** Whether the browser Notification API exists in this runtime. */
function hasWebNotifications(): boolean {
  return typeof window !== "undefined" && "Notification" in window;
}

/**
 * Request notification permission once, lazily. Returns the resulting
 * permission. Never throws. Calling this from a user gesture (e.g. toggling a
 * setting) gives the best chance of the prompt being honored.
 */
export async function ensureNotificationPermission(): Promise<NotificationPermission> {
  // The native binding doesn't use web permissions.
  if (nativeNotify()) return "granted";
  if (!hasWebNotifications()) return "denied";
  if (Notification.permission !== "default") return Notification.permission;
  try {
    return await Notification.requestPermission();
  } catch {
    return Notification.permission;
  }
}

/**
 * Fire a desktop notification. Resolves to `true` if something was shown.
 * Honors the `silent` setting by short-circuiting before any notification.
 * Resilient: permission denied / binding missing / constructor throwing all
 * resolve to `false` without surfacing an error.
 */
export async function notify(opts: {
  title: string;
  body: string;
  silent: boolean;
  tag?: string;
}): Promise<boolean> {
  if (opts.silent) return false;

  const native = nativeNotify();
  if (native) {
    try {
      await native(opts.title, opts.body);
      return true;
    } catch {
      // Fall through to the web API.
    }
  }

  if (!hasWebNotifications()) return false;

  try {
    let permission = Notification.permission;
    if (permission === "default") {
      permission = await ensureNotificationPermission();
    }
    if (permission !== "granted") return false;
    // `tag` de-dupes at the OS level (a repeat replaces rather than stacks).
    new Notification(opts.title, { body: opts.body, tag: opts.tag });
    return true;
  } catch {
    return false;
  }
}

// Lazily-created shared audio element so we don't allocate one per completion.
let audioEl: HTMLAudioElement | null = null;

/**
 * Play the bundled completion chime, gated behind the local "completion sound"
 * preference. No-op when disabled, when audio is unavailable, or when the
 * browser blocks autoplay (the rejected promise is swallowed).
 */
export function playCompletionSound(): void {
  if (!getCompletionSoundEnabled()) return;
  if (typeof Audio === "undefined") return;
  try {
    if (!audioEl) {
      audioEl = new Audio(completeSoundUrl);
      audioEl.volume = 0.6;
    }
    audioEl.currentTime = 0;
    void audioEl.play().catch(() => {
      // Autoplay blocked (no prior user gesture) — ignore silently.
    });
  } catch {
    // Ignore any construction/playback error.
  }
}
