"use client";

import { Check, Monitor, Moon, Palette, Sun } from "lucide-react";
import { useEffect, useId, useRef } from "react";

import {
  applyThemePreference,
  type ThemePreference,
  persistThemePreference,
  readThemePreferenceFromDom,
} from "@/lib/appearance";
import { moveCompositeFocus } from "@/lib/composite-focus";
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
    const selected = el.dataset.modeOption === preference;
    el.setAttribute("aria-checked", String(selected));
    el.tabIndex = selected ? 0 : -1;
  });
}

export function AppearanceControls({ className }: { className?: string }) {
  const labelId = useId();
  const detailsRef = useRef<HTMLDetailsElement>(null);
  const summaryRef = useRef<HTMLElement>(null);

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
    const onPointerDown = (event: PointerEvent) => {
      const details = detailsRef.current;
      if (details?.open && event.target instanceof Node && !details.contains(event.target)) {
        details.open = false;
      }
    };
    media.addEventListener("change", onChange);
    document.addEventListener("pointerdown", onPointerDown);
    return () => {
      media.removeEventListener("change", onChange);
      document.removeEventListener("pointerdown", onPointerDown);
    };
  }, []);

  function selectMode(next: ThemePreference) {
    applyThemePreference(next);
    persistThemePreference(next);
    syncModeAria(next);
  }

  return (
    <details
      ref={detailsRef}
      className={cn("mode-picker group relative", className)}
      onKeyDown={(event) => {
        if (event.key !== "Escape" || !event.currentTarget.open) return;
        event.preventDefault();
        event.currentTarget.open = false;
        summaryRef.current?.focus();
      }}
    >
      <summary ref={summaryRef} className="inline-flex h-9 cursor-pointer list-none items-center gap-2 rounded-sm border border-border bg-background px-3 text-sm font-medium text-foreground hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background [&::-webkit-details-marker]:hidden">
        <Palette className="size-4" aria-hidden />
        Appearance
      </summary>
      <div className="absolute right-0 top-full z-20 mt-1 grid w-64 max-w-[calc(100vw-3rem)] gap-2 rounded-sm border border-border bg-popover p-3 text-popover-foreground shadow-sm">
        <p id={labelId} className="text-sm font-medium">Color mode</p>
        <div
          role="radiogroup"
          aria-labelledby={labelId}
          className="flex flex-wrap gap-1"
          onKeyDown={(event) => moveCompositeFocus(event, "[data-mode-option]", true)?.click()}
        >
          {MODE_OPTIONS.map((item) => {
            const Icon = item.icon;
            return (
              <button
                key={item.id}
                type="button"
                role="radio"
                aria-checked={item.id === "system"}
                tabIndex={item.id === "system" ? 0 : -1}
                data-mode-option={item.id}
                aria-label={`${item.label}: ${item.description}`}
                title={`${item.label} — ${item.description}`}
                onClick={() => selectMode(item.id)}
                className={cn(
                  "mode-option group/option inline-flex min-h-9 grow items-center justify-center gap-1.5 rounded-sm px-1.5 py-1 text-xs font-medium",
                  "text-foreground/85 hover:bg-muted hover:text-foreground",
                  "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring",
                )}
              >
                <Icon className="size-3.5 shrink-0 group-aria-checked/option:hidden" aria-hidden />
                <Check className="hidden size-3.5 shrink-0 group-aria-checked/option:block" aria-hidden />
                <span className="whitespace-nowrap">{item.label}</span>
              </button>
            );
          })}
        </div>
      </div>
    </details>
  );
}

/** @deprecated Prefer AppearanceControls; kept for import compatibility. */
export function ThemeToggle() {
  return <AppearanceControls />;
}
