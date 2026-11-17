import { ApiDocs } from '@/components/docs/api-docs';
import { ParticipantShell } from '@/components/dashboard/participant-shell';
import { SwaggerUIBrowser } from '@/components/docs/swagger-ui-browser';
import { loadDashboardData } from '@/lib/dashboard-data';

export default async function ParticipantSwaggerPage() {
  const { platform } = await loadDashboardData();

  return (
    <ParticipantShell
      activePath="/docs/platform-api"
      overview={platform}
      title="Participant API"
    >
      <ApiDocs
        specURL="/docs/platform-api-v2.openapi.yaml"
        explorer={<SwaggerUIBrowser specURL="/docs/platform-api-v2.openapi.yaml" />}
      />
    </ParticipantShell>
  );
}
