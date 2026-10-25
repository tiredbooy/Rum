// Builds the per-browser manifests from src/manifest.base.json.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

export const ROOT = join(dirname(fileURLToPath(import.meta.url)), "..");
export const SRC = join(ROOT, "src");

/** Firefox needs a stable add-on id to keep storage and signing consistent. */
export const GECKO_ID = "rum-downloader@rum.app";

export const TARGETS = ["chrome", "firefox"];

export function readBase() {
  return JSON.parse(readFileSync(join(SRC, "manifest.base.json"), "utf8"));
}

export function makeManifest(target, base = readBase()) {
  const m = structuredClone(base);
  if (target === "chrome") {
    m.minimum_chrome_version = "116";
    m.background = { service_worker: "background.js", type: "module" };
  } else if (target === "firefox") {
    m.background = { scripts: ["background.js"], type: "module" };
    m.browser_specific_settings = {
      gecko: {
        id: GECKO_ID,
        strict_min_version: "128.0",
        data_collection_permissions: { required: ["none"] },
      },
    };
  } else {
    throw new Error(`unknown target ${target}`);
  }
  return m;
}
