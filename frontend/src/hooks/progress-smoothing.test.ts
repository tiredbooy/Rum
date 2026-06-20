import { describe, expect, it } from "vitest";
import { stepProgress } from "./useSmoothProgress";
import { pushSample } from "./useSpeedHistory";

const cfg = { lerpFactor: 0.12, minStepPerFrame: 0.35, snapThreshold: 0.15 };

describe("stepProgress", () => {
  it("eases toward the target without overshooting", () => {
    const next = stepProgress(0, 50, cfg);
    expect(next).toBeGreaterThan(0);
    expect(next).toBeLessThanOrEqual(50);
  });

  it("never moves backward for a target it has already passed", () => {
    expect(stepProgress(60, 55, cfg)).toBe(60);
  });

  it("advances at least minStepPerFrame while trailing", () => {
    // delta * lerp = 1 * 0.12 = 0.12 < minStep(0.35) -> uses minStep
    expect(stepProgress(40, 41, cfg)).toBeCloseTo(40.35, 5);
  });

  it("snaps to target inside the snap threshold", () => {
    expect(stepProgress(99.9, 99.95, cfg)).toBe(99.95);
  });

  it("snaps to 100 on completion", () => {
    expect(stepProgress(73, 100, cfg)).toBe(100);
    expect(stepProgress(73, 150, cfg)).toBe(100);
  });

  it("snaps down to ~0 on restart (target dropped below displayed)", () => {
    expect(stepProgress(80, 0, cfg)).toBe(0);
    expect(stepProgress(80, 1, cfg)).toBe(1);
  });

  it("is monotonic non-decreasing across a rising target sequence", () => {
    let displayed = 0;
    const targets = [5, 5, 20, 20, 20, 60, 99, 99];
    for (const t of targets) {
      // Run several frames per tick like the rAF loop would.
      for (let f = 0; f < 30; f++) {
        const next = stepProgress(displayed, t, cfg);
        expect(next).toBeGreaterThanOrEqual(displayed);
        displayed = next;
      }
    }
    expect(displayed).toBeCloseTo(99, 1);
  });
});

describe("pushSample", () => {
  it("appends to the buffer", () => {
    expect(pushSample([1, 2], 3, 5)).toEqual([1, 2, 3]);
  });

  it("caps to max, dropping the oldest", () => {
    expect(pushSample([1, 2, 3], 4, 3)).toEqual([2, 3, 4]);
  });

  it("does not mutate the input array", () => {
    const input = [1, 2, 3];
    pushSample(input, 4, 3);
    expect(input).toEqual([1, 2, 3]);
  });

  it("handles an empty buffer", () => {
    expect(pushSample([], 7, 3)).toEqual([7]);
  });
});
