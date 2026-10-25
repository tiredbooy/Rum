import { describe, expect, it } from "vitest";
import { syncDrafts } from "./draft-sync";

describe("syncDrafts", () => {
  it("keeps an in-progress edit when an unrelated field is saved", () => {
    const prev = { speed: "0", parallel: "3" };
    const next = { speed: "0", parallel: "5" }; // parallel was just saved
    const drafts = { speed: "250", parallel: "3" }; // user is typing in speed
    expect(syncDrafts(prev, next, drafts)).toEqual({
      speed: "250",
      parallel: "5",
    });
  });

  it("returns the same object when nothing on the server changed", () => {
    const server = { a: "1" };
    const drafts = { a: "typing" };
    expect(syncDrafts(server, { a: "1" }, drafts)).toBe(drafts);
  });

  it("adopts every server value on first load", () => {
    expect(syncDrafts(null, { a: "1", b: "2" }, { a: "", b: "" })).toEqual({
      a: "1",
      b: "2",
    });
  });
});
