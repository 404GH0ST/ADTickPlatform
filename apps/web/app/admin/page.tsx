

import { OrganizerShell } from '@/components/admin/organizer-shell';
import { ActionCard } from '@/components/ui/action-card';
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
        <ActionCard href="/admin/teams" title="Teams" description="Create teams and review the current registry." meta={`${dashboard.teams.length} teams`} />
        <ActionCard href="/admin/players" title="Players" description="Manage accounts, WireGuard peers, and access sync." meta={`${dashboard.players.length} players`} />
        <ActionCard href="/admin/challenges" title="Challenges" description="Validate, publish, and deploy challenge packages." meta={`${dashboard.challenges.length} challenges`} />
        <ActionCard href="/admin/deployments" title="Deployments" description="Track rollout jobs and reconcile stale work." meta={`${dashboard.deployments.length} jobs`} />
        <ActionCard href="/admin/game" title="Game" description="Run the match window, scheduler, and checker flow." meta={`${dashboard.gameStatus.total_ticks} persisted ticks`} />
        <ActionCard href="/admin/scoreboard" title="Scoreboard" description="Inspect authoritative scores from game-core." meta={`${dashboard.scoreboard.length} rows`} />
        <ActionCard href="/admin/audit" title="Audit" description="Review privileged actions across teams and organizers." meta="persistent control trail" />
      </section>
    </OrganizerShell>
  );
}
