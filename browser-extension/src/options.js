import { loadSettings, saveSettings } from "./lib/settings.js";
import { findRum } from "./lib/rum.js";
import { normalizeSettings, statusView } from "./lib/rules.js";

const $ = (id) => document.getElementById(id);

async function init() {
  const s = await loadSettings();
  $("token").value = s.token;
  $("port").value = s.port;
  $("auto").checked = s.autoCapture;
  $("min").value = s.minSizeMB;

  $("form").addEventListener("submit", async (e) => {
    e.preventDefault();
    const next = normalizeSettings({
      token: $("token").value,
      port: $("port").value,
      autoCapture: $("auto").checked,
      minSizeMB: $("min").value,
    });
    await saveSettings(next);
    // Show what was actually stored (invalid values fall back to defaults).
    $("port").value = next.port;
    $("min").value = next.minSizeMB;
    $("token").value = next.token;

    $("result").textContent = "Checking…";
    const found = await findRum(next);
    $("result").textContent = `Saved. ${statusView(found.state).label}`;
  });
}

init();
