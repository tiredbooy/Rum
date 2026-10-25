import test from "node:test";
import assert from "node:assert/strict";
import { addDownload, addVideo, findRum, openRum } from "../src/lib/rum.js";

const settings = { port: 8080, token: "rum_t" };
const RUM = { get: (h) => (h === "X-Rum-App" ? "rum" : null) };
const NOT_RUM = { get: () => null };
const res = (status, body, headers = RUM) => ({ ok: status < 300, status, headers, json: async () => body });
const asRum = (okBody) => async (_u, init) =>
  init.headers["X-Rum-Token"] ? res(200, okBody) : res(401, { code: "unauthorized" });
const refused = () => Promise.reject(new TypeError("fetch failed"));

test("findRum: paired on 200 and sends the token header", async () => {
  const calls = [];
  const fetchImpl = async (url, init) => {
    calls.push([url, init]);
    return init.headers["X-Rum-Token"] ? res(200, { ok: true, app: "rum" }) : res(401, { code: "unauthorized" });
  };
  const r = await findRum(settings, fetchImpl);
  assert.deepEqual([r.state, r.port], ["paired", 8080]);
  const authed = calls.filter(([u, init]) => u.includes(":8080/") && init.headers["X-Rum-Token"]);
  assert.equal(authed.length, 1);
  assert.equal(authed[0][0], "http://127.0.0.1:8080/ext/ping");
  assert.equal(authed[0][1].headers["X-Rum-Token"], "rum_t");
});

test("findRum: not_paired on 401 from Rum", async () => {
  const r = await findRum(settings, async () => res(401, { code: "unauthorized" }));
  assert.equal(r.state, "not_paired");
  const noToken = await findRum({ ...settings, token: "" }, async () => res(401, {}));
  assert.equal(noToken.state, "not_paired");
});

test("findRum: not_running when nothing answers", async () => {
  const r = await findRum(settings, refused);
  assert.equal(r.state, "not_running");
});

test("findRum: discovers Rum on a neighbouring port", async () => {
  const rum = asRum({ ok: true, app: "rum" });
  const fetchImpl = async (url, init) => (url.includes(":8083/") ? rum(url, init) : refused());
  const r = await findRum(settings, fetchImpl);
  assert.deepEqual([r.state, r.port], ["paired", 8083]);
});

test("findRum never sends the token to a service that is not Rum", async () => {
  const seen = [];
  const r = await findRum(settings, async (_u, init) => {
    seen.push(init.headers["X-Rum-Token"]);
    return res(200, { hello: "world" }, NOT_RUM);
  });
  assert.equal(r.state, "not_running");
  assert.ok(seen.length > 0 && seen.every((t) => t === undefined), "token leaked to a non-Rum port");
});

test("addDownload: true only on 2xx", async () => {
  let body;
  const ok = await addDownload({ url: "https://a/f" }, settings, async (u, init) => {
    body = JSON.parse(init.body);
    assert.equal(u, "http://127.0.0.1:8080/ext/add");
    assert.equal(init.method, "POST");
    return res(201, { id: "1" });
  });
  assert.equal(ok.ok, true);
  assert.equal(body.url, "https://a/f");
  assert.equal((await addDownload({ url: "x" }, settings, async () => res(502, {}))).ok, false);
  assert.equal((await addDownload({ url: "x" }, settings, async () => res(401, {}))).ok, false);
  assert.equal((await addDownload({ url: "x" }, settings, refused)).ok, false);
});

test("openRum posts to /ext/open", async () => {
  let url;
  const ok = await openRum(settings, async (u) => ((url = u), res(200, { ok: true })));
  assert.equal(ok, true);
  assert.equal(url, "http://127.0.0.1:8080/ext/open");
});

test("addVideo posts the page url to /ext/video and reports Rum's error code", async () => {
  let seen;
  const ok = await addVideo("https://www.youtube.com/watch?v=x", settings, async (u, init) => {
    seen = [u, init.method, JSON.parse(init.body)];
    return res(201, { id: "1" });
  });
  assert.equal(ok.ok, true);
  assert.deepEqual(seen, ["http://127.0.0.1:8080/ext/video", "POST", { url: "https://www.youtube.com/watch?v=x" }]);
  const missing = await addVideo("https://a/v", settings, async () => res(424, { code: "ytdlp_missing" }));
  assert.deepEqual([missing.ok, missing.code], [false, "ytdlp_missing"]);
  assert.equal((await addVideo("https://a/v", settings, refused)).ok, false);
});
