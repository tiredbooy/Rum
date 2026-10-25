// Drives background.js against a fake browser to check the capture ordering:
// the browser download is cancelled only AFTER Rum accepted the job.
import test from "node:test";
import assert from "node:assert/strict";

const MB = 1024 * 1024;
const log = [];
const listeners = {};
const store = { token: "rum_t", autoCapture: true, minSizeMB: 5, port: 8080 };
let rum = { up: true, addStatus: 201 };

const ev = (name) => ({ addListener: (f) => (listeners[name] = f) });
globalThis.chrome = {
  storage: { local: { get: async () => ({ ...store }), set: async () => {} } },
  downloads: {
    onCreated: ev("created"),
    pause: async (id) => log.push(["pause", id]),
    resume: async (id) => log.push(["resume", id]),
    cancel: async (id) => log.push(["cancel", id]),
    erase: async (q) => log.push(["erase", q.id]),
    search: async () => [],
    download: async (o) => log.push(["browserDownload", o.url]),
  },
  cookies: { getAll: async () => [{ name: "sid", value: "abc" }] },
  tabs: { query: async () => [{ url: "https://tab.example/page" }] },
  action: { setBadgeText: async () => {}, setBadgeBackgroundColor: async () => {} },
  contextMenus: { removeAll: async () => {}, create: () => {}, onClicked: ev("menu") },
  runtime: { onInstalled: ev("installed") },
};
globalThis.fetch = async (url, init) => {
  if (!rum.up) throw new TypeError("refused");
  const headers = { get: (h) => (h === "X-Rum-App" ? "rum" : null) };
  if (url.endsWith("/ext/ping")) {
    const authed = !!init.headers["X-Rum-Token"];
    return { ok: authed, status: authed ? 200 : 401, headers, json: async () => ({ app: "rum", code: "unauthorized" }) };
  }
  if (url.endsWith("/ext/add")) {
    log.push(["add", JSON.parse(init.body)]);
    return { ok: rum.addStatus < 300, status: rum.addStatus, headers, json: async () => ({}) };
  }
  throw new Error("unexpected " + url);
};
Object.defineProperty(globalThis, "navigator", { value: { userAgent: "TestUA/1" }, configurable: true });

await import("../src/background.js");

const big = (id, o = {}) => ({
  id, url: `https://cdn.example/f${id}.iso`, state: "in_progress", incognito: false,
  totalBytes: 50 * MB, referrer: "https://site.example/dl", filename: "/home/u/Downloads/f.iso", ...o,
});
const run = async (item) => {
  log.length = 0;
  listeners.created(item);
  await new Promise((r) => setTimeout(r, 60));
  return log.map((e) => e[0]);
};

test("accepted: pause, add (with headers+cookie), then cancel and erase", async () => {
  rum = { up: true, addStatus: 201 };
  assert.deepEqual(await run(big(1)), ["pause", "add", "cancel", "erase"]);
  const add = log.find((e) => e[0] === "add")[1];
  assert.equal(add.url, "https://cdn.example/f1.iso");
  assert.equal(add.referer, "https://site.example/dl");
  assert.equal(add.user_agent, "TestUA/1");
  assert.equal(add.cookie, "sid=abc");
  assert.equal(add.filename, "f.iso");
});

test("rejected by Rum: the browser download is resumed, never cancelled", async () => {
  rum = { up: true, addStatus: 502 };
  assert.deepEqual(await run(big(2)), ["pause", "add", "resume"]);
});

test("Rum not running: the browser download is left completely alone", async () => {
  rum = { up: false, addStatus: 201 };
  assert.deepEqual(await run(big(3)), []);
});

test("small, blob and incognito downloads are not touched", async () => {
  rum = { up: true, addStatus: 201 };
  assert.deepEqual(await run(big(4, { totalBytes: 1 * MB })), []);
  assert.deepEqual(await run(big(5, { url: "blob:https://x/uuid" })), []);
  assert.deepEqual(await run(big(6, { incognito: true })), []);
});

test("auto-capture off: nothing is captured", async () => {
  rum = { up: true, addStatus: 201 };
  store.autoCapture = false;
  assert.deepEqual(await run(big(7)), []);
  store.autoCapture = true;
});

test("context menu: Rum down falls back to a normal browser download", async () => {
  rum = { up: false, addStatus: 201 };
  log.length = 0;
  listeners.menu({ menuItemId: "rum-download", linkUrl: "https://a.example/file.zip" }, { url: "https://a.example/" });
  await new Promise((r) => setTimeout(r, 60));
  assert.deepEqual(log.map((e) => e[0]), ["browserDownload"]);
});

test("context menu: Rum up adds the job", async () => {
  rum = { up: true, addStatus: 201 };
  log.length = 0;
  listeners.menu({ menuItemId: "rum-download", mediaType: "video", srcUrl: "https://a.example/v.mp4", pageUrl: "https://a.example/p" }, {});
  await new Promise((r) => setTimeout(r, 60));
  assert.deepEqual(log.map((e) => e[0]), ["add"]);
  assert.equal(log[0][1].referer, "https://a.example/p");
});
