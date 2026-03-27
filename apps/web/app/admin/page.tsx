import Link from 'next/link';

import { OrganizerShell } from '@/components/admin/organizer-shell';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { loadAdminDashboardData } from '@/lib/admin-dashboard-data';

export const dynamic = 'force-dynamic';

export default async function AdminPage() {
  const dashboard = await loadAdminDashboardData();

  return (
    <OrganizerShell
      activePath="/admin"
      overview={dashboard.overview}
      title="Organizer Overview"
      description="Open the control surfaces for teams, challenges, deployments, and live game state."
    >
      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-7">
        <OverviewCard href="/admin/teams" title="Teams" description="Create teams and review the current registry." meta={`${dashboard.teams.length} teams`} />
        <OverviewCard href="/admin/players" title="Players" description="Manage accounts, WireGuard peers, and access sync." meta={`${dashboard.players.length} players`} />
        <OverviewCard href="/admin/challenges" title="Challenges" description="Validate, publish, and deploy challenge packages." meta={`${dashboard.challenges.length} challenges`} />
        <OverviewCard href="/admin/deployments" title="Deployments" description="Track rollout jobs and reconcile stale work." meta={`${dashboard.deployments.length} jobs`} />
        <OverviewCard href="/admin/game" title="Game" description="Run the match window, scheduler, and checker flow." meta={`${dashboard.gameStatus.total_ticks} persisted ticks`} />
        <OverviewCard href="/admin/scoreboard" title="Scoreboard" description="Inspect authoritative scores from game-core." meta={`${dashboard.scoreboard.length} rows`} />
        <OverviewCard href="/admin/audit" title="Audit" description="Review privileged actions across teams and organizers." meta="persistent control trail" />
      </section>
    </OrganizerShell>
  );
}

function OverviewCard({ href, title, description, meta }: { href: string; title: string; description: string; meta: string }) {
  return (
    <Link href={href} className="block min-w-0">
      <Card className="h-full border-border/80 transition-colors hover:border-foreground/30 hover:bg-muted/20">
        <CardHeader>
          <CardTitle>{title}</CardTitle>
          <CardDescription>{description}</CardDescription>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">{meta}</p>
        </CardContent>
      </Card>
    </Link>
  );
}
