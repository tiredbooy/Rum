import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";
import { OpenActions } from "./OpenActions";
import { DiskFullNotice } from "./DiskFullNotice";
import type { Download } from "@/_lib/types/download-types";

function bind(app?: Record<string, unknown>) {
  (globalThis as { window?: unknown }).window = app ? { go: { main: { App: app } } } : {};
}

afterEach(() => {
  delete (globalThis as { window?: unknown }).window;
});

const done = { id: "a", status: "completed" } as const;
const full: Download = {
  id: "b",
  url: "http://x/b",
  status: "error",
  downloaded: 0,
  remaining: 0,
  error_code: "disk_full",
  disk_path: "/data/dl",
};
const both = { OpenDownloadFolder: vi.fn(), OpenDownload: vi.fn() };

describe("OpenActions", () => {
  it("renders nothing in browser mode", () => {
    bind();
    expect(renderToStaticMarkup(createElement(OpenActions, { download: done }))).toBe("");
  });

  it("renders Open and Open folder on a completed download", () => {
    bind(both);
    const html = renderToStaticMarkup(createElement(OpenActions, { download: done }));
    expect(html).toContain('aria-label="Open"');
    expect(html).toContain('aria-label="Open folder"');
  });

  it("omits Open when only the folder binding exists", () => {
    bind({ OpenDownloadFolder: vi.fn() });
    const html = renderToStaticMarkup(createElement(OpenActions, { download: done }));
    expect(html).not.toContain('aria-label="Open"');
    expect(html).toContain('aria-label="Open folder"');
  });

  it("disk-full notice gets Open folder only with the binding", () => {
    bind();
    expect(renderToStaticMarkup(createElement(DiskFullNotice, { download: full }))).not.toContain(
      "Open folder",
    );
    bind(both);
    const html = renderToStaticMarkup(createElement(DiskFullNotice, { download: full }));
    expect(html).toContain('aria-label="Open folder"');
    expect(html).not.toContain('aria-label="Open"');
  });
});
