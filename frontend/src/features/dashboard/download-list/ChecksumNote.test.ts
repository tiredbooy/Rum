import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it } from "vitest";
import { ChecksumNote } from "./ChecksumNote";
import type { Download } from "@/_lib/types/download-types";

const render = (extra: Partial<Download>) =>
  renderToStaticMarkup(
    createElement(
      QueryClientProvider,
      { client: new QueryClient() },
      createElement(ChecksumNote, {
        download: { id: "1", url: "u", status: "completed", downloaded: 1, remaining: 0, ...extra },
      }),
    ),
  );

describe("ChecksumNote", () => {
  it("shows Verified with the algorithm", () => {
    expect(render({ checksum_status: "verified", checksum_result_algo: "sha256" })).toContain("Verified (SHA-256)");
  });
  it("offers Redownload on mismatch", () => {
    const html = render({ checksum_status: "mismatch" });
    expect(html).toContain("File is damaged. Redownload?");
    expect(html).toContain("Redownload</button>");
  });
  it("renders nothing without a result", () => {
    expect(render({})).toBe("");
  });
});
