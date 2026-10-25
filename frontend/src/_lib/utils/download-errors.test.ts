import { describe, expect, it } from "vitest";
import {
  DOWNLOAD_ERROR_MESSAGES,
  NETWORK_ERROR_CODES,
  describeDownloadError,
} from "./download-errors";

const CODES = [
  "network_down", "dns", "tls", "timeout", "proxy", "server_error",
  "permission", "disk_full", "blocked_page", "stalled", "link_expired", "site_unreachable", "night_limit", "unknown",
];

describe("download error map", () => {
  it("has an entry for every backend code", () => {
    for (const c of CODES) expect(DOWNLOAD_ERROR_MESSAGES[c], c).toBeDefined();
  });

  it("keeps each message plus action within 12 words", () => {
    for (const [code, { message, action }] of Object.entries(DOWNLOAD_ERROR_MESSAGES)) {
      const words = `${message} ${action}`.split(/\s+/).length;
      expect(words, code).toBeLessThanOrEqual(12);
    }
  });

  it("describes network_down as in the spec", () => {
    const i = describeDownloadError("network_down");
    expect(`${i.message} ${i.action}`).toBe("Connection lost. Check internet or VPN.");
  });

  it("describes stalled with a clear no-data message and a retry action", () => {
    const i = describeDownloadError("stalled");
    expect(i.message).toMatch(/stalled.*no data/i);
    expect(i.action).toMatch(/^Retry/);
  });

  it("blames the site, not the internet, for site_unreachable", () => {
    const i = describeDownloadError("site_unreachable");
    expect(`${i.message} ${i.action}`).toBe("This site is unreachable. Try a VPN or proxy.");
    expect(NETWORK_ERROR_CODES.has("site_unreachable")).toBe(false);
  });

  it("falls back to unknown for missing or new codes", () => {
    expect(describeDownloadError(undefined)).toBe(DOWNLOAD_ERROR_MESSAGES.unknown);
    expect(describeDownloadError("from_the_future")).toBe(DOWNLOAD_ERROR_MESSAGES.unknown);
  });

  it("flags only connectivity codes as network errors", () => {
    expect(NETWORK_ERROR_CODES.has("dns")).toBe(true);
    expect(NETWORK_ERROR_CODES.has("disk_full")).toBe(false);
  });
});
