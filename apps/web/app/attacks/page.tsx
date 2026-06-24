import { ControlCenter } from "@/components/dashboard/control-center";
import { ParticipantShell } from "@/components/dashboard/participant-shell";
import { loadDashboardData } from "@/lib/dashboard-data";

export const dynamic = "force-dynamic";

// The attacks surface renders a map-first view of the full accepted-attack feed,
// so load the broad slice instead of the default paginated page.
const MAP_ATTACK_LIMIT = 1000;

export default async function AttacksPage() {
  const dashboard = await loadDashboardData({
    attackQuery: { limit: MAP_ATTACK_LIMIT },
  });

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
