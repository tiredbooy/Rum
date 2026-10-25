import { request } from "./api";

export interface ExtensionToken {
  token: string;
}

/** The browser-extension pairing token (created on first use). */
export async function getExtensionToken(): Promise<ExtensionToken> {
  return request<ExtensionToken>(`/api/v1/extension/token`);
}

/** Replace the token; extensions paired with the old one stop working. */
export async function regenerateExtensionToken(): Promise<ExtensionToken> {
  return request<ExtensionToken>(`/api/v1/extension/token/regenerate`, {
    method: "POST",
    body: "{}",
  });
}
