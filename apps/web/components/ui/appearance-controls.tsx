"use client";

import { Monitor, Moon, Palette, Sun } from "lucide-react";
import { useEffect } from "react";

import {
  applyAppearance,
  COLOR_SCHEMES,
  type ColorScheme,
  type ThemePreference,
  persistScheme,
  persistThemePreference,
  readSchemeFromDom,
  readThemePreferenceFromDom,
} from "@/lib/appearance";
import { cn } from "@/lib/utils";

/**
 * Appearance stays available without placing seven low-frequency choices in
 * the primary header scan path.
 *
 * Selection chrome is CSS-driven from html data attributes so navigation
 * remounts never flash Graphite / wrong mode.
 */

const MODE_OPTIONS: readonly {
  id: ThemePreference;
  label: string;
  description: string;
  icon: typeof Sun;
}[] = [
  { id: "light", label: "Light", description: "Always use light mode", icon: Sun },
  { id: "dark", label: "Dark", description: "Always use dark mode", icon: Moon },
  { id: "system", label: "System", description: "Follow the OS color scheme", icon: Monitor },
];

function syncSchemeAria(scheme: ColorScheme) {
  document.querySelectorAll<HTMLElement>("[data-scheme-option]").forEach((el) => {
    el.setAttribute("aria-checked", el.dataset.schemeOption === scheme ? "true" : "false");
  });
}

function syncModeAria(preference: ThemePreference) {
  document.querySelectorAll<HTMLElement>("[data-mode-option]").forEach((el) => {
    el.setAttribute("aria-checked", el.dataset.modeOption === preference ? "true" : "false");
  });
}

export function AppearanceControls({ className }: { className?: string }) {
  useEffect(() => {
    const scheme = readSchemeFromDom();
    const preference = readThemePreferenceFromDom();
    applyAppearance(scheme, preference);
    syncSchemeAria(scheme);
    syncModeAria(preference);

    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => {
      const pref = readThemePreferenceFromDom();
      if (pref === "system") {
        applyAppearance(readSchemeFromDom(), "system");
      }
    };
    media.addEventListener("change", onChange);
    return () => media.removeEventListener("change", onChange);
  }, []);

  function selectScheme(next: ColorScheme) {
    applyAppearance(next, readThemePreferenceFromDom());
    persistScheme(next);
    syncSchemeAria(next);
  }

  function selectMode(next: ThemePreference) {
    applyAppearance(readSchemeFromDom(), next);
    persistThemePreference(next);
    syncModeAria(next);
  }

  return (
    <details className={cn("scheme-picker mode-picker group relative", className)}>
      <summary className="inline-flex h-9 cursor-pointer list-none items-center gap-2 rounded-sm border border-border bg-background px-3 text-sm font-medium text-foreground hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background [&::-webkit-details-marker]:hidden">
        <Palette className="size-4" aria-hidden />
        Appearance
      </summary>
      <div className="absolute right-0 top-full z-20 mt-1 grid min-w-64 gap-4 rounded-sm border border-border bg-popover p-3 text-popover-foreground shadow-sm">
        <fieldset className="grid gap-2">
          <legend className="text-sm font-medium">Color scheme</legend>
          <div
            role="radiogroup"
            aria-label="Color scheme"
            className="grid grid-cols-2 gap-1"
          >
            {COLOR_SCHEMES.map((item) => (
              <button
                key={item.id}
                type="button"
                role="radio"
                data-scheme-option={item.id}
                aria-label={`${item.label}: ${item.description}`}
                title={`${item.label} — ${item.description}`}
                onClick={() => selectScheme(item.id)}
                className={cn(
                  "scheme-option inline-flex h-9 items-center gap-2 rounded-sm px-2 text-xs font-medium",
                  "text-foreground/85 hover:bg-muted hover:text-foreground",
                  "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring",
                )}
              >
                <span
                  aria-hidden
                  className="relative flex size-3 shrink-0 overflow-hidden rounded-sm border border-border"
                >
                  <span className="h-full w-1/2" style={{ background: item.swatch.light }} />
                  <span className="h-full w-1/2" style={{ background: item.swatch.dark }} />
                  <span
                    className="absolute bottom-0 right-0 size-1.5"
                    style={{ background: item.swatch.accent }}
                  />
                </span>
                <span className="whitespace-nowrap">{item.label}</span>
              </button>
            ))}
          </div>
        </fieldset>

        <fieldset className="grid gap-2 border-t border-border pt-3">
          <legend className="sr-only">Color mode</legend>
          <p className="text-sm font-medium">Color mode</p>
          <div role="radiogroup" aria-label="Color mode" className="grid grid-cols-3 gap-1">
            {MODE_OPTIONS.map((item) => {
              const Icon = item.icon;
              return (
                <button
                  key={item.id}
                  type="button"
                  role="radio"
                  data-mode-option={item.id}
                  aria-label={`${item.label}: ${item.description}`}
                  title={`${item.label} — ${item.description}`}
                  onClick={() => selectMode(item.id)}
                  className={cn(
                    "mode-option inline-flex h-9 items-center justify-center gap-1.5 rounded-sm px-2 text-xs font-medium",
                    "text-foreground/85 hover:bg-muted hover:text-foreground",
                    "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring",
                  )}
                >
                  <Icon className="size-3.5 shrink-0" aria-hidden />
                  <span>{item.label}</span>
                </button>
              );
            })}
          </div>
        </fieldset>
      </div>
    </details>
  );
}

/** @deprecated Prefer AppearanceControls; kept for import compatibility. */
export function ThemeToggle() {
  return <AppearanceControls />;
}
