import Link from 'next/link';
import type { ReactNode } from 'react';

import { AppearanceControls } from '@/components/ui/appearance-controls';
import { WorkspaceFrame } from '@/components/ui/workspace-frame';

export type NavItem = {
  href: string;
  label: string;
};

export type AppShellProps = {
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

export function AppShell({
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
          <div className="flex flex-wrap gap-0.5 rounded-sm border border-border bg-background/60 p-0.5">
            {navItems.map((item) => (
              <Link
                key={item.href}
                href={item.href}
                data-active={activePath === item.href ? 'true' : 'false'}
                className="shell-nav-link"
              >
                {item.label}
              </Link>
            ))}
            {secondaryNavItems.length > 0 ? (
              <details className="group relative">
                <summary
                  className="shell-nav-link cursor-pointer list-none [&::-webkit-details-marker]:hidden"
                  data-active={
                    secondaryNavItems.some((item) => activePath === item.href)
                      ? 'true'
                      : 'false'
                  }
                >
                  More
                </summary>
                <div className="absolute left-0 top-full z-20 mt-1 min-w-44 rounded-sm border border-border bg-popover p-1 text-popover-foreground shadow-sm">
                  {secondaryNavItems.map((item) => (
                    <Link
                      key={item.href}
                      href={item.href}
                      data-active={activePath === item.href ? 'true' : 'false'}
                      className="shell-nav-link block"
                    >
                      {item.label}
                    </Link>
                  ))}
                </div>
              </details>
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
