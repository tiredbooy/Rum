// Firefox exposes promise-based `browser`; Chrome MV3's `chrome` is promise-based too.
export const ext = globalThis.browser ?? globalThis.chrome;
