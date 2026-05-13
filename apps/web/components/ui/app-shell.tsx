import Link from 'next/link';
import type { ReactNode } from 'react';

import { ThemeToggle } from '@/components/ui/theme-toggle';
import { cn } from '@/lib/utils';

export type NavItem = {
  href: string;
  label: string;
};

export type AppShellProps = {
  activePath: string;
  navItems: readonly NavItem[];
  title: string;
  description: string;
  children: ReactNode;
  headerActions?: ReactNode;
  summarySection?: ReactNode;
  alertSection?: ReactNode;
};

export function AppShell({
  activePath,
  navItems,
  title,
  description,
  children,
  headerActions,
  summarySection,
  alertSection,
}: AppShellProps) {
  return (
    <main className="min-h-screen bg-background text-foreground">
      <div className="mx-auto flex w-full max-w-[90rem] flex-col gap-3 px-3 py-4 sm:px-5 lg:px-6">
        <header className="workroom-rule flex flex-col gap-3 border-b pb-3 lg:flex-row lg:items-end lg:justify-between">
          <div className="max-w-3xl space-y-1">
            <h1 className="text-2xl font-semibold leading-8">{title}</h1>
            <p className="text-sm leading-6 text-muted-foreground">{description}</p>
          </div>
          <div className="flex flex-wrap gap-2">
            <ThemeToggle />
            {headerActions}
          </div>
        </header>

        <nav className="workroom-rule flex flex-wrap gap-1 border-b pb-3">
          {navItems.map((item) => (
            <Link
              key={item.href}
              href={item.href}
              className={cn(
                'rounded-sm border border-transparent px-2.5 py-1.5 text-sm text-muted-foreground transition-colors hover:border-border hover:bg-muted hover:text-foreground',
                activePath === item.href && 'border-border bg-card text-foreground shadow-[0_1px_0_var(--border)]',
              )}
            >
              {item.label}
            </Link>
          ))}
        </nav>

        {summarySection}
        {alertSection}

        {children}
      </div>
    </main>
  );
}
