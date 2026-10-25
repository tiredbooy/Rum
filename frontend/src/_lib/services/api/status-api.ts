import { request } from "./api";

/** GET /status: live internet reachability measured by the engine. */
export interface ConnectivityStatus {
  online: boolean;
  /** "site_blocked": internet works but a download host does not. */
  state?: "online" | "site_blocked" | "offline";
  last_probe_at?: string;
}

export function getConnectivityStatus(): Promise<ConnectivityStatus> {
  return request<ConnectivityStatus>("/api/v1/status");
}
