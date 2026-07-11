import { ControlCenter } from "@/components/dashboard/control-center";
import { ParticipantShell } from "@/components/dashboard/participant-shell";
import { loadDashboardData } from "@/lib/dashboard-data";

export const dynamic = "force-dynamic";

export default async function ServicesPage() {
  const dashboard = await loadDashboardData();

  return (
    <ParticipantShell
      activePath="/services"
      overview={dashboard.platform}
      title="Services"
    >
      <ControlCenter
        attackPage={dashboard.attackPage}
        realtimeBaseUrl={dashboard.platform.realtimeBaseUrl}
        scores={dashboard.scores}
        services={dashboard.services}
        initialTab="services"
        currentTeamName={dashboard.platform.teamName}
        overview={dashboard.platform}
      />
    </ParticipantShell>
  );
}
