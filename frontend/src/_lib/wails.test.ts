import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * The Wails bridge is the layer the folder-picker bug lived in: a rejected
 * binding call was swallowed into the same `null` a cancel produces, so a
 * picker that could not open was indistinguishable from a button that does
 * nothing. These tests pin the four outcomes apart.
 *
 * `wails.ts` reads `window.go` at call time, so a plain object stand-in is
 * enough — no DOM needed.
 */

type AppStub = Partial<{
  GetApiBase: () => unknown;
  ChooseDir: () => unknown;
  Capabilities: () => unknown;
}>;

function installWindow(app?: AppStub, runtime?: unknown) {
  (globalThis as { window?: unknown }).window = {
    ...(app ? { go: { main: { App: app } } } : {}),
    ...(runtime ? { runtime } : {}),
  };
}

/** Fresh module instance so the memoized API base never leaks between tests. */
async function loadWails() {
  vi.resetModules();
  return import("./wails");
}

beforeEach(() => installWindow());
afterEach(() => {
  delete (globalThis as { window?: unknown }).window;
});

describe("chooseDir", () => {
  it("reports 'unavailable' when the desktop binding is absent", async () => {
    installWindow(); // browser/dev: no window.go at all
    const { chooseDir } = await loadWails();
    await expect(chooseDir()).resolves.toEqual({ status: "unavailable" });
  });

  it("returns the picked path", async () => {
    installWindow({ ChooseDir: () => Promise.resolve("/home/me/Downloads") });
    const { chooseDir } = await loadWails();
    await expect(chooseDir()).resolves.toEqual({
      status: "picked",
      path: "/home/me/Downloads",
    });
  });

  it("treats an empty result as a cancel, not a failure", async () => {
    installWindow({ ChooseDir: () => Promise.resolve("") });
    const { chooseDir } = await loadWails();
    await expect(chooseDir()).resolves.toEqual({ status: "cancelled" });
  });

  it("surfaces a rejected picker as a failure with its message", async () => {
    installWindow({
      ChooseDir: () => Promise.reject(new Error("could not open the folder picker")),
    });
    const { chooseDir } = await loadWails();
    await expect(chooseDir()).resolves.toEqual({
      status: "failed",
      message: "could not open the folder picker",
    });
  });

  it("does not treat a whitespace-only path as a real selection", async () => {
    installWindow({ ChooseDir: () => Promise.resolve("   ") });
    const { chooseDir } = await loadWails();
    await expect(chooseDir()).resolves.toEqual({ status: "cancelled" });
  });
});

describe("hasChooseDir", () => {
  it("is false without the desktop shell", async () => {
    installWindow();
    const { hasChooseDir } = await loadWails();
    expect(hasChooseDir()).toBe(false);
  });

  it("is true once the binding is injected", async () => {
    installWindow({ ChooseDir: () => Promise.resolve("/x") });
    const { hasChooseDir } = await loadWails();
    expect(hasChooseDir()).toBe(true);
  });
});

describe("getCapabilities", () => {
  it("falls back to web-safe defaults with no shell", async () => {
    installWindow();
    const { getCapabilities } = await loadWails();
    await expect(getCapabilities()).resolves.toEqual({
      tray: false,
      folderPicker: false,
      platform: "web",
    });
  });

  it("reports what the shell says", async () => {
    installWindow({
      Capabilities: () =>
        Promise.resolve({ tray: true, folderPicker: true, platform: "windows" }),
    });
    const { getCapabilities } = await loadWails();
    await expect(getCapabilities()).resolves.toEqual({
      tray: true,
      folderPicker: true,
      platform: "windows",
    });
  });

  it("degrades gracefully when the binding throws", async () => {
    installWindow({
      Capabilities: () => Promise.reject(new Error("boom")),
      ChooseDir: () => Promise.resolve("/x"),
    });
    const { getCapabilities } = await loadWails();
    await expect(getCapabilities()).resolves.toEqual({
      tray: false,
      folderPicker: true, // the ChooseDir binding IS present
      platform: "web",
    });
  });
});

describe("getApiBase", () => {
  it("prefers the binding's loopback base", async () => {
    installWindow({ GetApiBase: () => Promise.resolve("http://127.0.0.1:8082/") });
    const { getApiBase, apiBaseSync } = await loadWails();
    await expect(getApiBase()).resolves.toBe("http://127.0.0.1:8082");
    expect(apiBaseSync()).toBe("http://127.0.0.1:8082");
  });

  it("keeps the fallback when the binding throws", async () => {
    installWindow({
      GetApiBase: () => {
        throw new Error("nope");
      },
    });
    const { getApiBase } = await loadWails();
    await expect(getApiBase()).resolves.toMatch(/^http:\/\//);
  });
});

describe("onWailsEvent", () => {
  it("is a no-op unsubscribe outside the desktop shell", async () => {
    installWindow();
    const { onWailsEvent } = await loadWails();
    const off = onWailsEvent("clipboard:url", () => {});
    expect(() => off()).not.toThrow();
  });

  it("subscribes through the Wails runtime and hands back its unsubscribe", async () => {
    const off = vi.fn();
    const EventsOn = vi.fn(() => off);
    installWindow(undefined, { EventsOn });

    const { onWailsEvent } = await loadWails();
    const handler = () => {};
    const unsubscribe = onWailsEvent("clipboard:url", handler);

    expect(EventsOn).toHaveBeenCalledWith("clipboard:url", handler);
    unsubscribe();
    expect(off).toHaveBeenCalled();
  });
});
