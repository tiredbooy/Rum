import { request } from "./api";

/** GET /status: live internet reachability measured by the engine. */
export interface ConnectivityStatus {
  online: boolean;
  last_probe_at?: string;
}

export function getConnectivityStatus(): Promise<ConnectivityStatus> {
  return request<ConnectivityStatus>("/api/v1/status");
}
