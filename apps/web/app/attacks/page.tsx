import { ControlCenter } from "@/components/dashboard/control-center";
import { ParticipantShell } from "@/components/dashboard/participant-shell";
import { loadDashboardData } from "@/lib/dashboard-data";

export const dynamic = "force-dynamic";

export default async function AttacksPage() {
  const dashboard = await loadDashboardData();

  return (
    <ParticipantShell
      activePath="/attacks"
      overview={dashboard.platform}
      title="Attacks"
      description="Accepted attack events with shared filters, globe context, pagination, and live updates."
    >
      <ControlCenter
        attackPage={dashboard.attackPage}
        realtimeBaseUrl={dashboard.platform.realtimeBaseUrl}
        scores={dashboard.scores}
        services={dashboard.services}
        initialTab="attacks"
        currentTeamName={dashboard.platform.teamName}
        overview={dashboard.platform}
      />
    </ParticipantShell>
  );
}
