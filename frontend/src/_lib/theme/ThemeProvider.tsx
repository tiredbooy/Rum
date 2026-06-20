/**
 * ThemeProvider — reads the persisted settings (preferred_theme, accent_color,
 * ui_density, reduced_motion) and applies them live to
 * `document.documentElement`. Mounted once near the app root, *inside* the
 * QueryClientProvider so it can read the settings cache. Updating any of these
 * settings re-applies instantly (the query cache change re-runs the effect).
 *
 * It also exposes the effective reduced-motion state via `useAppReducedMotion()`
 * so the dashboard's progress hook can prefer the setting over the bare
 * `prefers-reduced-motion` media query.
 */
import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { useSettings } from "@/_lib/services/queries/settings.queries";
import type { UIDensity } from "@/_lib/types/setting-types";
import { applyTheme, type PreferredTheme } from "./apply-theme";

interface ThemeContextValue {
  /** Effective reduced motion: the setting OR the OS `prefers-reduced-motion`. */
  reducedMotion: boolean;
}

const ThemeContext = createContext<ThemeContextValue>({ reducedMotion: false });

const REDUCE_MOTION_QUERY = "(prefers-reduced-motion: reduce)";
const DARK_QUERY = "(prefers-color-scheme: dark)";

function prefersReducedMotion(): boolean {
  if (typeof window === "undefined") return false;
  return window.matchMedia?.(REDUCE_MOTION_QUERY).matches ?? false;
}

function prefersDarkScheme(): boolean {
  if (typeof window === "undefined") return false;
  return window.matchMedia?.(DARK_QUERY).matches ?? false;
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const { data: settings } = useSettings();

  // Track OS-level media queries so "system" theme + auto reduced-motion follow
  // the OS even when no setting drives them.
  const [osDark, setOsDark] = useState<boolean>(() => prefersDarkScheme());
  const [osReduce, setOsReduce] = useState<boolean>(() =>
    prefersReducedMotion(),
  );

  useEffect(() => {
    if (typeof window === "undefined" || !window.matchMedia) return;
    const darkMql = window.matchMedia(DARK_QUERY);
    const reduceMql = window.matchMedia(REDUCE_MOTION_QUERY);
    const onDark = () => setOsDark(darkMql.matches);
    const onReduce = () => setOsReduce(reduceMql.matches);
    darkMql.addEventListener("change", onDark);
    reduceMql.addEventListener("change", onReduce);
    return () => {
      darkMql.removeEventListener("change", onDark);
      reduceMql.removeEventListener("change", onReduce);
    };
  }, []);

  const preferredTheme: PreferredTheme = settings?.preferred_theme ?? "system";
  const accentColor = settings?.accent_color ?? "";
  const uiDensity: UIDensity = settings?.ui_density ?? "comfortable";
  const reducedMotionSetting = settings?.reduced_motion ?? false;

  // The single source of truth for "should we reduce motion".
  const reducedMotion = reducedMotionSetting || osReduce;

  useEffect(() => {
    if (typeof document === "undefined") return;
    applyTheme(
      document.documentElement,
      {
        preferredTheme,
        accentColor,
        uiDensity,
        reducedMotion: reducedMotionSetting,
      },
      osDark,
    );
  }, [preferredTheme, accentColor, uiDensity, reducedMotionSetting, osDark, osReduce]);

  const value = useMemo<ThemeContextValue>(
    () => ({ reducedMotion }),
    [reducedMotion],
  );

  return (
    <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>
  );
}

/**
 * Effective reduced-motion preference: the `reduced_motion` setting OR the OS
 * `prefers-reduced-motion` media query. Components (e.g. the dashboard progress
 * hook) should prefer this over reading the media query directly so the in-app
 * setting can force-enable it.
 *
 * Safe to call outside the provider (returns the OS preference) so it degrades
 * gracefully if a consumer mounts before the tree is wired.
 */
export function useAppReducedMotion(): boolean {
  return useContext(ThemeContext).reducedMotion;
}
