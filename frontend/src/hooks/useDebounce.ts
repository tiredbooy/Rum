import { useEffect, useMemo, useRef } from "react";

interface DebounceOptions {
  delay: number;
  leading?: boolean;
  trailing?: boolean;
}

export function useDebouncedCallback<T extends (...args: any[]) => any>(
  callback: T,
  options: DebounceOptions,
): {
  (...args: Parameters<T>): void;
  cancel: () => void;
  flush: () => void;
} {
  const { delay, leading = false, trailing = true } = options;

  // Always use the latest callback
  const callbackRef = useRef(callback);
  useEffect(() => {
    callbackRef.current = callback;
  }, [callback]);

  // Mutable refs for timeout, args, and leading flag
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const argsRef = useRef<Parameters<T> | null>(null);
  const leadingExecuted = useRef(false);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (timeoutRef.current) clearTimeout(timeoutRef.current);
    };
  }, []);

  // Build the debounced function with methods in one shot – no mutation later
  const debounced = useMemo(() => {
    const fn = (...args: Parameters<T>) => {
      argsRef.current = args;

      // Leading edge
      if (leading && !leadingExecuted.current) {
        leadingExecuted.current = true;
        callbackRef.current(...args);
      }

      // Clear any pending timeout
      if (timeoutRef.current) clearTimeout(timeoutRef.current);

      // Set trailing timeout
      timeoutRef.current = setTimeout(() => {
        if (trailing || !leading) {
          callbackRef.current(...args);
        }
        timeoutRef.current = null;
        leadingExecuted.current = false;
        argsRef.current = null;
      }, delay);
    };

    fn.cancel = () => {
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
        timeoutRef.current = null;
      }
      leadingExecuted.current = false;
      argsRef.current = null;
    };

    fn.flush = () => {
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
        timeoutRef.current = null;
        if (argsRef.current) {
          callbackRef.current(...argsRef.current);
          argsRef.current = null;
        }
      }
      leadingExecuted.current = false;
    };

    return fn;
    // Dependencies: only recreate when debouncing behaviour changes
  }, [delay, leading, trailing]);

  return debounced;
}
