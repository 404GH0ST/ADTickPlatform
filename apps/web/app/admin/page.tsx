import { AnnouncementsPanel } from '@/components/admin/announcements-panel';
import { TrustedStatePanel } from '@/components/admin/trusted-state-panel';
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
      <div className="grid gap-4">
        <TrustedStatePanel />
        <AnnouncementsPanel />
        <section className="surface-workroom grid overflow-hidden rounded-sm border md:grid-cols-2 xl:grid-cols-4 [&>*]:border-b [&>*]:border-border [&>*:last-child]:border-b-0 md:[&>*:nth-child(odd)]:border-r xl:[&>*]:border-r xl:[&>*:nth-child(4n)]:border-r-0 xl:[&>*:nth-last-child(-n+3)]:border-b-0">
          <ActionCard href="/admin/teams" title="Teams" description="Create teams and review the current registry." meta={`${dashboard.teams.length} teams`} />
          <ActionCard href="/admin/players" title="Players" description="Manage accounts, WireGuard peers, and access sync." meta={`${dashboard.players.length} players`} />
          <ActionCard href="/admin/challenges" title="Challenges" description="Validate, publish, and deploy challenge packages." meta={`${dashboard.challenges.length} challenges`} />
          <ActionCard href="/admin/deployments" title="Deployments" description="Track rollout jobs and reconcile stale work." meta={`${dashboard.deployments.length} jobs`} />
          <ActionCard href="/admin/game" title="Game" description="Run the match window, scheduler, and checker flow." meta={`${dashboard.gameStatus.total_ticks} persisted ticks`} />
          <ActionCard href="/admin/scoreboard" title="Scoreboard" description="Inspect authoritative scores from game-core." meta={`${dashboard.scoreboard.length} rows`} />
          <ActionCard href="/admin/audit" title="Audit" description="Review privileged actions across teams and organizers." meta="persistent control trail" />
        </section>
      </div>
    </OrganizerShell>
  );
}
