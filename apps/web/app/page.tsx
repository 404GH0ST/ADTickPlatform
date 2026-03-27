import Link from 'next/link';

import { ParticipantShell } from '@/components/dashboard/participant-shell';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
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
      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <OverviewCard
          href="/scoreboard"
          title="Scoreboard"
          description="See the live standings without the rest of the participant controls."
          meta={`${dashboard.scores.length} teams live`}
        />
        <OverviewCard
          href="/services"
          title="Services"
          description="Manage unlocks, SSH access, restarts, and resets for your own services."
          meta={`${dashboard.services.length} services under your team`}
        />
        <OverviewCard
          href="/attacks"
          title="Attacks"
          description="Review accepted submissions and recent attack activity."
          meta={`${dashboard.attackPage.total_count} accepted events`}
        />
        <OverviewCard
          href="/docs/participant"
          title="Manual & API"
          description="Open the participant workflow, Swagger UI, and raw OpenAPI spec."
          meta="manual, swagger, openapi"
        />
      </section>
    </ParticipantShell>
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
