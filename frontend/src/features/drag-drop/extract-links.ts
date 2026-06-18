// Pure link extraction from browser drag-and-drop payloads, plus a smart
// downloadable/page classifier. Kept dependency-free and DOM-light so it is
// unit-testable in a node environment (HTML parsing uses DOMParser only when
// a real DataTransfer is read in the browser; the string core accepts html
// and parses it with a tiny regex fallback when DOMParser is absent).

const HTTP_URL_RE = /\bhttps?:\/\/[^\s"'<>)\]]+/gi;
const MAX_ANCHORS = 5000;

// File extensions we treat as directly downloadable.
const DOWNLOADABLE_EXT = new Set([
  "zip", "rar", "7z", "tar", "gz", "bz2", "xz", "tgz",
  "mp4", "mkv", "avi", "mov", "webm", "flv", "m4v",
  "mp3", "flac", "wav", "m4a", "aac", "ogg",
  "iso", "img", "bin",
  "pdf", "epub", "apk", "exe", "msi", "dmg", "deb", "rpm", "appimage",
]);
// part1/part2... style archive segments.
const PART_RE = /\.part\d+$/i;
// Page-like extensions that are never "downloadable files".
const PAGE_EXT = new Set(["html", "htm", "php", "asp", "aspx", "jsp", "do"]);

function dedupe(urls: string[]): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const u of urls) {
    const t = u.trim();
    if (!t || seen.has(t)) continue;
    seen.add(t);
    out.push(t);
  }
  return out;
}

function fromUriList(uriList: string): string[] {
  return uriList
    .split(/\r?\n/)
    .map((l) => l.trim())
    .filter((l) => l && !l.startsWith("#"));
}

function fromPlain(plain: string): string[] {
  return plain.match(HTTP_URL_RE) ?? [];
}

function fromHtml(html: string): string[] {
  // Prefer a real DOM parser when available (browser); fall back to regex in node.
  if (typeof DOMParser !== "undefined") {
    const doc = new DOMParser().parseFromString(html, "text/html");
    const out: string[] = [];
    const push = (v: string | null) => {
      if (v && /^https?:\/\//i.test(v)) out.push(v);
    };
    const anchors = Array.from(doc.querySelectorAll("a[href]")).slice(0, MAX_ANCHORS);
    anchors.forEach((a) => push(a.getAttribute("href")));
    doc.querySelectorAll("img[src],video[src],source[src],[data-href]").forEach((el) => {
      push(el.getAttribute("src"));
      push(el.getAttribute("data-href"));
    });
    return out;
  }
  return html.match(HTTP_URL_RE) ?? [];
}

export function extractUrlsFromStrings(input: {
  uriList?: string;
  html?: string;
  plain?: string;
}): string[] {
  const out: string[] = [];
  if (input.uriList) out.push(...fromUriList(input.uriList));
  if (input.html) out.push(...fromHtml(input.html));
  if (input.plain) out.push(...fromPlain(input.plain));
  return dedupe(out);
}

export function extractUrlsFromDataTransfer(dt: DataTransfer): string[] {
  return extractUrlsFromStrings({
    uriList: dt.getData("text/uri-list") || undefined,
    html: dt.getData("text/html") || undefined,
    plain: dt.getData("text/plain") || undefined,
  });
}

function extOf(url: string): string {
  try {
    const path = new URL(url).pathname;
    const base = path.split("/").pop() ?? "";
    const dot = base.lastIndexOf(".");
    return dot >= 0 ? base.slice(dot + 1).toLowerCase() : "";
  } catch {
    return "";
  }
}

function isDownloadable(url: string): boolean {
  if (PART_RE.test(new URL(url).pathname)) return true;
  const ext = extOf(url);
  if (!ext || PAGE_EXT.has(ext)) return false;
  return DOWNLOADABLE_EXT.has(ext);
}

export function classifyLinks(urls: string[]): {
  all: string[];
  downloadable: string[];
} {
  const all = dedupe(urls.filter((u) => /^https?:\/\//i.test(u)));
  const downloadable = all.filter((u) => {
    try {
      return isDownloadable(u);
    } catch {
      return false;
    }
  });
  return { all, downloadable };
}
