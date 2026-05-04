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
      <div className="mx-auto flex w-full max-w-7xl flex-col gap-5 px-4 py-5 sm:px-6 lg:px-8">
        <header className="flex flex-col gap-3 border-b pb-3 lg:flex-row lg:items-end lg:justify-between">
          <div className="space-y-1">
            <h1 className="text-2xl font-semibold">{title}</h1>
            <p className="text-sm text-muted-foreground">{description}</p>
          </div>
          <div className="flex flex-wrap gap-2">
            <ThemeToggle />
            {headerActions}
          </div>
        </header>

        <nav className="flex flex-wrap gap-2 border-b pb-3">
          {navItems.map((item) => (
            <Link
              key={item.href}
              href={item.href}
              className={cn(
                'rounded-md border px-3 py-2 text-sm text-muted-foreground transition-colors hover:text-foreground',
                activePath === item.href && 'border-foreground bg-card text-foreground',
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
