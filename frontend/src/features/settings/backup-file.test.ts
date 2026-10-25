import { describe, expect, it } from "vitest";
import { exportFileName, importSummary, parseDownloadList } from "./backup-file";

describe("parseDownloadList", () => {
  it("accepts a v1 list", () => {
    const doc = parseDownloadList('{"version":1,"downloads":[{"url":"http://x/a"}]}');
    expect(doc?.downloads).toHaveLength(1);
  });
  it.each(["", "nope", "null", '{"version":2,"downloads":[]}', '{"version":1}'])(
    "rejects %j",
    (text) => expect(parseDownloadList(text)).toBeNull(),
  );
});

describe("importSummary", () => {
  it("says what happened in one line", () => {
    expect(importSummary({ added: 3, skipped: 1, invalid: 2 })).toBe(
      "Added 3 (paused), 1 skipped, 2 invalid",
    );
    expect(importSummary({ added: 0, skipped: 0, invalid: 0 })).toBe("Nothing to import");
  });
});

describe("exportFileName", () => {
  it("is dated", () => {
    expect(exportFileName(new Date("2026-01-31T10:00:00Z"))).toBe("rum-downloads-2026-01-31.json");
  });
});
