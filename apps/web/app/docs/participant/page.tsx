import { ParticipantShell } from '@/components/dashboard/participant-shell';
import { PARTICIPANT_MANUAL_STEPS, ParticipantManual } from '@/components/docs/participant-manual';
import { loadDashboardData } from '@/lib/dashboard-data';

export default async function ParticipantManualPage() {
  const { platform } = await loadDashboardData();

  return (
    <ParticipantShell
      activePath="/docs/participant"
      overview={platform}
      title="Participant Manual"
    >
      <ParticipantManual
        steps={PARTICIPANT_MANUAL_STEPS}
        apiExplorerHref="/docs/platform-api"
        openApiHref="/docs/platform-api-v2.openapi.yaml"
      />
    </ParticipantShell>
  );
}
