import { describe, expect, it } from "vitest";
import { expandWildcard, hasWildcard, validateRange, WILDCARD_MAX } from "./wildcard";

describe("hasWildcard", () => {
  it("detects a star", () => {
    expect(hasWildcard("https://x.com/file*.rar")).toBe(true);
    expect(hasWildcard("https://x.com/file1.rar")).toBe(false);
  });
});

describe("validateRange", () => {
  it("rejects to < from", () => {
    expect(validateRange({ from: 5, to: 2, pad: 0 })).toMatch(/from/i);
  });
  it("rejects counts over the cap", () => {
    expect(validateRange({ from: 1, to: WILDCARD_MAX + 1, pad: 0 })).toMatch(/1000/);
  });
  it("accepts a valid range", () => {
    expect(validateRange({ from: 1, to: 12, pad: 2 })).toBeNull();
  });
});

describe("expandWildcard", () => {
  it("expands without padding", () => {
    expect(expandWildcard("https://x.com/f*.zip", { from: 1, to: 3, pad: 0 })).toEqual([
      "https://x.com/f1.zip",
      "https://x.com/f2.zip",
      "https://x.com/f3.zip",
    ]);
  });
  it("expands with zero-padding", () => {
    expect(expandWildcard("https://x.com/game.part*.rar", { from: 1, to: 2, pad: 2 })).toEqual([
      "https://x.com/game.part01.rar",
      "https://x.com/game.part02.rar",
    ]);
  });
  it("only replaces the first star", () => {
    expect(expandWildcard("https://x.com/*/f*.zip", { from: 1, to: 1, pad: 0 })).toEqual([
      "https://x.com/1/f*.zip",
    ]);
  });
});
