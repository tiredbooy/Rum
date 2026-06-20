/**
 * Small, dependency-free color helpers for the theme provider. We accept an
 * `accent_color` hex (`#RGB` or `#RRGGBB`, validated by the backend) and derive
 * the CSS custom properties that Tailwind tokens (`--primary`, `--accent`, the
 * matching foregrounds, and `--ring`) read from. Everything is plain sRGB hex
 * math so it works in every browser and in tests without a DOM.
 */

export interface AccentVars {
  /** Background fill for primary/accent surfaces (the chosen hex, normalized). */
  accent: string;
  /** A readable foreground (black/white) chosen by luminance contrast. */
  accentForeground: string;
}

/** Strict hex matcher: `#RGB` or `#RRGGBB` only (mirrors the backend rule). */
const HEX_RE = /^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/;

/** Whether a string is a valid `#RGB`/`#RRGGBB` accent color. */
export function isValidAccent(value: string): boolean {
  return HEX_RE.test(value.trim());
}

/** Expand `#RGB` to `#RRGGBB`; pass through `#RRGGBB`; returns lowercased. */
export function normalizeHex(value: string): string {
  const v = value.trim().toLowerCase();
  if (/^#[0-9a-f]{3}$/.test(v)) {
    return `#${v[1]}${v[1]}${v[2]}${v[2]}${v[3]}${v[3]}`;
  }
  return v;
}

/** Parse a normalized hex into 0–255 RGB channels. */
function toRgb(hex: string): { r: number; g: number; b: number } {
  const h = normalizeHex(hex);
  return {
    r: parseInt(h.slice(1, 3), 16),
    g: parseInt(h.slice(3, 5), 16),
    b: parseInt(h.slice(5, 7), 16),
  };
}

/**
 * WCAG relative luminance (0 = black, 1 = white). Used to pick a foreground
 * that stays readable on top of the accent fill.
 */
export function relativeLuminance(hex: string): number {
  const { r, g, b } = toRgb(hex);
  const channel = (c: number): number => {
    const s = c / 255;
    return s <= 0.03928 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4);
  };
  return 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b);
}

/**
 * Derive the accent fill + a contrasting foreground for a validated hex.
 * Returns `null` for an empty or invalid value so callers fall back to the
 * app's default `--primary`/`--accent` tokens.
 */
export function deriveAccentVars(value: string | undefined | null): AccentVars | null {
  if (!value) return null;
  const trimmed = value.trim();
  if (!isValidAccent(trimmed)) return null;
  const accent = normalizeHex(trimmed);
  // Threshold 0.4 keeps mid-bright accents (e.g. emerald, sky) on a dark fg and
  // only flips to white for genuinely dark accents — feels more premium than a
  // strict 0.5 split.
  const accentForeground =
    relativeLuminance(accent) > 0.4 ? "#0a0a0a" : "#ffffff";
  return { accent, accentForeground };
}
