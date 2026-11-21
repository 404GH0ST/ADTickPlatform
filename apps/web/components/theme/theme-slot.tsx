"use client";

import { Component, type ComponentType, type ReactNode } from "react";

import { useActiveTheme } from "@/components/theme/active-theme-provider";
import { HydrationSuspense } from "@/components/ui/hydration-suspense";
import type { ThemeSlotName, ThemeSlotProps } from "@/lib/theme-contract";
import { GENERATED_THEME_VIEWS } from "@/themes/views.generated";

type BoundaryProps = { slot: string; themeId: string; fallback: ReactNode; children: ReactNode };

/**
 * Keeps a broken theme from taking a page down: if the override throws while
 * rendering in the browser, the stock view is shown instead and the error is
 * logged. (Server-side throws are handled by the Suspense boundary below.)
 */
class SlotErrorBoundary extends Component<BoundaryProps, { failed: boolean }> {
  state = { failed: false };

  static getDerivedStateFromError() {
    return { failed: true };
  }

  componentDidCatch(error: unknown) {
    console.error(
      `[theme:${this.props.themeId}] ${this.props.slot} override failed; using the default view.`,
      error,
    );
  }

  render() {
    return this.state.failed ? this.props.fallback : this.props.children;
  }
}

/**
 * Renders the active theme's override for `name`, or `fallback` (the stock
 * view) when the theme does not provide one.
 */
export function ThemeSlot<K extends ThemeSlotName>({
  name,
  fallback: Fallback,
  props,
}: {
  name: K;
  fallback: ComponentType<ThemeSlotProps[K]>;
  props: ThemeSlotProps[K];
}) {
  const theme = useActiveTheme();
  const Override = (GENERATED_THEME_VIEWS[theme.id]?.[name] ?? theme.components?.[name]) as ComponentType<ThemeSlotProps[K]> | undefined;
  // Generic props spread: TS cannot prove ThemeSlotProps[K] is an object, so widen once here.
  const p = props as ThemeSlotProps[K] & object;
  if (!Override) {
    return <Fallback {...p} />;
  }
  // Error boundaries do not run during server rendering. When a component
  // throws on the server, React instead emits the nearest Suspense fallback
  // and retries in the browser, where SlotErrorBoundary catches a repeated
  // throw and shows the stock view. The Suspense fallback is null, not the
  // stock view: views legitimately suspend while server content streams, and
  // a stock fallback would flash the stock layout before the themed one.
  return (
    <SlotErrorBoundary key={theme.id} slot={name} themeId={theme.id} fallback={<Fallback {...p} />}>
      <HydrationSuspense fallback={null}>
        <Override {...p} />
      </HydrationSuspense>
    </SlotErrorBoundary>
  );
}

/**
 * Wraps a stock component so it renders through a slot:
 *   export const ThemedTeamsTab = themed("OrganizerTeams", TeamsTab);
 * Call sites keep plain JSX (<ThemedTeamsTab {...props} />).
 */
export function themed<K extends ThemeSlotName>(
  name: K,
  fallback: ComponentType<ThemeSlotProps[K]>,
): ComponentType<ThemeSlotProps[K]> {
  function Themed(props: ThemeSlotProps[K]) {
    return <ThemeSlot name={name} fallback={fallback} props={props} />;
  }
  Themed.displayName = `Themed(${name})`;
  return Themed;
}
