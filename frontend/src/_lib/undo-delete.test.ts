import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  UNDO_TOAST_MS,
  deleteWithUndo,
  mergeResults,
  type UndoDeleteDeps,
} from "./undo-delete";
import { deleteDownload, deleteDownloads, restoreDownloads } from "./services/api/download-api";

const result = (ids: string[]) => ({ ids, pending_delete_at: "t", undo_seconds: 12 });

function makeDeps(over: Partial<UndoDeleteDeps> = {}) {
  const undoable = vi.fn();
  const deps: UndoDeleteDeps = {
    run: vi.fn(async () => result(["a"])),
    restore: vi.fn(async () => ({ restored: 1 })),
    refresh: vi.fn(),
    notify: { undoable, info: vi.fn(), error: vi.fn() },
    ...over,
  };
  return { deps, undoable };
}

describe("deleteWithUndo", () => {
  it("deletes immediately and shows a 10s Undo toast", async () => {
    const { deps, undoable } = makeDeps();
    const ids = await deleteWithUndo(false, deps);
    expect(ids).toEqual(["a"]);
    expect(deps.run).toHaveBeenCalledWith({ deleteFile: false });
    expect(undoable).toHaveBeenCalledTimes(1);
    const [message, action, ms] = undoable.mock.calls[0];
    expect(message).toBe("Removed from list");
    expect(action.label).toBe("Undo");
    expect(ms).toBe(UNDO_TOAST_MS);
    expect(UNDO_TOAST_MS).toBe(10_000);
  });

  it("words the toast differently for Delete file and for bulk", async () => {
    const one = makeDeps();
    await deleteWithUndo(true, one.deps);
    expect(one.deps.run).toHaveBeenCalledWith({ deleteFile: true });
    expect(one.undoable.mock.calls[0][0]).toBe("File deleted");

    const many = makeDeps({ run: vi.fn(async () => result(["a", "b", "c"])) });
    await deleteWithUndo(false, many.deps);
    expect(many.undoable.mock.calls[0][0]).toBe("3 removed from list");
  });

  it("Undo restores every id once and refreshes the lists", async () => {
    const { deps, undoable } = makeDeps({ run: vi.fn(async () => result(["a", "b"])) });
    await deleteWithUndo(false, deps);
    undoable.mock.calls[0][1].onClick();
    await vi.waitFor(() => expect(deps.refresh).toHaveBeenCalledTimes(2));
    expect(deps.restore).toHaveBeenCalledWith(["a", "b"]);
    expect(deps.notify.error).not.toHaveBeenCalled();
  });

  it("tells the user when the undo window has passed", async () => {
    const { deps, undoable } = makeDeps({
      restore: vi.fn(async () => {
        throw new Error("not found");
      }),
    });
    await deleteWithUndo(false, deps);
    undoable.mock.calls[0][1].onClick();
    await vi.waitFor(() => expect(deps.notify.error).toHaveBeenCalledWith("Too late to undo"));
  });

  it("shows no Undo toast when nothing matched", async () => {
    const { deps, undoable } = makeDeps({ run: vi.fn(async () => result([])) });
    expect(await deleteWithUndo(false, deps)).toEqual([]);
    expect(undoable).not.toHaveBeenCalled();
    expect(deps.notify.info).toHaveBeenCalledWith("Nothing to remove");
  });

  it("reports a failed delete and offers no Undo", async () => {
    const { deps, undoable } = makeDeps({
      run: vi.fn(async () => {
        throw new Error("server down");
      }),
    });
    expect(await deleteWithUndo(false, deps)).toEqual([]);
    expect(undoable).not.toHaveBeenCalled();
    expect(deps.notify.error).toHaveBeenCalledWith("server down");
  });
});

describe("mergeResults", () => {
  it("joins ids from several single deletes", () => {
    expect(mergeResults([result(["a"]), result(["b"])]).ids).toEqual(["a", "b"]);
  });
});

describe("delete API client", () => {
  const originalFetch = globalThis.fetch;
  let calls: { url: string; init?: RequestInit }[];
  beforeEach(() => {
    calls = [];
    (globalThis as { window?: unknown }).window = {};
    globalThis.fetch = vi.fn(async (url: string, init?: RequestInit) => {
      calls.push({ url: String(url), init });
      return { ok: true, status: 200, json: async () => result(["a"]) };
    }) as unknown as typeof fetch;
  });
  afterEach(() => {
    globalThis.fetch = originalFetch;
    delete (globalThis as { window?: unknown }).window;
  });

  it("keeps data by default and sends delete_file=true only on request", async () => {
    await deleteDownload("x");
    await deleteDownload("x", { deleteFile: true });
    await deleteDownloads("completed", { deleteFile: true });
    expect(calls[0].url).not.toContain("delete_file");
    expect(calls[0].init?.method).toBe("DELETE");
    expect(calls[1].url).toContain("/api/v1/downloads/x?delete_file=true");
    expect(calls[2].url).toContain("status=completed");
    expect(calls[2].url).toContain("delete_file=true");
  });

  it("posts the ids to restore", async () => {
    await restoreDownloads(["a", "b"]);
    expect(calls[0].url).toContain("/api/v1/downloads/restore");
    expect(calls[0].init?.method).toBe("POST");
    expect(JSON.parse(String(calls[0].init?.body))).toEqual({ ids: ["a", "b"] });
  });
});
