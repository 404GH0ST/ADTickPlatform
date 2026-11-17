import Link from 'next/link';
import type { ReactNode } from 'react';

import { AppearanceControls } from '@/components/ui/appearance-controls';
import { WorkspaceFrame } from '@/components/ui/workspace-frame';

export type NavItem = {
  href: string;
  label: string;
};

export type AppShellProps = {
  /**
   * Which side of the platform is rendering. Themes can use it to give the
   * organizer console a different layout, or to keep it on DefaultAppShell.
   */
  surface?: 'participant' | 'organizer';
  activePath: string;
  navItems: readonly NavItem[];
  secondaryNavItems?: readonly NavItem[];
  title: string;
  children: ReactNode;
  headerActions?: ReactNode;
  /** Right side of the primary nav row (e.g. match readiness). */
  navActions?: ReactNode;
  /** Directly under the nav, above summary/alerts (e.g. expanded readiness). */
  belowNav?: ReactNode;
  summarySection?: ReactNode;
  alertSection?: ReactNode;
};

/**
 * The stock page chrome. Themes may replace it via `components.AppShell` in
 * their theme.ts; see components/ui/app-shell.tsx for the dispatch.
 */
export function DefaultAppShell({
  activePath,
  navItems,
  secondaryNavItems = [],
  title,
  children,
  headerActions,
  navActions,
  belowNav,
  summarySection,
  alertSection,
}: AppShellProps) {
  return (
    <main className="workspace-canvas text-foreground">
      <WorkspaceFrame>
        <header className="workroom-rule flex flex-col gap-3 border-b pb-3 sm:flex-row sm:items-center sm:justify-between">
          <h1 className="text-balance text-2xl font-semibold tracking-tight text-foreground">
            {title}
          </h1>
          <div className="flex shrink-0 flex-wrap items-center gap-2">
            <AppearanceControls />
            {headerActions}
          </div>
        </header>

        <nav
          className="workroom-rule flex flex-wrap items-center gap-1 border-b pb-3"
          aria-label="Primary"
        >
          <div className="flex max-w-full gap-0.5 overflow-x-auto rounded-sm border border-border bg-background/60 p-0.5">
            {navItems.map((item) => (
              <Link
                key={item.href}
                href={item.href}
                data-active={activePath === item.href ? 'true' : 'false'}
                className="shell-nav-link shrink-0"
              >
                {item.label}
              </Link>
            ))}
            {secondaryNavItems.length > 0 ? (
              <>
                <span aria-hidden className="mx-1 my-1.5 w-px shrink-0 self-stretch bg-border" />
                {secondaryNavItems.map((item) => (
                  <Link
                    key={item.href}
                    href={item.href}
                    data-active={activePath === item.href ? 'true' : 'false'}
                    className="shell-nav-link shrink-0"
                  >
                    {item.label}
                  </Link>
                ))}
              </>
            ) : null}
          </div>
          {navActions ? (
            <div className="ml-auto flex flex-wrap items-center gap-2 pt-1 sm:pt-0">
              {navActions}
            </div>
          ) : null}
        </nav>

        {belowNav}

        {summarySection}
        {alertSection}

        <div className="flex min-h-0 flex-1 flex-col gap-3">{children}</div>
      </WorkspaceFrame>
    </main>
  );
}
