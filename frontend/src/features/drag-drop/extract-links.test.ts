import { describe, expect, it } from "vitest";
import { classifyLinks, extractUrlsFromStrings } from "./extract-links";

describe("extractUrlsFromStrings", () => {
  it("parses text/uri-list, ignoring comment lines", () => {
    const urls = extractUrlsFromStrings({
      uriList: "# comment\nhttps://x.com/a.zip\nhttps://x.com/b.mp4\n",
    });
    expect(urls).toEqual(["https://x.com/a.zip", "https://x.com/b.mp4"]);
  });

  it("extracts hrefs and media src from HTML", () => {
    const urls = extractUrlsFromStrings({
      html: `<a href="https://x.com/a.rar">a</a><img src="https://x.com/i.png"><video src="https://x.com/v.mp4">`,
    });
    expect(urls).toContain("https://x.com/a.rar");
    expect(urls).toContain("https://x.com/i.png");
    expect(urls).toContain("https://x.com/v.mp4");
  });

  it("falls back to http(s) URLs in plain text", () => {
    const urls = extractUrlsFromStrings({
      plain: "see https://x.com/a.iso and http://y.com/b.bin here",
    });
    expect(urls).toEqual(["https://x.com/a.iso", "http://y.com/b.bin"]);
  });

  it("dedupes preserving order and prefers uri-list over html", () => {
    const urls = extractUrlsFromStrings({
      uriList: "https://x.com/a.zip",
      html: `<a href="https://x.com/a.zip">dup</a><a href="https://x.com/c.7z">c</a>`,
    });
    expect(urls).toEqual(["https://x.com/a.zip", "https://x.com/c.7z"]);
  });
});

describe("classifyLinks", () => {
  it("separates downloadable file links from page-like links", () => {
    const { all, downloadable } = classifyLinks([
      "https://x.com/game.part1.rar",
      "https://x.com/movie.mkv",
      "https://x.com/page.html",
      "https://x.com/about",
      "https://x.com/search?q=1",
    ]);
    expect(all).toHaveLength(5);
    expect(downloadable).toEqual([
      "https://x.com/game.part1.rar",
      "https://x.com/movie.mkv",
    ]);
  });
});
