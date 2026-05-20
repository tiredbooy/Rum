import { useState, useEffect, useRef } from "react";
import { API_URL } from "@/_lib/services/api/api";
import type { Download } from "@/_lib/types/download-types";

export function useJobProgress(jobId: string | undefined) {
  const [progress, setProgress] = useState<Partial<Download> | null>(null);
  const [error, setError] = useState<string | null>(null);
  const eventSourceRef = useRef<EventSource | null>(null);

  useEffect(() => {
    if (!jobId) return;

    const url = `${API_URL}/api/v1/downloads/${jobId}/stream`;
    const es = new EventSource(url);
    eventSourceRef.current = es;

    es.onmessage = (event) => {
      try {
        const update: Download = JSON.parse(event.data);
        setProgress(update);
        setError(null);
      } catch {
      }
    };

    es.onerror = () => {
      setError("Connection lost, retrying…");
    };

    return () => {
      es.close();
      eventSourceRef.current = null;
    };
  }, [jobId]);

  return { progress, error };
}
