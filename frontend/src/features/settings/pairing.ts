/** Pure helpers for the browser-extension pairing card. */

/** "rum_1a2b…9f0e": enough to recognise a token without shoulder-surfing it. */
export function maskToken(token: string): string {
  if (token.length <= 12) return "•".repeat(token.length);
  return `${token.slice(0, 8)}…${token.slice(-4)}`;
}

/** Minimal slice of the browser clipboard API we rely on. */
export interface ClipboardLike {
  writeText?: (text: string) => Promise<void>;
}

/**
 * Copy text, reporting whether it worked. The Wails webview does not always
 * expose navigator.clipboard, so the caller may pass a legacy fallback (select
 * the field + execCommand("copy")).
 */
export async function copyText(
  text: string,
  clipboard: ClipboardLike | undefined,
  legacyCopy?: () => boolean,
): Promise<boolean> {
  if (clipboard?.writeText) {
    try {
      await clipboard.writeText(text);
      return true;
    } catch {
      // fall through to the legacy path
    }
  }
  try {
    return legacyCopy ? legacyCopy() : false;
  } catch {
    return false;
  }
}
