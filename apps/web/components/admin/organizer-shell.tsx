import type { AdminOverview } from '@/lib/admin-dashboard-types';
import Link from 'next/link';
import type { ReactNode } from 'react';

import { OrganizerStatusSummary } from '@/components/admin/organizer-status-summary';
import { Button } from '@/components/ui/button';
import { ThemeToggle } from '@/components/ui/theme-toggle';
import { cn } from '@/lib/utils';

const navItems = [
  { href: '/admin', label: 'Overview' },
  { href: '/admin/teams', label: 'Teams' },
  { href: '/admin/players', label: 'Players' },
  { href: '/admin/challenges', label: 'Challenges' },
  { href: '/admin/deployments', label: 'Deployments' },
  { href: '/admin/game', label: 'Game' },
  { href: '/admin/scoreboard', label: 'Scoreboard' },
  { href: '/admin/attacks', label: 'Attacks' },
  { href: '/admin/audit', label: 'Audit' },
] as const;

type Props = {
  activePath: string;
  overview: AdminOverview;
  children: ReactNode;
  title?: string;
  description?: string;
};

export function OrganizerShell({
  activePath,
  overview,
  children,
  title = 'Organizer',
  description = 'Teams, players, challenges, deployments, and live match operations.',
}: Props) {
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
            <Button asChild variant="outline">
              <a href="/">Participant</a>
            </Button>
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

        <OrganizerStatusSummary overview={overview} />

        {children}
      </div>
    </main>
  );
}
