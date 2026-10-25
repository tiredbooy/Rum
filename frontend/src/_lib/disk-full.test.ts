import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import {
  bytesToFree,
  countDiskFull,
  diskFullBannerText,
  diskFullHeadline,
  isDiskFull,
} from "./disk-full";
import { DiskFullBanner } from "@/components/disk-full-banner";
import { DiskFullNotice } from "@/features/dashboard/download-list/DiskFullNotice";
import type { Download } from "./types/download-types";

const GB = 1024 ** 3;

const full: Download = {
  id: "a",
  url: "http://x/a",
  status: "error",
  downloaded: 0,
  remaining: 0,
  error: "not enough disk space",
  error_code: "disk_full",
  needed_bytes: 5 * GB,
  free_bytes: 2 * GB,
  disk_path: "/data/dl",
};

describe("disk-full state", () => {
  it("is recognised only for an errored job with the disk_full code", () => {
    expect(isDiskFull(full)).toBe(true);
    expect(isDiskFull({ ...full, status: "running" })).toBe(false);
    expect(isDiskFull({ ...full, error_code: "dns" })).toBe(false);
  });

  it("says how much to free, including the safety margin", () => {
    // 5 GB needed + 1% margin - 2 GB free.
    const expected = 5 * GB + Math.floor((5 * GB) / 100) - 2 * GB;
    expect(bytesToFree(full)).toBe(expected);
    expect(diskFullHeadline(full)).toBe("Disk full — free 3.0 GB to continue");
  });

  it("falls back to a generic line when sizes are unknown", () => {
    expect(diskFullHeadline({ ...full, needed_bytes: undefined })).toBe(
      "Disk full — free some space to continue",
    );
  });

  it("counts waiting downloads and words the banner", () => {
    const list = [full, { ...full, id: "b" }, { ...full, id: "c", status: "paused" as const }];
    expect(countDiskFull(list)).toBe(2);
    expect(diskFullBannerText(1)).toBe("1 download is waiting for disk space");
    expect(diskFullBannerText(2)).toBe("2 downloads are waiting for disk space");
  });
});

describe("disk-full UI", () => {
  it("card notice shows headline, folder and an accessible Retry", () => {
    const html = renderToStaticMarkup(createElement(DiskFullNotice, { download: full }));
    expect(html).toContain("Disk full — free 3.0 GB to continue");
    expect(html).toContain("/data/dl");
    expect(html).toContain('role="alert"');
    expect(html).toContain('aria-label="Retry"');
  });

  it("banner renders the count and nothing when none wait", () => {
    const html = renderToStaticMarkup(createElement(DiskFullBanner, { count: 3 }));
    expect(html).toContain("3 downloads are waiting for disk space");
    expect(renderToStaticMarkup(createElement(DiskFullBanner, { count: 0 }))).toBe("");
  });
});
