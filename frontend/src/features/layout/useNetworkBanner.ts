import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { getConnectivityStatus } from "@/_lib/services/api/status-api";
import { expireBack, nextBannerKind, type BannerKind } from "@/_lib/network-banner";
import { useProgressStore } from "@/stores/download-progress-store";

const STATUS_POLL_MS = 10_000;
const BACK_ONLINE_MS = 6_000;

/** Connectivity notice for the app shell, driven by live reachability. */
export function useNetworkBanner(): { kind: BannerKind; failed: number } {
  const engineUp = useProgressStore((s) => s.online);
  const failed = useProgressStore((s) => s.networkBlocked);
  const { data } = useQuery({
    queryKey: ["connectivity-status"],
    queryFn: getConnectivityStatus,
    refetchInterval: STATUS_POLL_MS,
    retry: false,
  });
  // Unknown (engine down / not fetched yet) reads as online: no evidence, no banner.
  const internetOnline = data?.online ?? true;

  const [kind, setKind] = useState<BannerKind>("none");
  useEffect(() => {
    setKind((prev) => nextBannerKind(prev, { engineUp, internetOnline }));
  }, [engineUp, internetOnline]);

  useEffect(() => {
    if (kind !== "back") return;
    const t = setTimeout(() => setKind(expireBack), BACK_ONLINE_MS);
    return () => clearTimeout(t);
  }, [kind]);

  return { kind, failed };
}
