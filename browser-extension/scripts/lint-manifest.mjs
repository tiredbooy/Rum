// Validates the generated manifests. Exits non-zero on any problem.
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { ROOT, SRC, TARGETS, makeManifest } from "./manifest.mjs";

const ALLOWED_PERMISSIONS = new Set(["contextMenus", "downloads", "cookies", "storage"]);

/** Returns a list of problems (empty when the manifest is fine). */
export function lintManifest(m, target, fileExists = (p) => existsSync(join(SRC, p)), version) {
  const errs = [];
  const need = (cond, msg) => cond || errs.push(`${target}: ${msg}`);

  need(m.manifest_version === 3, "manifest_version must be 3");
  need(typeof m.name === "string" && m.name.length > 0 && m.name.length <= 45, "name required (max 45 chars)");
  need(typeof m.description === "string" && m.description.length <= 132, "description required (max 132 chars)");
  need(/^\d+(\.\d+){0,3}$/.test(m.version ?? ""), "version must be dotted numbers");
  if (version) need(m.version === version, `version ${m.version} != package.json ${version}`);

  for (const p of m.permissions ?? []) need(ALLOWED_PERMISSIONS.has(p), `unexpected permission "${p}"`);
  for (const p of ALLOWED_PERMISSIONS) need((m.permissions ?? []).includes(p), `missing permission "${p}"`);
  need(!("optional_permissions" in m) && !("content_scripts" in m), "no content scripts / optional permissions expected");

  const files = [
    ...Object.values(m.icons ?? {}),
    ...Object.values(m.action?.default_icon ?? {}),
    m.action?.default_popup,
    m.options_ui?.page,
    m.background?.service_worker,
    ...(m.background?.scripts ?? []),
  ].filter(Boolean);
  for (const f of files) need(fileExists(f), `referenced file missing: ${f}`);

  if (target === "chrome") {
    need(!!m.background?.service_worker, "chrome needs background.service_worker");
    need(!m.background?.scripts, "chrome must not use background.scripts");
    need(!m.browser_specific_settings, "chrome must not carry browser_specific_settings");
  } else {
    need(Array.isArray(m.background?.scripts) && m.background.scripts.length > 0, "firefox needs background.scripts");
    need(!m.background?.service_worker, "firefox must not use service_worker");
    need(/^[^@\s]+@[^@\s]+$/.test(m.browser_specific_settings?.gecko?.id ?? ""), "firefox needs gecko.id like name@domain");
  }
  return errs;
}

if (import.meta.url === `file://${process.argv[1]}`) {
  const version = JSON.parse(readFileSync(join(ROOT, "package.json"), "utf8")).version;
  const errs = TARGETS.flatMap((t) => lintManifest(makeManifest(t), t, undefined, version));
  if (errs.length) {
    console.error(errs.join("\n"));
    process.exit(1);
  }
  console.log(`manifests OK (${TARGETS.join(", ")})`);
}
