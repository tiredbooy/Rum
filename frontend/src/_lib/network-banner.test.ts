import { describe, expect, it } from "vitest";
import { bannerMessage, expireBack, nextBannerKind } from "./network-banner";

const up = { engineUp: true, internetOnline: true };

describe("nextBannerKind", () => {
  it("shows nothing while online, even with old network errors in history", () => {
    expect(nextBannerKind("none", up)).toBe("none");
  });

  it("shows the network banner only while offline", () => {
    expect(nextBannerKind("none", { ...up, internetOnline: false })).toBe("offline");
    expect(nextBannerKind("offline", { ...up, internetOnline: false })).toBe("offline");
  });

  it("goes to 'back' when connectivity returns, then clears on expiry", () => {
    const back = nextBannerKind("offline", up);
    expect(back).toBe("back");
    expect(nextBannerKind(back, up)).toBe("back");
    expect(expireBack(back)).toBe("none");
    expect(expireBack("offline")).toBe("offline");
  });

  it("engine down wins, and a later network outage is a different kind", () => {
    expect(nextBannerKind("none", { engineUp: false, internetOnline: true })).toBe("engine");
    expect(nextBannerKind("engine", up)).toBe("none");
    expect(nextBannerKind("engine", { engineUp: true, internetOnline: false })).toBe("offline");
  });
});

describe("bannerMessage", () => {
  it("names the blocked downloads when offline", () => {
    expect(bannerMessage("offline", 2)).toContain("2 downloads");
    expect(bannerMessage("offline", 1)).toContain("1 download ");
    expect(bannerMessage("offline", 0)).toMatch(/internet/i);
  });
  it("says back online, hinting failed ones only when they exist", () => {
    expect(bannerMessage("back", 0)).toBe("Back online.");
    expect(bannerMessage("back", 3)).toContain("failed");
  });
  it("is empty for none", () => {
    expect(bannerMessage("none", 0)).toBe("");
  });
});
