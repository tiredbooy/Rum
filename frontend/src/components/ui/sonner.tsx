"use client"

import * as React from "react"
import {
  CircleCheckIcon,
  InfoIcon,
  Loader2Icon,
  OctagonXIcon,
  TriangleAlertIcon,
} from "lucide-react"
import { Toaster as Sonner, type ToasterProps } from "sonner"

type Theme = "light" | "dark"

/**
 * Resolve the active theme without depending on `next-themes` (not installed).
 * Convention in this app is the shadcn `.dark` class on <html>; we fall back to
 * the OS `prefers-color-scheme` when no explicit class is present.
 */
function getTheme(): Theme {
  if (typeof document === "undefined") return "light"
  const root = document.documentElement
  if (root.classList.contains("dark")) return "dark"
  if (root.classList.contains("light")) return "light"
  if (
    typeof window !== "undefined" &&
    window.matchMedia?.("(prefers-color-scheme: dark)").matches
  ) {
    return "dark"
  }
  return "light"
}

function useResolvedTheme(): Theme {
  const [theme, setTheme] = React.useState<Theme>(() => getTheme())

  React.useEffect(() => {
    const update = () => setTheme(getTheme())

    // React to OS-level theme changes.
    const media = window.matchMedia?.("(prefers-color-scheme: dark)")
    media?.addEventListener?.("change", update)

    // React to the `.dark`/`.light` class being toggled on <html>.
    const observer = new MutationObserver(update)
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["class"],
    })

    return () => {
      media?.removeEventListener?.("change", update)
      observer.disconnect()
    }
  }, [])

  return theme
}

function getReducedMotion(): boolean {
  if (typeof window === "undefined") return false
  return window.matchMedia?.("(prefers-reduced-motion: reduce)").matches ?? false
}

const Toaster = ({ ...props }: ToasterProps) => {
  const theme = useResolvedTheme()
  const [reducedMotion, setReducedMotion] = React.useState<boolean>(() =>
    getReducedMotion()
  )

  // The document direction (set via radix DirectionProvider / <html dir>) drives
  // which side toasts anchor to so they stay RTL-aware.
  const dir =
    typeof document !== "undefined"
      ? (document.documentElement.getAttribute("dir") as
          | "rtl"
          | "ltr"
          | null)
      : null
  const position: ToasterProps["position"] =
    dir === "rtl" ? "top-left" : "top-right"

  React.useEffect(() => {
    const media = window.matchMedia?.("(prefers-reduced-motion: reduce)")
    const update = () => setReducedMotion(getReducedMotion())
    media?.addEventListener?.("change", update)
    return () => media?.removeEventListener?.("change", update)
  }, [])

  return (
    <Sonner
      theme={theme}
      dir={dir ?? "auto"}
      position={position}
      richColors
      closeButton
      // Honour reduced-motion: skip the slide/swipe animation entirely.
      duration={reducedMotion ? 6000 : 4000}
      className="toaster group"
      icons={{
        success: <CircleCheckIcon className="size-4" />,
        info: <InfoIcon className="size-4" />,
        warning: <TriangleAlertIcon className="size-4" />,
        error: <OctagonXIcon className="size-4" />,
        loading: <Loader2Icon className="size-4 animate-spin motion-reduce:animate-none" />,
      }}
      style={
        {
          "--normal-bg": "var(--popover)",
          "--normal-text": "var(--popover-foreground)",
          "--normal-border": "var(--border)",
          "--border-radius": "var(--radius)",
        } as React.CSSProperties
      }
      {...props}
    />
  )
}

export { Toaster }
