import { useEffect, useRef } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { API_URL } from "@/_lib/services/api/api";
import { downloadKeys } from "@/_lib/services/queries/download.queries";
import type { Download } from "@/_lib/types/download-types";

export function AllProgressStream() {
  const queryClient = useQueryClient();
  const eventSourceRef = useRef<EventSource | null>(null);

  useEffect(() => {
    const url = `${API_URL}/api/v1/downloads/stream`;
    const es = new EventSource(url);
    eventSourceRef.current = es;

    es.onmessage = (event) => {
      try {
        const raw = JSON.parse(event.data);
        const update: Partial<Download> & { id: string } = {
          ...raw,
          id: raw.job_id,
        };

        if (!update.id) return;

        queryClient.setQueryData<Download>(
          downloadKeys.detail(update.id),
          (old) => {
            if (!old) return old;
            return {
              ...old,
              downloaded: update.downloaded ?? old.downloaded,
              totalSize: update.total_size ?? old.total_size,
              speed: update.speed ?? old.speed,
              progress: update.progress ?? old.progress,
              status: update.status ?? old.status,
            };
          },
        );
      } catch (err) {
        console.error("Progress event parse error", err);
      }
    };

    es.onerror = () => console.warn("Global progress stream error");

    return () => {
      es.close();
      eventSourceRef.current = null;
    };
  }, [queryClient]);

  return null;
}
