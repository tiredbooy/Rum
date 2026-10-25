import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/_lib/services/api/api";
import { refreshDownloadUrl } from "@/_lib/services/api/download-api";
import type { Download } from "@/_lib/types/download-types";
import {
  isLinkExpired,
  linkProblem,
  refreshFailure,
} from "./link-refresh";

const base: Download = {
  id: "1",
  url: "https://old.example/f.bin",
  status: "error",
  downloaded: 0,
  remaining: 0,
};

describe("isLinkExpired", () => {
  it("is true only for a failed download coded link_expired", () => {
    expect(isLinkExpired({ ...base, error_code: "link_expired" })).toBe(true);
    expect(isLinkExpired({ ...base, error_code: "disk_full" })).toBe(false);
    expect(isLinkExpired({ ...base })).toBe(false);
    expect(
      isLinkExpired({ ...base, status: "running", error_code: "link_expired" }),
    ).toBe(false);
  });
});

describe("linkProblem", () => {
  it("accepts absolute http(s) links, ignoring surrounding spaces", () => {
    expect(linkProblem("  https://a.example/x?t=1 ")).toBeNull();
    expect(linkProblem("http://a.example/x")).toBeNull();
  });

  it("rejects empty, relative and non-http links with a short message", () => {
    for (const bad of ["", "   ", "/x", "ftp://a.example/x", "not a link"]) {
      const msg = linkProblem(bad);
      expect(msg).toBeTruthy();
      expect(msg!.split(/\s+/).length).toBeLessThanOrEqual(12);
    }
  });
});

describe("refreshFailure", () => {
  it("turns a remote_changed envelope into a restart question", () => {
    const err = new ApiError("new link is a different file", 409, "remote_changed");
    expect(refreshFailure(err)).toEqual({ kind: "confirm-restart" });
  });

  it("maps other failures to a short actionable message", () => {
    const unreachable = new ApiError("x", 400, "validation_error");
    const f = refreshFailure(unreachable);
    expect(f.kind).toBe("error");
    if (f.kind === "error") expect(f.message.split(/\s+/).length).toBeLessThanOrEqual(12);
    expect(refreshFailure(new Error("boom")).kind).toBe("error");
  });
});

describe("refreshDownloadUrl", () => {
  const originalFetch = globalThis.fetch;
  beforeEach(() => {
    (globalThis as { window?: unknown }).window = {};
  });
  afterEach(() => {
    globalThis.fetch = originalFetch;
    delete (globalThis as { window?: unknown }).window;
  });

  it("PATCHes /downloads/:id/url with the link and restart flag", async () => {
    const fetchMock = vi.fn(async () => ({
      ok: true,
      status: 200,
      json: async () => ({ id: "1", resumed: true, restarted: false }),
    }));
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    const res = await refreshDownloadUrl("1", "https://new.example/f.bin", true);

    expect(res.resumed).toBe(true);
    const [path, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
    expect(path).toContain("/api/v1/downloads/1/url");
    expect(init.method).toBe("PATCH");
    expect(JSON.parse(init.body as string)).toEqual({
      url: "https://new.example/f.bin",
      restart: true,
    });
  });

  it("surfaces remote_changed as an ApiError the dialog can branch on", async () => {
    globalThis.fetch = vi.fn(async () => ({
      ok: false,
      status: 409,
      json: async () => ({ error: "new link is a different file", code: "remote_changed" }),
    })) as unknown as typeof fetch;

    await expect(refreshDownloadUrl("1", "https://new.example/f.bin")).rejects.toMatchObject({
      code: "remote_changed",
      status: 409,
    });
  });
});
