import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError, request } from "./api";

/**
 * The settings form puts a save failure next to the control that caused it.
 * That only works if `request` carries the API's per-field envelope through
 * instead of flattening it into a bare Error message — which is what it used to
 * do, discarding every field message the backend sent.
 */

const originalFetch = globalThis.fetch;

/** Await a request expected to reject, returning the typed ApiError. */
async function catchApiError(p: Promise<unknown>): Promise<ApiError> {
  try {
    await p;
  } catch (err) {
    expect(err).toBeInstanceOf(ApiError);
    return err as ApiError;
  }
  throw new Error("expected the request to reject");
}

function mockFetch(status: number, body: unknown) {
  globalThis.fetch = vi.fn(async () => ({
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  })) as unknown as typeof fetch;
}

beforeEach(() => {
  (globalThis as { window?: unknown }).window = {};
});

afterEach(() => {
  globalThis.fetch = originalFetch;
  delete (globalThis as { window?: unknown }).window;
});

describe("request", () => {
  it("returns the parsed body on success", async () => {
    mockFetch(200, { preferred_theme: "dark" });
    await expect(request("/api/v1/settings")).resolves.toEqual({
      preferred_theme: "dark",
    });
  });

  it("throws an ApiError carrying the per-field messages", async () => {
    mockFetch(400, {
      error: "validation failed",
      code: "validation_error",
      fields: { connections: "Must be 1 to 16." },
    });

    const err = await catchApiError(request("/api/v1/settings"));
    expect(err.status).toBe(400);
    expect(err.code).toBe("validation_error");
    expect(err.message).toBe("validation failed");
    expect(err.fieldError("connections")).toBe("Must be 1 to 16.");
    expect(err.fieldError("proxy")).toBeUndefined();
  });

  it("falls back to the status when the body is not JSON", async () => {
    globalThis.fetch = vi.fn(async () => ({
      ok: false,
      status: 500,
      json: async () => {
        throw new Error("not json");
      },
    })) as unknown as typeof fetch;

    const err = await catchApiError(request("/api/v1/settings"));
    expect(err.message).toBe("HTTP 500");
    expect(err.fieldError("anything")).toBeUndefined();
  });
});
