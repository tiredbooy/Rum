import { useEffect, useRef, useState } from "react";

/**
 * Merge a new server snapshot into the local text drafts, overwriting only the
 * fields whose server value changed. Returns `drafts` itself when nothing did.
 *
 * Resetting every draft whenever the settings cache changes meant saving one
 * field wiped whatever the user was still typing into another.
 */
export function syncDrafts(
  prev: Record<string, string> | null,
  next: Record<string, string>,
  drafts: Record<string, string>,
): Record<string, string> {
  let out = drafts;
  for (const key of Object.keys(next)) {
    if (prev !== null && prev[key] === next[key]) continue;
    if (out[key] === next[key]) continue;
    if (out === drafts) out = { ...drafts };
    out[key] = next[key];
  }
  return out;
}

/**
 * Text drafts for settings inputs, seeded from `server` (null while loading)
 * and re-synced per field only when that field's server value changes.
 */
export function useSyncedDrafts<K extends string>(
  server: Record<K, string> | null,
  initial: Record<K, string>,
) {
  const [drafts, setDrafts] = useState<Record<K, string>>(initial);
  const prev = useRef<Record<string, string> | null>(null);

  // Keyed on the values, not the object identity, so a fresh cache object with
  // the same contents is a no-op.
  const signature = server ? JSON.stringify(server) : "";
  useEffect(() => {
    if (!server) return;
    setDrafts((d) => syncDrafts(prev.current, server, d) as Record<K, string>);
    prev.current = server;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [signature]);

  return [drafts, setDrafts] as const;
}
