import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { themeInputsFrom } from "./apply-theme";

describe("themeInputsFrom", () => {
  it("returns null until the settings have loaded", () => {
    // Applying defaults before the real values arrive flipped a dark-theme user
    // to light (the OS scheme) for the length of the first request.
    expect(themeInputsFrom(undefined)).toBeNull();
  });

  it("maps the stored settings", () => {
    expect(
      themeInputsFrom({
        preferred_theme: "dark",
        accent_color: "#112233",
        ui_density: "compact",
        reduced_motion: true,
      }),
    ).toEqual({
      preferredTheme: "dark",
      accentColor: "#112233",
      uiDensity: "compact",
      reducedMotion: true,
    });
  });

  it("falls back per field when the server omits one", () => {
    expect(themeInputsFrom({ preferred_theme: "light" })).toEqual({
      preferredTheme: "light",
      accentColor: "",
      uiDensity: "comfortable",
      reducedMotion: false,
    });
  });
});

describe("index.html", () => {
  it("does not hard-code a theme class on <body>", () => {
    // `.dark` on <body> re-declares every colour token below <html>, so the
    // ThemeProvider's class on <html> could never switch the app to light.
    const html = readFileSync(
      path.resolve(__dirname, "../../../index.html"),
      "utf8",
    );
    expect(html).not.toMatch(/<body[^>]*class="[^"]*\b(dark|light)\b/);
  });
});
