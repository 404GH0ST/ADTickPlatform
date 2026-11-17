"use client";

import { DefaultAppShell, type AppShellProps } from "@/components/ui/default-app-shell";
import { ThemeSlot } from "@/components/theme/theme-slot";

export type { AppShellProps, NavItem } from "@/components/ui/default-app-shell";

/** Renders the active theme's shell override, or the default shell. */
export function AppShell(props: AppShellProps) {
  return <ThemeSlot name="AppShell" fallback={DefaultAppShell} props={props} />;
}
