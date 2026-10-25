import { ext } from "./ns.js";
import { DEFAULTS, normalizeSettings } from "./rules.js";

export async function loadSettings() {
  try {
    return normalizeSettings(await ext.storage.local.get(Object.keys(DEFAULTS)));
  } catch {
    return normalizeSettings(undefined);
  }
}

export async function saveSettings(patch) {
  await ext.storage.local.set(patch);
}
