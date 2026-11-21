import type { Metadata } from 'next';
import { ParticipantOverview } from '@/components/dashboard/participant-overview';
import { ParticipantShell } from '@/components/dashboard/participant-shell';
import { loadDashboardData } from '@/lib/dashboard-data';

export const metadata: Metadata = { title: { absolute: 'Overview | AD Platform' } };

export const dynamic = 'force-dynamic';

export default async function HomePage() {
  const dashboard = await loadDashboardData();

  return (
    <ParticipantShell
      activePath="/"
      overview={dashboard.platform}
      title="Participant Overview"
    >
      <ParticipantOverview
        teamName={dashboard.platform.teamName}
        scoreCount={dashboard.scores.length}
        serviceCount={dashboard.services.length}
        attackCount={dashboard.attackPage.total_count}
        unavailable={{
          scores: dashboard.platform.scoresUnavailable,
          services: dashboard.platform.servicesUnavailable,
          attacks: dashboard.platform.attacksUnavailable,
        }}
      />
    </ParticipantShell>
  );
}
