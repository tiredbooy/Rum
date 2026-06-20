import { describe, expect, it } from "vitest";
import { computeAggregateEta } from "./useAggregateEta";

describe("computeAggregateEta", () => {
  it("sums remaining bytes and speed over running downloads", () => {
    const r = computeAggregateEta([
      { status: "running", speed: 8, total_size: 100, downloaded: 20 }, // 80 left
      { status: "running", speed: 2, total_size: 200, downloaded: 100 }, // 100 left
      { status: "completed", speed: 0, total_size: 50, downloaded: 50 }, // ignored
      { status: "paused", speed: 0, total_size: 50, downloaded: 10 }, // ignored
    ] as never);
    expect(r.activeCount).toBe(2);
    expect(r.totalSpeed).toBe(10);
    expect(r.remainingBytes).toBe(180);
    expect(r.etaSeconds).toBe(18); // 180 / 10
    expect(r.hasUnknownSize).toBe(false);
  });

  it("returns null ETA when combined speed is zero (stalled)", () => {
    const r = computeAggregateEta([
      { status: "running", speed: 0, total_size: 100, downloaded: 20 },
    ] as never);
    expect(r.etaSeconds).toBeNull();
    expect(r.activeCount).toBe(1);
  });

  it("flags unknown size and excludes it from remaining", () => {
    const r = computeAggregateEta([
      { status: "running", speed: 5, total_size: 0, downloaded: 0 },
      { status: "running", speed: 5, total_size: 100, downloaded: 0 },
    ] as never);
    expect(r.hasUnknownSize).toBe(true);
    expect(r.remainingBytes).toBe(100);
    expect(r.etaSeconds).toBe(10); // 100 remaining / (5+5) combined speed
  });

  it("returns null ETA and zero counts when nothing is running", () => {
    const r = computeAggregateEta([
      { status: "completed", speed: 0, total_size: 100, downloaded: 100 },
    ] as never);
    expect(r.activeCount).toBe(0);
    expect(r.etaSeconds).toBeNull();
  });
});
