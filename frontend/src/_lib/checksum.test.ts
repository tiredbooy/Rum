import { describe, expect, it } from "vitest";
import { algoForChecksum } from "./checksum";

describe("algoForChecksum", () => {
  it("detects md5 and sha256 by length", () => {
    expect(algoForChecksum("a".repeat(32))).toBe("md5");
    expect(algoForChecksum(` ${"B".repeat(64)} `)).toBe("sha256");
  });
  it("returns undefined otherwise", () => {
    expect(algoForChecksum("abc")).toBeUndefined();
    expect(algoForChecksum("z".repeat(32))).toBeUndefined();
  });
});
