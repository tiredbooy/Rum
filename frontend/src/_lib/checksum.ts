import type { ChecksumAlgo } from "@/_lib/types/download-types";

/** Pick the hash algorithm from a pasted checksum's length (hex chars). */
export function algoForChecksum(value: string): ChecksumAlgo | undefined {
  const hex = value.trim();
  if (!/^[0-9a-fA-F]+$/.test(hex)) return undefined;
  if (hex.length === 32) return "md5";
  if (hex.length === 64) return "sha256";
  return undefined;
}
