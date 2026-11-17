"use client";

import { useRouter } from "next/navigation";
import { useState, useTransition } from "react";

import { useActiveTheme } from "@/components/theme/active-theme-provider";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { getThemes } from "@/lib/themes";
import { cn } from "@/lib/utils";

type SaveState = { kind: "idle" } | { kind: "saving"; id: string } | { kind: "error"; message: string };

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

  async function select(themeID: string) {
    if (themeID === active.id || state.kind === "saving") {
      return;
    }
    setState({ kind: "saving", id: themeID });
    try {
      await saveTheme(themeID);
      setState({ kind: "idle" });
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
        <div role="radiogroup" aria-label="Platform theme" className="grid gap-2 sm:grid-cols-2">
          {getThemes().map((theme) => {
            const checked = theme.id === active.id;
            const saving = state.kind === "saving" && state.id === theme.id;
            return (
              <button
                key={theme.id}
                type="button"
                role="radio"
                aria-checked={checked}
                data-theme-option={theme.id}
                disabled={state.kind === "saving" || refreshing}
                onClick={() => void select(theme.id)}
                className={cn(
                  "flex items-center gap-3 rounded-sm border border-border bg-background p-3 text-left",
                  "hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                  "disabled:cursor-wait",
                  checked && "border-foreground/60 bg-muted",
                )}
              >
                <span aria-hidden className="relative flex h-8 w-10 shrink-0 overflow-hidden rounded-sm border border-border">
                  <span className="h-full w-1/2" style={{ background: theme.swatch.light }} />
                  <span className="h-full w-1/2" style={{ background: theme.swatch.dark }} />
                  <span className="absolute bottom-0 right-0 size-3" style={{ background: theme.swatch.accent }} />
                </span>
                <span className="grid min-w-0 gap-0.5">
                  <span className="flex items-center gap-2 text-sm font-medium text-foreground">
                    {theme.label}
                    {checked ? (
                      <span className="rounded-sm border border-border px-1.5 text-[11px] font-medium leading-5">Active</span>
                    ) : null}
                    {saving ? <span className="text-xs text-muted-foreground">Applying…</span> : null}
                  </span>
                  <span className="truncate text-xs text-muted-foreground">{theme.description}</span>
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
