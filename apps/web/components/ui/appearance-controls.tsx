"use client";

import { Monitor, Moon, Sun } from "lucide-react";
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
 * Compact appearance chrome:
 * - Scheme: one chip; hover/focus expands to all four options
 * - Mode: one chip; hover/focus expands to light / dark / system
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
    <div className={cn("flex flex-wrap items-center gap-2", className)}>
      <div
        role="radiogroup"
        aria-label="Color scheme. Hover or focus to choose Graphite, Ink, Paper, or Moss."
        className="expand-picker scheme-picker inline-flex h-9 items-stretch overflow-hidden rounded-sm border border-border bg-background"
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
              "expand-option scheme-option inline-flex h-full items-center gap-1.5 text-xs font-medium",
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

      <div
        role="radiogroup"
        aria-label="Color mode. Hover or focus to choose Light, Dark, or System."
        className="expand-picker mode-picker inline-flex h-9 items-stretch overflow-hidden rounded-sm border border-border bg-background"
      >
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
                "expand-option mode-option inline-flex h-full items-center gap-1.5 text-xs font-medium",
                "text-foreground/85 hover:bg-muted hover:text-foreground",
                "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring",
              )}
            >
              <Icon className="size-3.5 shrink-0" aria-hidden />
              <span className="whitespace-nowrap">{item.label}</span>
            </button>
          );
        })}
      </div>
    </div>
  );
}

/** @deprecated Prefer AppearanceControls; kept for import compatibility. */
export function ThemeToggle() {
  return <AppearanceControls />;
}
