// Builds dist/chrome and dist/firefox (+ rum-chrome.zip, rum-firefox.xpi).
// Plain Node, no dependencies.
import { cpSync, mkdirSync, readdirSync, readFileSync, rmSync, statSync, writeFileSync } from "node:fs";
import { join, relative } from "node:path";
import { ROOT, SRC, TARGETS, makeManifest } from "./manifest.mjs";
import { zip } from "./zip.mjs";

const DIST = join(ROOT, "dist");

function listFiles(dir) {
  return readdirSync(dir).flatMap((n) => {
    const p = join(dir, n);
    return statSync(p).isDirectory() ? listFiles(p) : [p];
  });
}

rmSync(DIST, { recursive: true, force: true });

for (const target of TARGETS) {
  const out = join(DIST, target);
  mkdirSync(out, { recursive: true });
  cpSync(SRC, out, { recursive: true, filter: (p) => !p.endsWith("manifest.base.json") });
  writeFileSync(join(out, "manifest.json"), JSON.stringify(makeManifest(target), null, 2) + "\n");

  const files = listFiles(out).map((p) => ({
    name: relative(out, p).split("\\").join("/"),
    data: readFileSync(p),
  }));
  const pkg = join(DIST, target === "firefox" ? "rum-firefox.xpi" : "rum-chrome.zip");
  writeFileSync(pkg, zip(files));
  console.log(`${target}: ${relative(ROOT, out)}/  ->  ${relative(ROOT, pkg)} (${files.length} files)`);
}
