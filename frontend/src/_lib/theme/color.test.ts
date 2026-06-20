import { describe, expect, it } from "vitest";
import {
  deriveAccentVars,
  isValidAccent,
  normalizeHex,
  relativeLuminance,
} from "./color";
import { resolveColorScheme } from "./apply-theme";

describe("isValidAccent", () => {
  it("accepts #RGB and #RRGGBB", () => {
    expect(isValidAccent("#abc")).toBe(true);
    expect(isValidAccent("#A1B2C3")).toBe(true);
    expect(isValidAccent("  #fff  ")).toBe(true);
  });
  it("rejects bad input", () => {
    expect(isValidAccent("")).toBe(false);
    expect(isValidAccent("abc")).toBe(false);
    expect(isValidAccent("#12")).toBe(false);
    expect(isValidAccent("#1234")).toBe(false);
    expect(isValidAccent("rgb(0,0,0)")).toBe(false);
  });
});

describe("normalizeHex", () => {
  it("expands shorthand and lowercases", () => {
    expect(normalizeHex("#ABC")).toBe("#aabbcc");
    expect(normalizeHex("#A1B2C3")).toBe("#a1b2c3");
  });
});

describe("relativeLuminance", () => {
  it("is 0 for black and 1 for white", () => {
    expect(relativeLuminance("#000000")).toBeCloseTo(0, 5);
    expect(relativeLuminance("#ffffff")).toBeCloseTo(1, 5);
  });
});

describe("deriveAccentVars", () => {
  it("returns null for empty/invalid", () => {
    expect(deriveAccentVars("")).toBeNull();
    expect(deriveAccentVars(undefined)).toBeNull();
    expect(deriveAccentVars("not-a-color")).toBeNull();
  });
  it("picks white fg for a dark accent, dark fg for a bright accent", () => {
    expect(deriveAccentVars("#1e1b4b")?.accentForeground).toBe("#ffffff");
    expect(deriveAccentVars("#f59e0b")?.accentForeground).toBe("#0a0a0a");
  });
  it("normalizes the accent fill", () => {
    expect(deriveAccentVars("#ABC")?.accent).toBe("#aabbcc");
  });
});

describe("resolveColorScheme", () => {
  it("honors explicit choices", () => {
    expect(resolveColorScheme("light", true)).toBe("light");
    expect(resolveColorScheme("dark", false)).toBe("dark");
  });
  it("follows the OS for system", () => {
    expect(resolveColorScheme("system", true)).toBe("dark");
    expect(resolveColorScheme("system", false)).toBe("light");
  });
});
