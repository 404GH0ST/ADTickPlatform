import { ControlCenter } from "@/components/dashboard/control-center";
import { ParticipantShell } from "@/components/dashboard/participant-shell";
import { loadDashboardData } from "@/lib/dashboard-data";

export const dynamic = "force-dynamic";
const MAP_ATTACK_LIMIT = 1000;

export default async function AttacksPage() {
  const dashboard = await loadDashboardData({
    attackQuery: { limit: MAP_ATTACK_LIMIT },
  });

  return (
    <ParticipantShell
      activePath="/attacks"
      overview={dashboard.platform}
      title="Attack Map"
      description="Dedicated live attack monitoring with the broad accepted-attack feed rendered as a map-first surface."
    >
      <ControlCenter
        attackFocusMode="map"
        attackPage={dashboard.attackPage}
        realtimeBaseUrl={dashboard.platform.realtimeBaseUrl}
        scores={dashboard.scores}
        services={dashboard.services}
        initialTab="attacks"
        overview={dashboard.platform}
      />
    </ParticipantShell>
  );
}
