import { describe, expect, it } from "vitest";
import { countNetworkBlocked, probeBackend } from "./backend-probe";

const fakeFetch = (impl: () => Promise<Partial<Response>>) =>
  (() => impl()) as unknown as typeof fetch;

describe("probeBackend", () => {
  it("is true on an ok response", async () => {
    expect(await probeBackend("http://x", fakeFetch(async () => ({ ok: true })))).toBe(true);
  });
  it("is false on a non-ok response", async () => {
    expect(await probeBackend("http://x", fakeFetch(async () => ({ ok: false })))).toBe(false);
  });
  it("is false when the request throws", async () => {
    const f = fakeFetch(async () => {
      throw new TypeError("Failed to fetch");
    });
    expect(await probeBackend("http://x", f)).toBe(false);
  });
});

describe("countNetworkBlocked", () => {
  it("counts a download once when several cached lists contain it", () => {
    const d = { id: "a", status: "error", error_code: "dns" };
    expect(countNetworkBlocked([d, { ...d }, { id: "b", status: "error", error_code: "dns" }])).toBe(2);
  });

  it("counts only errored downloads with a connectivity code", () => {
    expect(
      countNetworkBlocked([
        { status: "error", error_code: "network_down" },
        { status: "error", error_code: "dns" },
        { status: "error", error_code: "disk_full" },
        { status: "running", error_code: "network_down" },
        { status: "error" },
        null,
      ]),
    ).toBe(2);
  });
});
