import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import {
  showUpdateBanner,
  updateBannerText,
  updateSummary,
  type UpdateStatus,
} from "./update-notice";
import { UpdateBanner } from "@/components/update-banner";
import { describeDownloadError } from "./utils/download-errors";

const base: UpdateStatus = { current: "1.0.0", available: false, dismissed: false };
const newer: UpdateStatus = {
  ...base,
  latest: "1.2.0",
  tag: "v1.2.0",
  available: true,
  release_url: "https://github.com/tiredbooy/Rum/releases/tag/v1.2.0",
};

describe("update banner visibility", () => {
  it("is hidden with no data, when up to date, and after Later", () => {
    expect(showUpdateBanner(undefined)).toBe(false);
    expect(showUpdateBanner(base)).toBe(false);
    expect(showUpdateBanner({ ...newer, dismissed: true })).toBe(false);
  });
  it("shows for an available, undismissed release", () => {
    expect(showUpdateBanner(newer)).toBe(true);
  });
  it("stays hidden when only a failed check is known", () => {
    expect(showUpdateBanner({ ...base, error: "update_check_failed" })).toBe(false);
  });
});

describe("update copy", () => {
  it("names the version", () => {
    expect(updateBannerText(newer)).toBe("Rum v1.2.0 available");
  });
  it("summarises each settings state", () => {
    expect(updateSummary(undefined, false)).toBe("");
    expect(updateSummary(undefined, true)).toBe("Checking…");
    expect(updateSummary({ ...base, latest: "1.0.0" }, false)).toBe("Up to date (v1.0.0)");
    expect(updateSummary(base, false)).toBe("Not checked yet");
    expect(updateSummary(newer, false)).toBe("Rum v1.2.0 available");
    expect(updateSummary({ ...base, error: "update_check_failed" }, false)).toMatch(/reach/i);
  });
  it("keeps update error text within 12 words", () => {
    for (const code of ["update_check_failed", "update_verify_failed"]) {
      const { message, action } = describeDownloadError(code);
      expect(message).not.toBe(describeDownloadError("unknown").message);
      expect(`${message} ${action}`.split(/\s+/).length).toBeLessThanOrEqual(12);
    }
  });
});

describe("UpdateBanner", () => {
  const props = { text: "Rum v1.2.0 available", busy: false, onDownload: () => {}, onLater: () => {} };
  it("renders the notice with Download and Later", () => {
    const html = renderToStaticMarkup(createElement(UpdateBanner, props));
    expect(html).toContain("Rum v1.2.0 available");
    expect(html).toContain("Download");
    expect(html).toContain("Later");
    expect(html).toContain('role="status"');
  });
  it("disables both actions while downloading", () => {
    const html = renderToStaticMarkup(createElement(UpdateBanner, { ...props, busy: true }));
    expect(html.match(/disabled=""/g)?.length).toBe(2);
    expect(html).toContain("Downloading");
  });
});

describe("update summary after Later", () => {
  it("still reports the available release when dismissed", () => {
    expect(updateSummary({ ...newer, dismissed: true }, false)).toBe("Rum v1.2.0 available");
  });
});
