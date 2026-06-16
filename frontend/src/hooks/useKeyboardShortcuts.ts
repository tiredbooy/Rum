import { useEffect } from "react";

export interface KeyboardShortcutHandlers {
  /** `n` — open the new-download dialog. */
  onNewDownload?: () => void;
  /** `/` — focus the filter/search control. */
  onFocusFilter?: () => void;
  /** `p` — pause all downloads. */
  onPauseAll?: () => void;
  /** `s` — start/resume all downloads. */
  onStartAll?: () => void;
}

const EDITABLE_TAGS = new Set(["INPUT", "TEXTAREA", "SELECT"]);

/**
 * Returns true when focus is in a text field / contentEditable so global
 * single-key shortcuts don't fire while the user is typing.
 */
function isTypingTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  if (EDITABLE_TAGS.has(target.tagName)) return true;
  if (target.isContentEditable) return true;
  // Inside an open combobox/menu/listbox, let the widget own the keys.
  if (target.closest("[role='textbox'],[contenteditable='true']")) return true;
  return false;
}

/**
 * Unobtrusive global keyboard shortcuts for the dashboard.
 *
 *   n  → new download dialog
 *   /  → focus the filter
 *   p  → pause all
 *   s  → start all
 *
 * Shortcuts are ignored while typing in inputs/textareas/selects/contentEditable
 * and when a modifier (Ctrl/Cmd/Alt) is held, so they never clash with browser
 * or OS shortcuts. Pass only the handlers you need.
 */
export function useKeyboardShortcuts(handlers: KeyboardShortcutHandlers): void {
  const { onNewDownload, onFocusFilter, onPauseAll, onStartAll } = handlers;

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.ctrlKey || e.metaKey || e.altKey) return;
      if (e.defaultPrevented) return;
      if (isTypingTarget(e.target)) return;

      switch (e.key) {
        case "n":
        case "N":
          if (onNewDownload) {
            e.preventDefault();
            onNewDownload();
          }
          break;
        case "/":
          if (onFocusFilter) {
            e.preventDefault();
            onFocusFilter();
          }
          break;
        case "p":
        case "P":
          if (onPauseAll) {
            e.preventDefault();
            onPauseAll();
          }
          break;
        case "s":
        case "S":
          if (onStartAll) {
            e.preventDefault();
            onStartAll();
          }
          break;
        default:
          break;
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onNewDownload, onFocusFilter, onPauseAll, onStartAll]);
}
