import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { DownloadErrorNote } from "./DownloadErrorNote";

const render = (props: { code?: string; error: string }) =>
  renderToStaticMarkup(createElement(DownloadErrorNote, props));

describe("DownloadErrorNote", () => {
  it("shows the plain message and keeps the raw error in the title", () => {
    const html = render({ code: "network_down", error: "dial tcp 1.2.3.4:443: i/o timeout" });
    expect(html).toContain("Connection lost.");
    expect(html).toContain("Check internet or VPN.");
    expect(html).toContain('title="dial tcp 1.2.3.4:443: i/o timeout"');
    expect(html).toContain("Details");
  });

  it("tells the user a blocked site is unreachable, not that the internet is down", () => {
    const html = render({ code: "site_unreachable", error: "dial tcp: i/o timeout" });
    expect(html).toContain("This site is unreachable.");
    expect(html).toContain("Try a VPN or proxy.");
    expect(html).not.toMatch(/internet/i);
  });

  it("uses the unknown message when the code is missing", () => {
    expect(render({ error: "boom" })).toContain("Download failed.");
  });
});
