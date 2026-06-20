import { useEffect, useRef, useState } from "react";

/** Tunables for {@link useSmoothProgress}. */
export interface SmoothProgressOptions {
  /**
   * Easing factor applied each frame: `displayed += (target - displayed) * k`.
   * Higher = snappier, lower = smoother/laggier. Range (0, 1]. Default 0.12.
   */
  lerpFactor?: number;
  /**
   * Minimum percent the bar advances per frame while it still trails the
   * target. Guarantees visible, continuous motion between sparse SSE ticks
   * even when `(target - displayed) * lerpFactor` would round to ~0.
   */
  minStepPerFrame?: number;
  /**
   * Distance (in %) under which the displayed value snaps directly to the
   * target, so the bar settles cleanly instead of asymptotically crawling.
   */
  snapThreshold?: number;
  /**
   * Override reduced-motion detection. When `true`, the hook returns the raw
   * target and runs no animation loop (the caller's CSS transition handles it).
   * Defaults to reading `prefers-reduced-motion: reduce`.
   */
  reducedMotion?: boolean;
}

const DEFAULTS = {
  lerpFactor: 0.12,
  minStepPerFrame: 0.35,
  snapThreshold: 0.15,
} as const;

/** Target below which we treat the download as "restarted" and snap to it. */
const RESTART_TARGET = 1;

/**
 * Pure easing step used by the animation loop. Eases `displayed` toward
 * `target` and is **monotonic non-decreasing** for a non-decreasing target:
 * it never moves the bar backward except when the target itself dropped (a
 * restart), in which case it snaps straight to the (lower) target.
 *
 * Exported for unit testing the math without a DOM/rAF.
 */
export function stepProgress(
  displayed: number,
  target: number,
  opts: Required<Omit<SmoothProgressOptions, "reducedMotion">>,
): number {
  // Completion or restart: jump immediately. A restart is the target dropping
  // well below the displayed value (download reset to ~0 and climbing again).
  if (target >= 100) return 100;
  if (target <= RESTART_TARGET && target < displayed) return target;

  const delta = target - displayed;
  if (delta <= 0) return displayed; // already caught up; never regress.
  if (delta <= opts.snapThreshold) return target;

  const eased = delta * opts.lerpFactor;
  const step = Math.max(eased, opts.minStepPerFrame);
  // Don't overshoot the target.
  return Math.min(target, displayed + step);
}

function prefersReducedMotion(): boolean {
  if (typeof window === "undefined" || !window.matchMedia) return false;
  return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

/**
 * Client-side interpolation for a progress bar driven by sparse SSE updates.
 *
 * Holds a displayed percent in state and, on every animation frame while
 * `isActive`, eases it toward the latest `targetPct` at a bounded rate so the
 * bar glides continuously instead of lurching on each tick. Monotonic while
 * running (never moves backward), snaps to 100 on completion, and resets when
 * the target drops to ~0 (a restarted download).
 *
 * Respects reduced motion: when reduced (via the `reducedMotion` option or the
 * `prefers-reduced-motion` media query) it returns the raw target and runs no
 * loop, letting the consumer's CSS transition do the smoothing.
 *
 * The rAF loop only runs while `isActive` and while there is distance left to
 * cover, so idle/finished cards cost no CPU.
 */
export function useSmoothProgress(
  targetPct: number,
  isActive: boolean,
  opts?: SmoothProgressOptions,
): number {
  const reduced = opts?.reducedMotion ?? prefersReducedMotion();

  const config: Required<Omit<SmoothProgressOptions, "reducedMotion">> = {
    lerpFactor: opts?.lerpFactor ?? DEFAULTS.lerpFactor,
    minStepPerFrame: opts?.minStepPerFrame ?? DEFAULTS.minStepPerFrame,
    snapThreshold: opts?.snapThreshold ?? DEFAULTS.snapThreshold,
  };

  const clampedTarget = Math.min(100, Math.max(0, targetPct));

  const [displayed, setDisplayed] = useState(clampedTarget);

  // Refs the rAF loop reads without re-subscribing.
  const displayedRef = useRef(displayed);
  const targetRef = useRef(clampedTarget);
  const rafRef = useRef<number | null>(null);
  const configRef = useRef(config);
  configRef.current = config;
  targetRef.current = clampedTarget;

  // Reduced motion (or inactive): mirror the target directly, no animation.
  useEffect(() => {
    if (!reduced && isActive) return;
    if (rafRef.current != null) {
      cancelAnimationFrame(rafRef.current);
      rafRef.current = null;
    }
    displayedRef.current = clampedTarget;
    setDisplayed(clampedTarget);
  }, [reduced, isActive, clampedTarget]);

  // Animated path: ease the displayed value toward the target each frame.
  useEffect(() => {
    if (reduced || !isActive) return;

    // Handle restart/completion synchronously so we don't animate through them.
    if (clampedTarget >= 100) {
      displayedRef.current = 100;
      setDisplayed(100);
      return;
    }
    if (clampedTarget <= RESTART_TARGET && clampedTarget < displayedRef.current) {
      displayedRef.current = clampedTarget;
      setDisplayed(clampedTarget);
    }

    const tick = () => {
      const next = stepProgress(
        displayedRef.current,
        targetRef.current,
        configRef.current,
      );
      if (next !== displayedRef.current) {
        displayedRef.current = next;
        setDisplayed(next);
      }
      // Keep looping until we've caught the target; then sleep until it moves.
      if (displayedRef.current < targetRef.current) {
        rafRef.current = requestAnimationFrame(tick);
      } else {
        rafRef.current = null;
      }
    };

    if (rafRef.current == null) {
      rafRef.current = requestAnimationFrame(tick);
    }

    return () => {
      if (rafRef.current != null) {
        cancelAnimationFrame(rafRef.current);
        rafRef.current = null;
      }
    };
    // `clampedTarget` is the trigger: a new SSE tick restarts the loop if it
    // had gone idle. The loop reads live values via refs.
  }, [clampedTarget, isActive, reduced]);

  // Cleanup on unmount (StrictMode double-invoke safe: the effect cleanups
  // above cancel any in-flight frame before re-running).
  useEffect(() => {
    return () => {
      if (rafRef.current != null) {
        cancelAnimationFrame(rafRef.current);
        rafRef.current = null;
      }
    };
  }, []);

  return reduced || !isActive ? clampedTarget : displayed;
}
