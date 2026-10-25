# Rum browser extension

Sends downloads from your browser to Rum. One codebase for Chrome, Edge, Brave
and Firefox (Manifest V3).

- **Right-click, Download with Rum** on links, images, video and audio.
- **Auto-capture**: browser downloads of 5 MB or more go to Rum instead. The
  browser's copy is cancelled only after Rum has accepted the job. Size and
  on/off are in the options page.
- Sends the URL, file name, referer, user-agent and the cookies for that URL, so
  logged-in downloads work.
- **Never loses a download**: if Rum is not running, not paired, or says no, the
  browser downloads the file as usual.
- Skips `blob:` and `data:` links and private/incognito windows.

## Build

Needs Node 20+. No dependencies.

```sh
cd browser-extension
npm run build   # dist/chrome, dist/firefox, dist/rum-chrome.zip, dist/rum-firefox.xpi
npm test        # unit tests (node:test)
npm run lint    # manifest checks
```

## Install

### Chrome, Edge, Brave

1. Open `chrome://extensions` (Edge: `edge://extensions`, Brave: `brave://extensions`).
2. Turn on **Developer mode**.
3. Click **Load unpacked** and pick `browser-extension/dist/chrome`.

The Chrome Web Store needs a one-time $5 developer fee, so this project
distributes the unpacked folder. If you publish it there, upload
`dist/rum-chrome.zip`.

### Firefox

Temporary (until Firefox restarts), good for trying it:

1. Open `about:debugging#/runtime/this-firefox`.
2. Click **Load Temporary Add-on** and pick
   `browser-extension/dist/firefox/manifest.json`.

Permanent: Firefox only keeps add-ons that Mozilla has signed. Signing is free
for self-distribution:

1. Create an account on [addons.mozilla.org](https://addons.mozilla.org/developers/).
2. **Submit a New Add-on**, choose **On your own** (unlisted), upload
   `dist/rum-firefox.xpi`.
3. Download the signed `.xpi` once validation finishes and open it in Firefox.

The add-on id is `rum-downloader@rum.app` (set in `scripts/manifest.mjs`); keep it
the same for every upload.

## Pair it with Rum

Rum only accepts extension requests that carry a secret token, so websites can't
use it.

1. In Rum open **Settings, Browser extension** and click **Copy**.
2. In the browser open the extension's **Options** and paste it under **Pairing
   token**, then **Save**. It should say **Connected**.

**Regenerate** in Rum makes the old token stop working; paste the new one.

The port defaults to `8080`. If another program holds it, Rum moves to the next
free port (8081, ...) and the extension finds it by itself; change **Port** only
if you know Rum is somewhere else.

## Permissions

| Permission | Why |
|---|---|
| `downloads` | see browser downloads, cancel the ones Rum took over |
| `cookies` + all sites | read the cookies for the download link so Rum can use your login |
| `contextMenus` | the right-click entry |
| `storage` | remember the token and options (kept on this device only) |

The extension talks only to `127.0.0.1` (Rum on your own machine). Firefox may
ask you to grant the site access after installing.

## How it works

- `src/background.js`: capture and context menu.
- `src/lib/rules.js`: pure decisions (what to capture, payloads, settings).
- `src/lib/rum.js`: the `/ext/*` calls. It first checks the port answers like Rum
  (without the token) and only then sends the token, so a stray service on
  another port never sees it.
- Rum side: `GET /ext/ping`, `POST /ext/add`, `POST /ext/open`, each needing
  `X-Rum-Token`, rate-limited, and open to `chrome-extension://` /
  `moz-extension://` origins only on `/ext/*`.

## Limits

- Auto-capture needs the file size up front; downloads that don't announce one
  stay with the browser.
- Downloads from private windows are never captured.
- Rum can only be raised ("Open Rum") while it is running; the extension can't
  start it.
