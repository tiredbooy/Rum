import { describe, expect, it } from "vitest";
import { isValidProxy } from "./proxy";
import { clampNum, rangeError } from "./numeric-bounds";

describe("isValidProxy", () => {
  it("accepts an empty value as 'no proxy'", () => {
    expect(isValidProxy("")).toBe(true);
    expect(isValidProxy("   ")).toBe(true);
  });

  it("accepts a bare host:port as an http proxy", () => {
    expect(isValidProxy("127.0.0.1:1080")).toBe(true);
  });

  it("accepts the schemes net/http can dial", () => {
    expect(isValidProxy("http://user:pass@host:3128")).toBe(true);
    expect(isValidProxy("https://host:3128")).toBe(true);
    expect(isValidProxy("socks5://127.0.0.1:9050")).toBe(true);
    expect(isValidProxy("socks5h://127.0.0.1:9050")).toBe(true);
  });

  it("rejects a scheme the engine would silently ignore", () => {
    expect(isValidProxy("ftp://host:21")).toBe(false);
  });

  it("rejects a value with no host", () => {
    expect(isValidProxy("http://")).toBe(false);
  });
});

describe("clampNum", () => {
  it("clamps into the field range", () => {
    expect(clampNum("99", "connections")).toBe(16);
    expect(clampNum("0", "connections")).toBe(1);
    expect(clampNum("-4", "speed_limit_kb")).toBe(0);
    expect(clampNum("200", "max_parallel")).toBe(64);
  });

  it("falls back to the default for blank or unparseable input", () => {
    expect(clampNum("", "connections")).toBe(8);
    expect(clampNum("abc", "max_retries")).toBe(3);
  });

  it("rounds a fractional entry", () => {
    expect(clampNum("4.6", "connections")).toBe(5);
  });
});

describe("rangeError", () => {
  it("stays quiet for an in-range value", () => {
    expect(rangeError("8", "connections")).toBeUndefined();
    expect(rangeError("0", "speed_limit_kb")).toBeUndefined();
  });

  it("stays quiet for a blank field (it commits to the default)", () => {
    expect(rangeError("", "connections")).toBeUndefined();
  });

  it("explains the range when the typed value is out of bounds", () => {
    expect(rangeError("0", "connections")).toBe("Must be 1 to 16.");
    expect(rangeError("61", "retry_backoff_sec")).toBe("Must be 1 to 60 seconds.");
    expect(rangeError("-1", "speed_limit_kb")).toBe("Must be 0 or more.");
  });
});
