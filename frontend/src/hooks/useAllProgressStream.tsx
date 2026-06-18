import { useEffect, useRef } from "react";
import { useQueryClient, type QueryClient } from "@tanstack/react-query";
import { apiBaseSync } from "@/_lib/wails";
import { downloadKeys } from "@/_lib/services/queries/download.queries";
import { useProgressStore } from "@/stores/download-progress-store";
import type { Download } from "@/_lib/types/download-types";

/**
 * Normalize a raw SSE progress payload (snake_case, possibly bogus values)
 * into a partial Download patch. Progress is clamped to [0, 100] so the UI is
 * robust regardless of what the backend sends (it can emit negative or >100
 * values when the total size is unknown / -1).
 */
function patchFromRaw(raw: any): (Partial<Download> & { id: string }) | null {
  const id: string | undefined = raw?.job_id ?? raw?.id;
  if (!id) return null;

  const patch: Partial<Download> & { id: string } = { id };

  if (raw.downloaded != null) patch.downloaded = raw.downloaded;
  if (raw.total_size != null) patch.total_size = raw.total_size;
  if (raw.speed != null) patch.speed = raw.speed;
  if (raw.status != null) patch.status = raw.status;
  if (raw.eta != null) patch.eta = raw.eta;
  if (raw.progress != null) {
    patch.progress = Math.min(100, Math.max(0, raw.progress));
  }

  return patch;
}

function applyPatch(queryClient: QueryClient, patch: Partial<Download> & { id: string }) {
  // Update the per-job detail cache (read by useJobProgress). Seed it if absent
  // so live updates are not dropped before the first list fetch resolves.
  queryClient.setQueryData<Download>(
    downloadKeys.detail(patch.id),
    (old: Download | undefined) => ({
      ...(old ?? ({ id: patch.id } as Download)),
      ...patch,
    }),
  );

  // Also patch any cached list query so list views update live without a refetch.
  queryClient
    .getQueriesData<Download[]>({ queryKey: downloadKeys.all })
    .forEach(([key, list]) => {
      if (!Array.isArray(list)) return;
      let changed = false;
      const next = list.map((dl) => {
        if (dl.id !== patch.id) return dl;
        changed = true;
        return { ...dl, ...patch };
      });
      if (changed) queryClient.setQueryData(key, next);
    });
}

/**
 * Reports whether any download tracked in the React Query cache is currently
 * running or pending. Used to decide whether the SSE stream needs to be open:
 * idle (everything completed/paused/error) means we can close it to save RAM/CPU.
 */
function hasActiveDownloads(queryClient: QueryClient): boolean {
  return queryClient
    .getQueriesData<Download[]>({ queryKey: downloadKeys.all })
    .some(([, list]) =>
      Array.isArray(list)
        ? list.some((dl) => dl?.status === "running" || dl?.status === "pending")
        : false,
    );
}

export function AllProgressStream() {
  const queryClient = useQueryClient();
  const esRef = useRef<EventSource | null>(null);
  const reconnectRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const attemptsRef = useRef(0);

  const setOnline = useProgressStore((s) => s.setOnline);
  const setHasActivity = useProgressStore((s) => s.setHasActivity);
  const hasActivity = useProgressStore((s) => s.hasActivity);

  // Keep the activity flag in sync with the download cache. Subscribing to the
  // query cache means adding/completing a download flips the flag, which in turn
  // (via the connect effect below) opens or pauses the SSE stream.
  useEffect(() => {
    const recompute = () => setHasActivity(hasActiveDownloads(queryClient));
    recompute();
    const cache = queryClient.getQueryCache();
    const unsubscribe = cache.subscribe(recompute);
    return unsubscribe;
  }, [queryClient, setHasActivity]);

  useEffect(() => {
    // No work to watch: keep the stream closed but report "online" so the
    // connection banner stays hidden — this is an intentional idle pause, not
    // a dropped connection.
    if (!hasActivity) {
      if (reconnectRef.current) {
        clearTimeout(reconnectRef.current);
        reconnectRef.current = null;
      }
      esRef.current?.close();
      esRef.current = null;
      attemptsRef.current = 0;
      setOnline(true);
      return;
    }

    let cancelled = false;
    const url = `${apiBaseSync()}/api/v1/downloads/stream`;

    const connect = () => {
      if (cancelled) return;

      const es = new EventSource(url);
      esRef.current = es;

      es.onopen = () => {
        attemptsRef.current = 0;
        setOnline(true);
      };

      es.onmessage = (event) => {
        try {
          const patch = patchFromRaw(JSON.parse(event.data));
          if (patch) applyPatch(queryClient, patch);
        } catch (err) {
          console.error("Progress event parse error", err);
        }
      };

      es.onerror = () => {
        // EventSource auto-retries, but when the server closes the stream
        // (SubscribeAll channel closed) it can get stuck. Force a clean
        // reconnect with capped exponential backoff so the UI never freezes.
        es.close();
        esRef.current = null;
        setOnline(false);
        if (cancelled) return;

        const attempt = attemptsRef.current++;
        const delay = Math.min(1000 * 2 ** attempt, 15000);
        reconnectRef.current = setTimeout(connect, delay);
      };
    };

    connect();

    return () => {
      cancelled = true;
      if (reconnectRef.current) clearTimeout(reconnectRef.current);
      esRef.current?.close();
      esRef.current = null;
    };
  }, [queryClient, setOnline, hasActivity]);

  return null;
}
