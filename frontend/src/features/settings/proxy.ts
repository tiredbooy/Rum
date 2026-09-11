/**
 * Proxy validation mirroring config.NormalizeProxy on the Go side: empty is
 * valid (direct connection), a bare `host:port` is treated as an http proxy, and
 * only the schemes net/http can dial natively are accepted. Keeping the same
 * rule client-side means an unusable value is caught before it is saved rather
 * than being silently ignored at download time.
 */
const PROXY_SCHEMES = new Set(["http:", "https:", "socks5:", "socks5h:"]);

export function isValidProxy(raw: string): boolean {
  const value = raw.trim();
  if (value === "") return true;

  const candidate = value.includes("://") ? value : `http://${value}`;
  try {
    const url = new URL(candidate);
    if (!PROXY_SCHEMES.has(url.protocol)) return false;
    return url.hostname !== "";
  } catch {
    return false;
  }
}
