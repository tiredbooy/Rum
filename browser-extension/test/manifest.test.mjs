import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { inflateRawSync } from "node:zlib";
import { ROOT, TARGETS, makeManifest } from "../scripts/manifest.mjs";
import { lintManifest } from "../scripts/lint-manifest.mjs";
import { crc32, zip } from "../scripts/zip.mjs";

const version = JSON.parse(readFileSync(join(ROOT, "package.json"), "utf8")).version;

for (const target of TARGETS) {
  test(`${target} manifest passes lint`, () => {
    assert.deepEqual(lintManifest(makeManifest(target), target, undefined, version), []);
  });
}

test("lint catches a missing file and a stray permission", () => {
  const m = makeManifest("chrome");
  m.permissions.push("history");
  const errs = lintManifest(m, "chrome", (f) => f !== "popup.html");
  assert.ok(errs.some((e) => e.includes("history")));
  assert.ok(errs.some((e) => e.includes("popup.html")));
});

test("firefox variant has gecko id + background.scripts, chrome has service_worker", () => {
  const ff = makeManifest("firefox");
  const ch = makeManifest("chrome");
  assert.ok(ff.browser_specific_settings.gecko.id.includes("@"));
  assert.deepEqual(ff.background.scripts, ["background.js"]);
  assert.equal(ch.background.service_worker, "background.js");
  assert.equal(ch.background.scripts, undefined);
});

test("zip writes a readable archive", () => {
  const data = Buffer.from("hello hello hello hello hello");
  const out = zip([{ name: "a/b.txt", data }]);
  assert.equal(out.readUInt32LE(0), 0x04034b50);
  assert.equal(out.readUInt32LE(14), crc32(data));
  const nameLen = out.readUInt16LE(26);
  const compSize = out.readUInt32LE(18);
  const body = out.subarray(30 + nameLen, 30 + nameLen + compSize);
  const method = out.readUInt16LE(8);
  assert.equal((method === 8 ? inflateRawSync(body) : body).toString(), data.toString());
  assert.equal(out.readUInt32LE(out.length - 22), 0x06054b50);
});
