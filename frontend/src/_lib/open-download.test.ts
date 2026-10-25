import { afterEach, describe, expect, it, vi } from "vitest";

type AppStub = Partial<{
  OpenDownloadFolder: (id: string) => Promise<void>;
  RevealDownload: (id: string) => Promise<void>;
  OpenDownload: (id: string) => Promise<void>;
}>;

function installApp(app?: AppStub) {
  (globalThis as { window?: unknown }).window = app
    ? { go: { main: { App: app } } }
    : {};
}

async function load() {
  vi.resetModules();
  return import("./open-download");
}

afterEach(() => {
  delete (globalThis as { window?: unknown }).window;
});

const completed = { status: "completed" } as const;
const diskFull = { status: "error", error_code: "disk_full" } as const;

describe("openActionsFor", () => {
  it("offers nothing in browser mode (no bindings)", async () => {
    installApp();
    const { openActionsFor } = await load();
    expect(openActionsFor(completed)).toEqual({ folder: false, open: false });
    expect(openActionsFor(diskFull)).toEqual({ folder: false, open: false });
  });

  it("offers folder + open on completed when both bindings exist", async () => {
    installApp({ OpenDownloadFolder: vi.fn(), OpenDownload: vi.fn() });
    const { openActionsFor } = await load();
    expect(openActionsFor(completed)).toEqual({ folder: true, open: true });
  });

  it("offers only the folder on disk_full", async () => {
    installApp({ OpenDownloadFolder: vi.fn(), OpenDownload: vi.fn() });
    const { openActionsFor } = await load();
    expect(openActionsFor(diskFull)).toEqual({ folder: true, open: false });
  });

  it("offers nothing for running or plain-error downloads", async () => {
    installApp({ OpenDownloadFolder: vi.fn(), OpenDownload: vi.fn() });
    const { openActionsFor } = await load();
    expect(openActionsFor({ status: "running" })).toEqual({ folder: false, open: false });
    expect(openActionsFor({ status: "error", error_code: "timeout" })).toEqual({
      folder: false,
      open: false,
    });
  });

  it("hides open when an older shell lacks that binding", async () => {
    installApp({ OpenDownloadFolder: vi.fn() });
    const { openActionsFor } = await load();
    expect(openActionsFor(completed)).toEqual({ folder: true, open: false });
  });
});

describe("running an action", () => {
  it("passes the id to the binding and reports ok", async () => {
    const fn = vi.fn().mockResolvedValue(undefined);
    installApp({ OpenDownloadFolder: fn });
    const { openDownloadFolder } = await load();
    expect(await openDownloadFolder("j1")).toEqual({ status: "ok" });
    expect(fn).toHaveBeenCalledWith("j1");
  });

  it("reports unavailable without a binding", async () => {
    installApp();
    const { openDownload } = await load();
    expect(await openDownload("j1")).toEqual({ status: "unavailable" });
  });

  it("surfaces a rejected call as failed", async () => {
    installApp({ OpenDownload: vi.fn().mockRejectedValue(new Error("path not found")) });
    const { openDownload } = await load();
    expect(await openDownload("j1")).toEqual({ status: "failed", message: "path not found" });
  });
});
