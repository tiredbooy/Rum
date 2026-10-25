// Background (service worker on Chrome/Edge/Brave, background script on Firefox).
import { ext } from "./lib/ns.js";
import { loadSettings } from "./lib/settings.js";
import { addDownload, addVideo, findRum } from "./lib/rum.js";
import { basename, buildAddPayload, cookieHeader, pickMenuUrl, shouldCapture } from "./lib/rules.js";

const MENU_ID = "rum-download";
const VIDEO_MENU_ID = "rum-video";
const SIZE_POLLS = 5;
const SIZE_POLL_MS = 400;
const SUPPRESS_MS = 60_000;

/** Browser download ids we already decided about. */
const handled = new Set();
/** Names Chrome settled on (onDeterminingFilename), by download id. */
const suggested = new Map();
/** URLs we deliberately gave back to the browser; do not capture them again. */
const suppressed = new Map();

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

function suppress(url) {
  suppressed.set(url, Date.now() + SUPPRESS_MS);
}

function isSuppressed(url) {
  const until = suppressed.get(url);
  if (until && until > Date.now()) return true;
  suppressed.delete(url);
  return false;
}

/** Brief badge feedback: "OK" when Rum took the download, "!" when it did not. */
async function flash(good) {
  try {
    await ext.action.setBadgeBackgroundColor({ color: good ? "#16a34a" : "#dc2626" });
    await ext.action.setBadgeText({ text: good ? "OK" : "!" });
    setTimeout(() => ext.action.setBadgeText({ text: "" }).catch(() => {}), 3000);
  } catch {
    // cosmetic only
  }
}

async function cookiesFor(url, storeId) {
  try {
    const query = storeId ? { url, storeId } : { url };
    return cookieHeader(await ext.cookies.getAll(query));
  } catch {
    return ""; // permission missing or first-party isolation: send without cookies
  }
}

async function activeTabUrl() {
  try {
    const [tab] = await ext.tabs.query({ active: true, lastFocusedWindow: true });
    return tab?.url || "";
  } catch {
    return "";
  }
}

// ---- auto-capture -----------------------------------------------------------

async function waitForSize(item) {
  let cur = item;
  for (let i = 0; i < SIZE_POLLS; i++) {
    if (Math.max(cur.totalBytes ?? -1, cur.fileSize ?? -1) > 0 || cur.state !== "in_progress") return cur;
    await sleep(SIZE_POLL_MS);
    try {
      const [fresh] = await ext.downloads.search({ id: item.id });
      if (!fresh) return cur;
      cur = fresh;
    } catch {
      return cur;
    }
  }
  return cur;
}

async function captureDownload(item) {
  if (handled.has(item.id) || isSuppressed(item.url)) return;
  const settings = await loadSettings();

  let current = item;
  let verdict = shouldCapture(current, settings);
  if (verdict.reason === "unknown-size") {
    current = await waitForSize(current);
    verdict = shouldCapture(current, settings);
  }
  if (!verdict.capture) return;

  // Never interfere unless Rum is up and paired.
  const found = await findRum(settings);
  if (found.state !== "paired") return;

  handled.add(item.id);
  try {
    await ext.downloads.pause(item.id); // do not race Rum for bandwidth
  } catch {
    // already finished or not pausable; carry on
  }

  const payload = buildAddPayload({
    url: current.url,
    filename: suggested.get(item.id) || basename(current.filename),
    referer: current.referrer || (await activeTabUrl()),
    userAgent: navigator.userAgent,
    cookie: await cookiesFor(current.url, current.cookieStoreId),
  });
  const result = await addDownload(payload, { ...settings, port: found.port });
  suggested.delete(item.id);

  if (result.ok) {
    // Rum owns the job now: only here is it safe to drop the browser's copy.
    try {
      await ext.downloads.cancel(item.id);
      await ext.downloads.erase({ id: item.id });
    } catch {
      // already complete: leave the file, Rum has the job too
    }
    flash(true);
  } else {
    try {
      await ext.downloads.resume(item.id); // never lose the user's download
    } catch {
      // nothing to resume
    }
    flash(false);
  }
}

ext.downloads.onCreated.addListener((item) => {
  captureDownload(item).catch(async () => {
    try {
      await ext.downloads.resume(item.id);
    } catch {
      // ignore
    }
  });
});

// Chrome only: remember the final file name (Content-Disposition resolved).
if (ext.downloads.onDeterminingFilename) {
  ext.downloads.onDeterminingFilename.addListener((item, suggest) => {
    if (item.filename) suggested.set(item.id, basename(item.filename));
    try {
      suggest(); // keep the browser's own choice
    } catch {
      // ignore
    }
  });
}

// ---- context menu -----------------------------------------------------------

async function downloadWithRum(info, tab) {
  const url = pickMenuUrl(info);
  if (!url) return flash(false);

  const settings = await loadSettings();
  const found = await findRum(settings);
  if (found.state === "paired") {
    const payload = buildAddPayload({
      url,
      referer: info.frameUrl || info.pageUrl || tab?.url || "",
      userAgent: navigator.userAgent,
      cookie: await cookiesFor(url),
    });
    const result = await addDownload(payload, { ...settings, port: found.port });
    if (result.ok) return flash(true);
  }

  // Rum is unreachable or said no: the browser downloads it instead.
  suppress(url);
  try {
    await ext.downloads.download({ url });
  } catch {
    // browser refused too (e.g. unsupported scheme); nothing more to do
  }
  flash(false);
}

/** "Download video with Rum" on a page: Rum lists the formats and takes the best. */
async function downloadVideoWithRum(info, tab) {
  const url = info.pageUrl || tab?.url || "";
  if (!/^https?:\/\//i.test(url)) return flash(false);
  const settings = await loadSettings();
  const found = await findRum(settings);
  if (found.state !== "paired") return flash(false);
  const result = await addVideo(url, { ...settings, port: found.port });
  flash(result.ok);
}

async function createMenu() {
  await ext.contextMenus.removeAll(); // promise form works on Chrome MV3 and Firefox
  ext.contextMenus.create({
    id: MENU_ID,
    title: "Download with Rum",
    contexts: ["link", "image", "video", "audio"],
  });
  ext.contextMenus.create({
    id: VIDEO_MENU_ID,
    title: "Download video with Rum",
    contexts: ["page"],
  });
}

ext.runtime.onInstalled.addListener(() => createMenu().catch(() => {}));
ext.contextMenus.onClicked.addListener((info, tab) => {
  if (info.menuItemId === MENU_ID) downloadWithRum(info, tab).catch(() => flash(false));
  if (info.menuItemId === VIDEO_MENU_ID) downloadVideoWithRum(info, tab).catch(() => flash(false));
});
