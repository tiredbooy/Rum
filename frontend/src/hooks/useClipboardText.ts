import { useCallback } from "react";

export function useClipboardText() {
  const pasteFromClipboard = useCallback(async (): Promise<string> => {
    try {
      return await navigator.clipboard.readText();
    } catch {
      return "";
    }
  }, []);

  return { pasteFromClipboard };
}
