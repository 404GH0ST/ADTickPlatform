import { ControlCenter } from "@/components/dashboard/control-center";
import { ParticipantShell } from "@/components/dashboard/participant-shell";
import { loadDashboardData } from "@/lib/dashboard-data";

export const dynamic = "force-dynamic";

export default async function ScoreboardPage() {
  const dashboard = await loadDashboardData();

  return (
    <ParticipantShell
      activePath="/scoreboard"
      overview={dashboard.platform}
      title="Scoreboard"
      description="Live ranking table with attack, defense, SLA, and total."
    >
      <ControlCenter
        attackPage={dashboard.attackPage}
        realtimeBaseUrl={dashboard.platform.realtimeBaseUrl}
        scores={dashboard.scores}
        services={dashboard.services}
        initialTab="scoreboard"
        currentTeamName={dashboard.platform.teamName}
      />
    </ParticipantShell>
  );
}
