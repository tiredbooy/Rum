/** Which connectivity notice is showing. */
export type BannerKind = "none" | "engine" | "offline" | "back";

interface BannerInput {
  /** Rum's own backend answers. */
  engineUp: boolean;
  /** The internet is reachable right now (GET /status). */
  internetOnline: boolean;
}

/**
 * Next banner state from current connectivity, not from error history: the
 * network notice shows only while offline, then "back" once it returns.
 */
export function nextBannerKind(prev: BannerKind, { engineUp, internetOnline }: BannerInput): BannerKind {
  if (!engineUp) return "engine";
  if (!internetOnline) return "offline";
  return prev === "offline" || prev === "back" ? "back" : "none";
}

/** The "Back online" notice is brief: clear it, leave every other state alone. */
export function expireBack(kind: BannerKind): BannerKind {
  return kind === "back" ? "none" : kind;
}

/** Banner text; `failed` is the count of downloads that failed on the network. */
export function bannerMessage(kind: BannerKind, failed: number): string {
  switch (kind) {
    case "engine":
      return "Rum's engine isn't responding. Retrying…";
    case "offline":
      return failed > 0
        ? `Network problem. ${failed} ${failed === 1 ? "download" : "downloads"} can't continue. Check internet or VPN.`
        : "No internet. Downloads resume when it returns.";
    case "back":
      return failed > 0 ? "Back online. Some downloads failed." : "Back online.";
    default:
      return "";
  }
}
