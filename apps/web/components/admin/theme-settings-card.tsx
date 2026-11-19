"use client";

import { useRouter } from "next/navigation";
import { useLayoutEffect, useRef, useState, useTransition } from "react";

import { useActiveTheme } from "@/components/theme/active-theme-provider";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { moveCompositeFocus } from "@/lib/composite-focus";
import { getThemes } from "@/lib/themes";
import { cn } from "@/lib/utils";

type SaveState = { kind: "idle" } | { kind: "saving"; id: string } | { kind: "error"; message: string };

// Changing to a theme with a different shell remounts the settings card.
let themeFocusAfterRefresh: { id: string; url: string } | undefined;

async function saveTheme(themeID: string) {
  const response = await fetch("/api/admin/platform/appearance", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ theme_id: themeID }),
    cache: "no-store",
  });
  if (!response.ok) {
    const payload = (await response.json().catch(() => null)) as { detail?: string; title?: string } | null;
    throw new Error(payload?.detail ?? payload?.title ?? `theme update failed with status ${response.status}`);
  }
}

/**
 * Organizer-wide theme. Themes come from apps/web/themes/<id>/ folders; adding
 * one there (and rebuilding) makes it show up here. Participants keep their
 * own light/dark choice on top of whichever theme is active.
 */
export function ThemeSettingsCard() {
  const router = useRouter();
  const active = useActiveTheme();
  const [state, setState] = useState<SaveState>({ kind: "idle" });
  const [refreshing, startRefresh] = useTransition();
  const busy = state.kind === "saving" || refreshing;
  const checkedOption = useRef<HTMLButtonElement>(null);

  useLayoutEffect(() => {
    if (themeFocusAfterRefresh?.id !== active.id) return;

    const { url } = themeFocusAfterRefresh;
    themeFocusAfterRefresh = undefined;
    if (url === window.location.href && document.activeElement === document.body) {
      checkedOption.current?.focus();
    }
  }, [active.id]);

  async function select(themeID: string, option: HTMLButtonElement) {
    if (themeID === active.id || busy) {
      return;
    }
    setState({ kind: "saving", id: themeID });
    try {
      await saveTheme(themeID);
      setState({ kind: "idle" });
      themeFocusAfterRefresh = document.activeElement === option
        ? { id: themeID, url: window.location.href }
        : undefined;
      // Re-render the root layout so html[data-scheme] and layout overrides switch.
      startRefresh(() => router.refresh());
    } catch (error) {
      setState({ kind: "error", message: error instanceof Error ? error.message : String(error) });
    }
  }

  return (
    <Card data-testid="theme-settings-card">
      <CardHeader>
        <CardTitle>Theme</CardTitle>
        <CardDescription>
          Applies to every page for all participants. Participants can still switch between light and dark.
        </CardDescription>
      </CardHeader>
      <CardContent className="grid gap-3">
        <div
          role="radiogroup"
          aria-label="Platform theme"
          aria-busy={busy || undefined}
          className="@container grid grid-cols-1 gap-2 sm:grid-cols-2"
          onKeyDown={(event) => {
            if (!busy) moveCompositeFocus(event, "[data-theme-option]", true)?.click();
          }}
        >
          {getThemes().map((theme) => {
            const checked = theme.id === active.id;
            const saving = state.kind === "saving" && state.id === theme.id;
            return (
              <button
                key={theme.id}
                ref={checked ? checkedOption : undefined}
                type="button"
                role="radio"
                aria-checked={checked}
                tabIndex={checked ? 0 : -1}
                data-theme-option={theme.id}
                aria-disabled={busy || undefined}
                onClick={(event) => void select(theme.id, event.currentTarget)}
                className={cn(
                  "flex items-center gap-3 rounded-sm border border-border bg-background p-3 text-left",
                  "@max-[12rem]:flex-col @max-[12rem]:items-start @max-[12rem]:gap-2",
                  "hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                  "aria-disabled:cursor-wait",
                  checked && "border-foreground/60 bg-muted",
                )}
              >
                <span aria-hidden className="relative flex h-8 w-10 shrink-0 overflow-hidden rounded-sm border border-border">
                  <span className="h-full w-1/2" style={{ background: theme.swatch.light }} />
                  <span className="h-full w-1/2" style={{ background: theme.swatch.dark }} />
                  <span className="absolute bottom-0 right-0 size-3" style={{ background: theme.swatch.accent }} />
                </span>
                <span className="min-w-0 space-y-0.5">
                  <span className="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm font-medium text-foreground">
                    <span className="min-w-0 wrap-anywhere">{theme.label}</span>
                    {checked ? (
                      <span className="rounded-sm border border-border px-1.5 text-[11px] font-medium leading-5">Active</span>
                    ) : null}
                    {saving ? <span className="text-xs text-muted-foreground">Applying…</span> : null}
                  </span>
                  <span className="block wrap-anywhere text-xs text-muted-foreground">{theme.description}</span>
                </span>
              </button>
            );
          })}
        </div>
        {state.kind === "error" ? (
          <p role="alert" className="text-sm text-destructive">
            {state.message}
          </p>
        ) : null}
      </CardContent>
    </Card>
  );
}
