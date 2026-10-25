import { describe, expect, it } from "vitest";
import {
  MAX_MIRRORS,
  cleanMirrors,
  mirrorLabel,
  mirrorProblem,
  withCleanMirrors,
} from "./mirrors";

describe("mirrorProblem", () => {
  it("accepts blank and http(s) links", () => {
    expect(mirrorProblem("")).toBeNull();
    expect(mirrorProblem("  ")).toBeNull();
    expect(mirrorProblem("https://a.example/f.zip")).toBeNull();
    expect(mirrorProblem("http://a.example/f.zip")).toBeNull();
  });

  it("rejects other schemes and junk", () => {
    expect(mirrorProblem("ftp://a.example/f")).toMatch(/http/);
    expect(mirrorProblem("not a link")).toMatch(/valid/);
  });
});

describe("cleanMirrors", () => {
  it("trims, drops blanks and repeats", () => {
    expect(cleanMirrors([" https://a/f ", "", "https://a/f", "https://b/f"])).toEqual([
      "https://a/f",
      "https://b/f",
    ]);
  });

  it("caps at the backend limit", () => {
    const many = Array.from({ length: 9 }, (_, i) => `https://m${i}/f`);
    expect(cleanMirrors(many)).toHaveLength(MAX_MIRRORS);
  });
});

describe("mirrorLabel", () => {
  it("is empty on the main link", () => {
    expect(mirrorLabel({ status: "running" })).toBeNull();
  });

  it("names the active mirror host", () => {
    expect(mirrorLabel({ status: "running", active_mirror: "cdn2.example.com" })).toBe(
      "Mirror cdn2.example.com",
    );
  });
});

describe("withCleanMirrors", () => {
  it("keeps cleaned mirrors for one link", () => {
    expect(
      withCleanMirrors({ urls: ["https://a/f"], mirrors: ["", " https://b/f "] }),
    ).toEqual({ urls: ["https://a/f"], mirrors: ["https://b/f"] });
  });

  it("omits mirrors when empty or for several links", () => {
    expect(withCleanMirrors({ urls: ["https://a/f"], mirrors: [""] })).toEqual({
      urls: ["https://a/f"],
    });
    expect(
      withCleanMirrors({ urls: ["https://a/f", "https://c/f"], mirrors: ["https://b/f"] }),
    ).toEqual({ urls: ["https://a/f", "https://c/f"] });
  });
});
