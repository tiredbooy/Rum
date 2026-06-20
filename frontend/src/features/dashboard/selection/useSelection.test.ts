import { beforeEach, describe, expect, it } from "vitest";
import { useSelection } from "./useSelection";

const ids = ["a", "b", "c", "d", "e"];

function reset() {
  useSelection.getState().clear();
}

describe("useSelection", () => {
  beforeEach(reset);

  it("toggles an id on and off and tracks the anchor", () => {
    const { toggle } = useSelection.getState();
    toggle("b");
    expect(useSelection.getState().selectedIds.has("b")).toBe(true);
    expect(useSelection.getState().lastClickedId).toBe("b");
    toggle("b");
    expect(useSelection.getState().selectedIds.has("b")).toBe(false);
  });

  it("setSelected adds/removes explicitly", () => {
    const { setSelected } = useSelection.getState();
    setSelected("a", true);
    setSelected("c", true);
    setSelected("a", false);
    expect([...useSelection.getState().selectedIds].sort()).toEqual(["c"]);
  });

  it("selectRange selects the inclusive range from the anchor", () => {
    const { toggle, selectRange } = useSelection.getState();
    toggle("b"); // anchor = b
    selectRange("d", ids); // b..d
    expect([...useSelection.getState().selectedIds].sort()).toEqual([
      "b",
      "c",
      "d",
    ]);
  });

  it("selectRange works regardless of anchor/target order", () => {
    const { toggle, selectRange } = useSelection.getState();
    toggle("d"); // anchor = d
    selectRange("b", ids); // b..d (target above anchor)
    expect([...useSelection.getState().selectedIds].sort()).toEqual([
      "b",
      "c",
      "d",
    ]);
  });

  it("selectRange keeps the original anchor so it extends from it", () => {
    const { toggle, selectRange } = useSelection.getState();
    toggle("b");
    selectRange("c", ids); // b..c
    selectRange("e", ids); // still anchored at b → b..e
    expect([...useSelection.getState().selectedIds].sort()).toEqual([
      "b",
      "c",
      "d",
      "e",
    ]);
  });

  it("selectRange with no anchor acts as a single add", () => {
    const { selectRange } = useSelection.getState();
    selectRange("c", ids);
    expect([...useSelection.getState().selectedIds]).toEqual(["c"]);
  });

  it("selectAll replaces the selection", () => {
    const { toggle, selectAll } = useSelection.getState();
    toggle("a");
    selectAll(["x", "y"]);
    expect([...useSelection.getState().selectedIds].sort()).toEqual(["x", "y"]);
  });

  it("selectByFilter adds matching items to the existing selection", () => {
    const { toggle, selectByFilter } = useSelection.getState();
    toggle("a");
    const items = [
      { id: "a", status: "completed" },
      { id: "b", status: "error" },
      { id: "c", status: "error" },
    ];
    selectByFilter(items, (i) => i.status === "error");
    expect([...useSelection.getState().selectedIds].sort()).toEqual([
      "a",
      "b",
      "c",
    ]);
  });

  it("clear empties the selection and anchor", () => {
    const { toggle, clear } = useSelection.getState();
    toggle("a");
    clear();
    expect(useSelection.getState().selectedIds.size).toBe(0);
    expect(useSelection.getState().lastClickedId).toBeNull();
  });
});
