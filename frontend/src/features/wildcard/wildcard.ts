// Pure IDM-style wildcard expansion: replace the first `*` in a URL with each
// integer in [from, to], optionally zero-padded. Capped to avoid runaway lists.

export const WILDCARD_MAX = 1000;

export interface WildcardRange {
  from: number;
  to: number;
  pad: number; // 0 = no padding
}

export function hasWildcard(url: string): boolean {
  return url.includes("*");
}

export function validateRange(r: WildcardRange): string | null {
  if (!Number.isFinite(r.from) || !Number.isFinite(r.to)) return "From and To must be numbers";
  if (r.from < 0 || r.to < 0) return "From and To must be ≥ 0";
  if (r.to < r.from) return "To must be ≥ From";
  if (r.pad < 0 || r.pad > 12) return "Zero-pad digits must be 0–12";
  if (r.to - r.from + 1 > WILDCARD_MAX) return `Range too large (max ${WILDCARD_MAX})`;
  return null;
}

export function expandWildcard(url: string, r: WildcardRange): string[] {
  const err = validateRange(r);
  if (err) throw new Error(err);
  if (!hasWildcard(url)) return [url];
  const out: string[] = [];
  for (let n = r.from; n <= r.to; n++) {
    const num = r.pad > 0 ? String(n).padStart(r.pad, "0") : String(n);
    out.push(url.replace("*", num));
  }
  return out;
}
