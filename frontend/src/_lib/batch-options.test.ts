import { describe, expect, it } from "vitest";
import { buildBatchOptions } from "./batch-options";

describe("buildBatchOptions", () => {
  it("is undefined when nothing is set", () => {
    expect(buildBatchOptions({ destPath: "  " })).toBeUndefined();
  });
  it("carries the night toggle", () => {
    expect(buildBatchOptions({ nightQueue: true })).toEqual({ night_queue: true });
  });
  it("combines folder and night", () => {
    expect(buildBatchOptions({ destPath: " a ", nightQueue: true })).toEqual({
      dest_path: "a",
      night_queue: true,
    });
  });
  it("omits night when off", () => {
    expect(buildBatchOptions({ destPath: "a", nightQueue: false })).toEqual({ dest_path: "a" });
  });
});
