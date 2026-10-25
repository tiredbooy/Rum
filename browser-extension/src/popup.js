import { ext } from "./lib/ns.js";
import { loadSettings, saveSettings } from "./lib/settings.js";
import { findRum, openRum } from "./lib/rum.js";
import { statusView } from "./lib/rules.js";

const $ = (id) => document.getElementById(id);

async function init() {
  const settings = await loadSettings();
  const auto = $("auto");
  auto.checked = settings.autoCapture;
  auto.addEventListener("change", () => saveSettings({ autoCapture: auto.checked }));

  $("options").addEventListener("click", () => {
    ext.runtime.openOptionsPage();
    window.close();
  });

  const found = await findRum(settings);
  const view = statusView(found.state);
  $("status").textContent = view.label;
  $("dot").className = `dot ${view.tone}`;

  const open = $("open");
  open.disabled = !view.canOpen;
  open.addEventListener("click", async () => {
    if (await openRum({ ...settings, port: found.port })) window.close();
    else $("status").textContent = "Could not open Rum";
  });
}

init();
