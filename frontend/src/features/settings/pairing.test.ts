import { afterEach, describe, expect, it, vi } from "vitest";
import { copyText, maskToken } from "./pairing";
import {
  getExtensionToken,
  regenerateExtensionToken,
} from "@/_lib/services/api/extension-api";

describe("maskToken", () => {
  it("keeps the prefix and tail only", () => {
    const t = "rum_" + "a".repeat(60) + "9f0e";
    expect(maskToken(t)).toBe("rum_aaaa…9f0e");
  });
  it("hides short values entirely", () => {
    expect(maskToken("abc")).toBe("•••");
  });
});

describe("copyText", () => {
  it("uses the clipboard API when present", async () => {
    const writeText = vi.fn(async () => {});
    await expect(copyText("tok", { writeText })).resolves.toBe(true);
    expect(writeText).toHaveBeenCalledWith("tok");
  });
  it("falls back when the clipboard API rejects", async () => {
    const writeText = vi.fn(async () => {
      throw new Error("denied");
    });
    const legacy = vi.fn(() => true);
    await expect(copyText("tok", { writeText }, legacy)).resolves.toBe(true);
    expect(legacy).toHaveBeenCalled();
  });
  it("falls back when there is no clipboard API", async () => {
    await expect(copyText("tok", undefined, () => true)).resolves.toBe(true);
  });
  it("reports failure when nothing works", async () => {
    await expect(copyText("tok", undefined)).resolves.toBe(false);
    await expect(
      copyText("tok", undefined, () => {
        throw new Error("no");
      }),
    ).resolves.toBe(false);
  });
});

describe("extension token api", () => {
  const originalFetch = globalThis.fetch;
  afterEach(() => {
    globalThis.fetch = originalFetch;
    delete (globalThis as { window?: unknown }).window;
  });

  function mock(body: unknown, status = 200) {
    (globalThis as { window?: unknown }).window = {};
    const fn = vi.fn(async (..._args: unknown[]) => ({
      ok: status < 300,
      status,
      json: async () => body,
    }));
    globalThis.fetch = fn as unknown as typeof fetch;
    return fn;
  }

  it("GETs the token", async () => {
    const fn = mock({ token: "rum_x" });
    await expect(getExtensionToken()).resolves.toEqual({ token: "rum_x" });
    expect(String(fn.mock.calls[0][0])).toContain("/api/v1/extension/token");
  });

  it("POSTs JSON to regenerate", async () => {
    const fn = mock({ token: "rum_new" });
    await expect(regenerateExtensionToken()).resolves.toEqual({ token: "rum_new" });
    const [url, init] = fn.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/v1/extension/token/regenerate");
    expect(init.method).toBe("POST");
    expect((init.headers as Record<string, string>)["Content-Type"]).toBe("application/json");
  });

  it("surfaces a failure", async () => {
    mock({ error: "boom" }, 500);
    await expect(getExtensionToken()).rejects.toThrow("boom");
  });
});
