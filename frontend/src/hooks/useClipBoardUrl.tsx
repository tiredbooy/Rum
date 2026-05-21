import { useState, useCallback, useEffect } from "react";
import { isValidUrl } from "@/_lib/utils/validators";

export function useClipboardUrl() {
  const [clipboardUrl, setClipboardUrl] = useState<string>("");

  // Raw text paste (no validation)
  const readRawText = useCallback(async (): Promise<string> => {
    try {
      return await navigator.clipboard.readText();
    } catch {
      return "";
    }
  }, []);

  // Validated paste (returns only if it’s a valid URL)
  const pasteValidUrl = useCallback(async (): Promise<string> => {
    const text = await readRawText();
    if (isValidUrl(text.trim())) {
      setClipboardUrl(text.trim());
      return text.trim();
    }
    return "";
  }, [readRawText]);

  // Auto‑detect on mount
  useEffect(() => {
    readRawText().then((text) => {
      if (isValidUrl(text.trim())) {
        setClipboardUrl(text.trim());
      }
    });
  }, [readRawText]);

  return { clipboardUrl, pasteValidUrl, readRawText };
}
