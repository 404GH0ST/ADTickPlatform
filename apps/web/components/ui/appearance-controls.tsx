"use client";

import { Monitor, Moon, Palette, Sun } from "lucide-react";
import { useEffect } from "react";

import {
  applyThemePreference,
  type ThemePreference,
  persistThemePreference,
  readThemePreferenceFromDom,
} from "@/lib/appearance";
import { cn } from "@/lib/utils";

/**
 * Participant appearance: light / dark / system only. The theme (colors and
 * layout) is chosen by the organizer; see components/admin/theme-settings-card.
 *
 * Selection chrome is CSS-driven from html data attributes so navigation
 * remounts never flash the wrong mode.
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

function syncModeAria(preference: ThemePreference) {
  document.querySelectorAll<HTMLElement>("[data-mode-option]").forEach((el) => {
    el.setAttribute("aria-checked", el.dataset.modeOption === preference ? "true" : "false");
  });
}

export function AppearanceControls({ className }: { className?: string }) {
  useEffect(() => {
    const preference = readThemePreferenceFromDom();
    applyThemePreference(preference);
    syncModeAria(preference);

    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => {
      const pref = readThemePreferenceFromDom();
      if (pref === "system") {
        applyThemePreference("system");
      }
    };
    media.addEventListener("change", onChange);
    return () => media.removeEventListener("change", onChange);
  }, []);

  function selectMode(next: ThemePreference) {
    applyThemePreference(next);
    persistThemePreference(next);
    syncModeAria(next);
  }

  return (
    <details className={cn("mode-picker group relative", className)}>
      <summary className="inline-flex h-9 cursor-pointer list-none items-center gap-2 rounded-sm border border-border bg-background px-3 text-sm font-medium text-foreground hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background [&::-webkit-details-marker]:hidden">
        <Palette className="size-4" aria-hidden />
        Appearance
      </summary>
      <div className="absolute right-0 top-full z-20 mt-1 grid min-w-64 gap-2 rounded-sm border border-border bg-popover p-3 text-popover-foreground shadow-sm">
        <fieldset className="grid gap-2">
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
