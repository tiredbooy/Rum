import { describe, expect, it } from "vitest";
import { patchFromRaw } from "./progress-patch";

describe("patchFromRaw", () => {
  it("applies error fields live", () => {
    const p = patchFromRaw({
      job_id: "a",
      status: "error",
      error: "gone",
      error_code: "link_expired",
      retryable: true,
    });
    expect(p).toMatchObject({ id: "a", error_code: "link_expired", retryable: true, error: "gone" });
  });

  it("clears stale failure when the job leaves error", () => {
    const p = patchFromRaw({ job_id: "a", status: "running", progress: 120 });
    expect(p?.error_code).toBeUndefined();
    expect(p?.retryable).toBeUndefined();
    expect("error_code" in p!).toBe(true);
    expect(p?.progress).toBe(100);
  });

  it("leaves error fields alone on progress-only frames of an errored job", () => {
    const p = patchFromRaw({ job_id: "a", status: "error", downloaded: 1 });
    expect("error_code" in p!).toBe(false);
  });
});
