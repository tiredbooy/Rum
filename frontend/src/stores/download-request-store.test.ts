import { afterEach, describe, expect, it } from "vitest";
import { useDownloadRequestStore } from "./download-request-store";

// The "From URL" tab (LoadFromUrlForm) and the working Bulk tab both push their
// selected URLs + chosen folder into this shared draft via updateDraft. The Add
// dialog's Save button reads draft.urls, so anything a tab writes here is what
// actually gets saved. These tests pin that contract.
describe("download-request-store draft", () => {
  afterEach(() => {
    useDownloadRequestStore.getState().resetDraft();
  });

  it("starts with an empty draft", () => {
    expect(useDownloadRequestStore.getState().draft).toEqual({ urls: [] });
  });

  it("exposes selected URLs and dest_path written by a tab on the draft", () => {
    // Mirrors what LoadFromUrlForm writes once URLs are fetched/selected.
    useDownloadRequestStore.getState().updateDraft({
      urls: ["https://x.com/a.zip", "https://x.com/b.mp4"],
      dest_path: "downloads/from-url",
    });

    const draft = useDownloadRequestStore.getState().draft;
    expect(draft.urls).toEqual([
      "https://x.com/a.zip",
      "https://x.com/b.mp4",
    ]);
    expect(draft.dest_path).toBe("downloads/from-url");
  });

  it("merges partial updates instead of replacing the draft", () => {
    const { updateDraft } = useDownloadRequestStore.getState();
    updateDraft({ urls: ["https://x.com/a.zip"] });
    updateDraft({ dest_path: "downloads/from-url" });

    const draft = useDownloadRequestStore.getState().draft;
    expect(draft.urls).toEqual(["https://x.com/a.zip"]);
    expect(draft.dest_path).toBe("downloads/from-url");
  });

  it("resetDraft clears a previously written selection", () => {
    useDownloadRequestStore
      .getState()
      .updateDraft({ urls: ["https://x.com/a.zip"], dest_path: "out" });
    useDownloadRequestStore.getState().resetDraft();

    expect(useDownloadRequestStore.getState().draft).toEqual({ urls: [] });
  });
});
