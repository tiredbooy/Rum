import { describe, expect, it } from "vitest";
import { countdownSeconds, finishTitle } from "./finish-countdown";

describe("countdownSeconds", () => {
  it("rounds up so 0 only shows at the deadline", () => {
    expect(countdownSeconds(10_000, 8_500)).toBe(2);
    expect(countdownSeconds(10_000, 9_999)).toBe(1);
    expect(countdownSeconds(10_000, 10_000)).toBe(0);
  });
  it("never goes negative", () => {
    expect(countdownSeconds(10_000, 20_000)).toBe(0);
  });
});

describe("finishTitle", () => {
  it("names the action and the seconds left", () => {
    expect(finishTitle("sleep", 42)).toBe("Sleeping in 42s");
    expect(finishTitle("shutdown", 1)).toBe("Shutting down in 1s");
  });
});
