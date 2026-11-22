import type { Metadata } from 'next';
import { OrganizerDashboard } from '@/components/admin/organizer-dashboard';
import { TrustedStatePanel } from '@/components/admin/trusted-state-panel';
import { OrganizerShell } from '@/components/admin/organizer-shell';
import { loadAdminDashboardData } from '@/lib/admin-dashboard-data';

export const metadata: Metadata = { title: 'Deployments' };

export const dynamic = 'force-dynamic';

export default async function AdminDeploymentsPage() {
  const dashboard = await loadAdminDashboardData();

  return (
    <OrganizerShell
      activePath="/admin/deployments"
      overview={dashboard.overview}
      title="Deployments"
    >
      <div className="grid gap-4">
        <TrustedStatePanel />
        <OrganizerDashboard
          attackPage={dashboard.attackPage}
          challenges={dashboard.challenges}
          checkerRunPage={dashboard.checkerRunPage}
          deployments={dashboard.deployments}
          gameStatus={dashboard.gameStatus}
          players={dashboard.players}
          serviceMetrics={dashboard.serviceMetrics}
          schedulerEventPage={dashboard.schedulerEventPage}
          scoreboard={dashboard.scoreboard}
          teams={dashboard.teams}
          unavailable={dashboard.unavailable}
          initialTab="deployments"
        />
      </div>
    </OrganizerShell>
  );
}
