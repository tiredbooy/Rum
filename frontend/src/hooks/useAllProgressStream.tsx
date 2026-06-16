import { useEffect, useRef } from "react";
import { useQueryClient, type QueryClient } from "@tanstack/react-query";
import { API_URL } from "@/_lib/services/api/api";
import { downloadKeys } from "@/_lib/services/queries/download.queries";
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

export function AllProgressStream() {
  const queryClient = useQueryClient();
  const esRef = useRef<EventSource | null>(null);
  const reconnectRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const attemptsRef = useRef(0);

  useEffect(() => {
    let cancelled = false;
    const url = `${API_URL}/api/v1/downloads/stream`;

    const connect = () => {
      if (cancelled) return;

      const es = new EventSource(url);
      esRef.current = es;

      es.onopen = () => {
        attemptsRef.current = 0;
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
  }, [queryClient]);

  return null;
}
