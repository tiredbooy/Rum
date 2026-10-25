import { describe, expect, it } from "vitest";
import {
  DEFAULT_NIGHT_PLAN,
  clockOptions,
  crossesMidnight,
  nightChip,
  validateNightPlan,
} from "./night-plan";

const plan = DEFAULT_NIGHT_PLAN;

describe("validateNightPlan", () => {
  it("accepts the default and a midnight-crossing window", () => {
    expect(validateNightPlan(plan)).toEqual({});
    expect(validateNightPlan({ ...plan, start: "23:00", end: "06:00" })).toEqual(
      {},
    );
  });

  it("rejects bad times, an empty window and a bad cap like the backend", () => {
    expect(validateNightPlan({ ...plan, start: "25:00" }).start).toBeDefined();
    expect(validateNightPlan({ ...plan, end: "7:00" }).end).toBeDefined();
    expect(
      validateNightPlan({ ...plan, start: "03:00", end: "03:00" }).window,
    ).toBeDefined();
    expect(validateNightPlan({ ...plan, cap_gb: -1 }).cap_gb).toBeDefined();
    expect(validateNightPlan({ ...plan, cap_gb: NaN }).cap_gb).toBeDefined();
    expect(validateNightPlan({ ...plan, cap_gb: 0 }).cap_gb).toBeUndefined();
  });
});

describe("crossesMidnight", () => {
  it("is true only when end is earlier than start", () => {
    expect(crossesMidnight({ start: "23:00", end: "06:00" })).toBe(true);
    expect(crossesMidnight({ start: "02:00", end: "07:00" })).toBe(false);
  });
});

describe("clockOptions", () => {
  it("has 96 steps and keeps an off-grid saved value selectable", () => {
    expect(clockOptions()).toHaveLength(96);
    const withOdd = clockOptions("02:10");
    expect(withOdd).toContain("02:10");
    expect(withOdd).toEqual([...withOdd].sort());
  });
});

describe("nightChip", () => {
  const state = { enabled: true, start: "02:00", active: false, cap_reached: false };
  it("shows the start time while the window is closed", () => {
    expect(nightChip({ night_queue: true, status: "paused" }, state)).toEqual({
      label: "Starts 02:00",
      tone: "waiting",
    });
  });
  it("shows Night while open and Cap reached when spent", () => {
    expect(
      nightChip({ night_queue: true, status: "running" }, { ...state, active: true })
        ?.label,
    ).toBe("Night");
    expect(
      nightChip(
        { night_queue: true, status: "paused" },
        { ...state, active: true, cap_reached: true },
      )?.label,
    ).toBe("Cap reached");
  });
  it("hides for non-night, finished jobs and a disabled plan", () => {
    expect(nightChip({ night_queue: false, status: "paused" }, state)).toBeNull();
    expect(nightChip({ night_queue: true, status: "completed" }, state)).toBeNull();
    expect(
      nightChip({ night_queue: true, status: "paused" }, { ...state, enabled: false }),
    ).toBeNull();
    expect(nightChip({ night_queue: true, status: "paused" }, undefined)).toBeNull();
  });
});
