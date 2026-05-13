

import { ParticipantShell } from '@/components/dashboard/participant-shell';
import { ActionCard } from '@/components/ui/action-card';
import { loadDashboardData } from '@/lib/dashboard-data';

export const dynamic = 'force-dynamic';

export default async function HomePage() {
  const dashboard = await loadDashboardData();

  return (
    <ParticipantShell
      activePath="/"
      overview={dashboard.platform}
      title="Participant Overview"
      description="Move between the live scoreboard, your services, recent attacks, and the participant manual."
    >
      <section className="surface-workroom grid overflow-hidden rounded-sm border md:grid-cols-2 xl:grid-cols-4 [&>*]:border-b [&>*]:border-border [&>*:last-child]:border-b-0 md:[&>*:nth-child(odd)]:border-r md:[&>*:nth-last-child(-n+2)]:border-b-0 xl:[&>*]:border-b-0 xl:[&>*]:border-r xl:[&>*:last-child]:border-r-0">
        <ActionCard
          href="/scoreboard"
          title="Scoreboard"
          description="See the live standings without the rest of the participant controls."
          meta={`${dashboard.scores.length} teams live`}
        />
        <ActionCard
          href="/services"
          title="Services"
          description="Manage unlocks, SSH access, restarts, and resets for your own services."
          meta={`${dashboard.services.length} services under your team`}
        />
        <ActionCard
          href="/attacks"
          title="Attacks"
          description="Review accepted submissions and recent attack activity."
          meta={`${dashboard.attackPage.total_count} accepted events`}
        />
        <ActionCard
          href="/docs/participant"
          title="Manual & API"
          description="Open the participant workflow, Swagger UI, and raw OpenAPI spec."
          meta="manual, swagger, openapi"
        />
      </section>
    </ParticipantShell>
  );
}
