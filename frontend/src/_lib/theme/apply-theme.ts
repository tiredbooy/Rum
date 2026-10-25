/**
 * Imperative theme application: maps the relevant settings onto
 * `document.documentElement` (color-scheme class, accent CSS vars, density
 * class, reduced-motion class). Kept as a pure-ish function that takes the root
 * element so it can run from the provider effect and be unit-tested with a
 * stub element.
 */
import type { Setting, UIDensity } from "@/_lib/types/setting-types";
import { deriveAccentVars } from "./color";

export type PreferredTheme = "system" | "light" | "dark";

export interface ThemeInputs {
  preferredTheme: PreferredTheme;
  accentColor: string;
  uiDensity: UIDensity;
  reducedMotion: boolean;
}

/**
 * Map loaded settings onto theme inputs, or null while they have not loaded.
 * Callers must not apply anything for null: defaults applied before the real
 * values arrive (e.g. "system" on a light OS) flip a dark-theme user to light
 * until the first response lands.
 */
export function themeInputsFrom(
  settings: Partial<Setting> | undefined,
): ThemeInputs | null {
  if (!settings) return null;
  return {
    preferredTheme: settings.preferred_theme ?? "system",
    accentColor: settings.accent_color ?? "",
    uiDensity: settings.ui_density ?? "comfortable",
    reducedMotion: settings.reduced_motion ?? false,
  };
}

/** CSS custom properties the accent overrides write to (and clear). */
const ACCENT_VARS = [
  "--primary",
  "--primary-foreground",
  "--accent",
  "--accent-foreground",
  "--ring",
  "--focus-ring",
  "--sidebar-primary",
  "--sidebar-primary-foreground",
  "--sidebar-ring",
] as const;

/** Resolve "system" against the OS color scheme; otherwise the explicit choice. */
export function resolveColorScheme(
  preferred: PreferredTheme,
  prefersDark: boolean,
): "light" | "dark" {
  if (preferred === "system") return prefersDark ? "dark" : "light";
  return preferred;
}

/**
 * Apply every theme input to a root element. Idempotent — safe to call on every
 * settings change. Returns nothing; all effects are on `root`.
 */
export function applyTheme(
  root: HTMLElement,
  inputs: ThemeInputs,
  prefersDark: boolean,
): void {
  // --- color scheme (.dark / .light) ---
  const scheme = resolveColorScheme(inputs.preferredTheme, prefersDark);
  root.classList.toggle("dark", scheme === "dark");
  root.classList.toggle("light", scheme === "light");
  // Native form controls / scrollbars follow.
  root.style.colorScheme = scheme;

  // --- accent color ---
  const accent = deriveAccentVars(inputs.accentColor);
  if (accent) {
    root.style.setProperty("--primary", accent.accent);
    root.style.setProperty("--primary-foreground", accent.accentForeground);
    root.style.setProperty("--accent", accent.accent);
    root.style.setProperty("--accent-foreground", accent.accentForeground);
    root.style.setProperty("--ring", accent.accent);
    root.style.setProperty("--focus-ring", accent.accent);
    root.style.setProperty("--sidebar-primary", accent.accent);
    root.style.setProperty(
      "--sidebar-primary-foreground",
      accent.accentForeground,
    );
    root.style.setProperty("--sidebar-ring", accent.accent);
  } else {
    // Clear overrides so the stylesheet defaults (per light/dark) take over.
    for (const v of ACCENT_VARS) root.style.removeProperty(v);
  }

  // --- density ---
  root.classList.toggle("density-compact", inputs.uiDensity === "compact");

  // --- reduced motion (explicit setting OR OS preference) ---
  const reduce =
    inputs.reducedMotion ||
    (typeof window !== "undefined" &&
      window.matchMedia?.("(prefers-reduced-motion: reduce)").matches === true);
  root.classList.toggle("reduce-motion", reduce);
}
