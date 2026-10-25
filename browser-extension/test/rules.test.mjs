import test from "node:test";
import assert from "node:assert/strict";
import {
  DEFAULTS,
  basename,
  buildAddPayload,
  cookieHeader,
  isCapturableUrl,
  normalizeSettings,
  pickMenuUrl,
  portCandidates,
  shouldCapture,
} from "../src/lib/rules.js";

const MB = 1024 * 1024;
const cfg = normalizeSettings({ token: "rum_x", autoCapture: true, minSizeMB: 5 });
const item = (o = {}) => ({
  id: 1,
  url: "https://cdn.example/big.iso",
  state: "in_progress",
  incognito: false,
  totalBytes: 100 * MB,
  ...o,
});

test("normalizeSettings applies defaults and clamps", () => {
  assert.deepEqual(normalizeSettings(undefined), DEFAULTS);
  assert.equal(DEFAULTS.port, 8080);
  assert.equal(DEFAULTS.autoCapture, true);
  assert.equal(DEFAULTS.minSizeMB, 5);
  const n = normalizeSettings({ port: "99999", minSizeMB: -3, token: "  rum_a \n", autoCapture: false });
  assert.equal(n.port, 8080);
  assert.equal(n.minSizeMB, 5);
  assert.equal(n.token, "rum_a");
  assert.equal(n.autoCapture, false);
  assert.equal(normalizeSettings({ port: "8081" }).port, 8081);
  assert.equal(normalizeSettings({ minSizeMB: "0" }).minSizeMB, 0);
});

test("isCapturableUrl accepts only http(s)", () => {
  assert.equal(isCapturableUrl("https://a.example/f"), true);
  assert.equal(isCapturableUrl("http://a.example/f"), true);
  for (const u of ["blob:https://a/uuid", "data:text/plain,hi", "file:///etc/passwd", "ftp://a/f", "", "nope", undefined]) {
    assert.equal(isCapturableUrl(u), false, String(u));
  }
});

test("shouldCapture: size threshold", () => {
  assert.equal(shouldCapture(item({ totalBytes: 5 * MB }), cfg).capture, true);
  assert.equal(shouldCapture(item({ totalBytes: 5 * MB - 1 }), cfg).capture, false);
  assert.equal(shouldCapture(item({ totalBytes: -1 }), cfg).reason, "unknown-size");
  assert.equal(shouldCapture(item({ totalBytes: 0, fileSize: 9 * MB }), cfg).capture, true);
  const big = normalizeSettings({ token: "t", minSizeMB: 100 });
  assert.equal(shouldCapture(item({ totalBytes: 50 * MB }), big).capture, false);
});

test("shouldCapture: guards", () => {
  assert.equal(shouldCapture(item(), { ...cfg, autoCapture: false }).reason, "disabled");
  assert.equal(shouldCapture(item(), { ...cfg, token: "" }).reason, "not-paired");
  assert.equal(shouldCapture(item({ url: "blob:https://x/1" }), cfg).reason, "scheme");
  assert.equal(shouldCapture(item({ url: "data:application/zip;base64,AA" }), cfg).reason, "scheme");
  assert.equal(shouldCapture(item({ incognito: true }), cfg).reason, "incognito");
  assert.equal(shouldCapture(item({ state: "complete" }), cfg).reason, "state");
});

test("basename handles both separators", () => {
  assert.equal(basename("/home/u/Downloads/a b.zip"), "a b.zip");
  assert.equal(basename("C:\\Users\\u\\Downloads\\a.zip"), "a.zip");
  assert.equal(basename(""), "");
  assert.equal(basename(undefined), "");
  assert.equal(basename("dir/"), "");
});

test("cookieHeader joins name=value pairs", () => {
  assert.equal(cookieHeader([{ name: "a", value: "1" }, { name: "b", value: "x y" }]), "a=1; b=x y");
  assert.equal(cookieHeader([{ name: "", value: "1" }]), "");
  assert.equal(cookieHeader([]), "");
  assert.equal(cookieHeader(undefined), "");
});

test("buildAddPayload omits empties", () => {
  assert.deepEqual(buildAddPayload({ url: "https://a/f" }), { url: "https://a/f" });
  assert.deepEqual(
    buildAddPayload({ url: "https://a/f", filename: "f.zip", referer: "https://a/", userAgent: "UA", cookie: "a=1" }),
    { url: "https://a/f", filename: "f.zip", referer: "https://a/", user_agent: "UA", cookie: "a=1" },
  );
});

test("pickMenuUrl prefers media src, then link, skips non-http", () => {
  assert.equal(pickMenuUrl({ mediaType: "image", srcUrl: "https://a/i.png", linkUrl: "https://a/page" }), "https://a/i.png");
  assert.equal(pickMenuUrl({ linkUrl: "https://a/file.zip" }), "https://a/file.zip");
  assert.equal(pickMenuUrl({ mediaType: "video", srcUrl: "blob:https://a/1" }), "");
  assert.equal(pickMenuUrl({}), "");
});

test("portCandidates starts at the configured port", () => {
  assert.deepEqual(portCandidates(8080, 3), [8080, 8081, 8082]);
  assert.equal(portCandidates(65535, 3).length, 1);
});

test("statusView covers every state", async () => {
  const { statusView } = await import("../src/lib/rules.js");
  assert.equal(statusView("paired").tone, "ok");
  assert.equal(statusView("paired").canOpen, true);
  assert.equal(statusView("not_paired").tone, "warn");
  assert.equal(statusView("not_running").tone, "bad");
  assert.equal(statusView(undefined).canOpen, false);
});
