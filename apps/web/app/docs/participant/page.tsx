import Link from 'next/link';

import { ParticipantShell } from '@/components/dashboard/participant-shell';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { loadDashboardData } from '@/lib/dashboard-data';

const participantSteps = [
  {
    step: '1',
    title: 'Sign In',
    detail: 'Call POST /api/v2/authenticate to get the JWT used for participant actions. Use that token as Bearer auth in your bot or automation.',
  },
  {
    step: '2',
    title: 'Find Targets',
    detail: 'Use GET /api/v2/services to list your own services and enemy endpoints over WireGuard, then download the whitebox source bundle for each challenge you own.',
  },
  {
    step: '3',
    title: 'Unlock SSH',
    detail: 'Recover the unlock proof from your own service, then submit it to open SSH for your team.',
  },
  {
    step: '4',
    title: 'Patch Or Recover',
    detail: 'Open the stable team SSH credential, patch safely, or use restart and factory reset when needed.',
  },
  {
    step: '5',
    title: 'Submit Flags',
    detail: 'Send stolen enemy flags to POST /api/v2/submit. The HTTP request returns a batch result with per-flag status and detail fields.',
  },
] as const;

const participantEndpoints = [
  'POST /api/v2/authenticate',
  'GET /api/v2/challenges',
  'GET /api/v2/challenges/{challenge_id}/source',
  'GET /api/v2/services',
  'GET /api/v2/scoreboard',
  'GET /api/v2/attacks',
  'POST /api/v2/submit',
  'POST /api/v2/services/{challenge_id}/unlock',
  'POST /api/v2/services/{challenge_id}/ssh-session',
  'POST /api/v2/services/{challenge_id}/reset/factory',
  'POST /api/v2/services/{challenge_id}/reset/restart',
] as const;

export default async function ParticipantManualPage() {
  const { platform } = await loadDashboardData();

  return (
    <ParticipantShell
      activePath="/docs/participant"
      overview={platform}
      title="Participant Manual"
      description="Core participant workflow and API entry points."
    >
      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_20rem]">
        <div className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Participant Flow</CardTitle>
              <CardDescription>
                Start here for the normal participant workflow. Use Swagger for the full request and response contract.
              </CardDescription>
            </CardHeader>
            <CardContent className="grid gap-0 md:grid-cols-2">
              {participantSteps.map((item) => (
                <StepRow key={item.step} step={item.step} title={item.title} detail={item.detail} />
              ))}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Participant Endpoints</CardTitle>
              <CardDescription>
                The participant API surface most teams automate against.
              </CardDescription>
            </CardHeader>
            <CardContent className="rounded-lg border p-0">
              <ul className="divide-y">
                {participantEndpoints.map((endpoint) => (
                  <li key={endpoint} className="px-4 py-3 text-sm font-mono">
                    {endpoint}
                  </li>
                ))}
              </ul>
            </CardContent>
          </Card>
        </div>

        <div className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>API Reference</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-col gap-2">
              <Button asChild variant="outline">
                <Link href="/docs/platform-api">Open Swagger</Link>
              </Button>
              <Button asChild variant="outline">
                <Link href="/docs/platform-api-v2.openapi.yaml">Open Raw OpenAPI YAML</Link>
              </Button>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Bearer Auth</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3 text-sm text-muted-foreground">
              <p>Authenticate first with <code>POST /api/v2/authenticate</code>.</p>
              <p>For scripts and bots, send <code>Authorization: Bearer &lt;team JWT&gt;</code> on authenticated routes.</p>
              <p>In Swagger, click <strong>Authorize</strong> and paste only the JWT value. Swagger adds the <code>Bearer</code> prefix automatically.</p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Practical Notes</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3 text-sm text-muted-foreground">
              <p>Service traffic and SSH both use the same service IP.</p>
              <p>Targets are reachable through WireGuard, not host-published ports.</p>
              <p>Factory reset keeps the current match unlock state for the owning team.</p>
              <p>Successful responses are raw JSON bodies. Errors use <code>application/problem+json</code>.</p>
              <p>Swagger is the browser-facing source of truth for request and response shapes.</p>
            </CardContent>
          </Card>
        </div>
      </div>
    </ParticipantShell>
  );
}

function StepRow({ step, title, detail }: { step: string; title: string; detail: string }) {
  return (
    <div className="border-b px-4 py-4 last:border-b-0 md:[&:nth-last-child(2)]:border-b-0 md:[&:nth-child(odd)]:border-r">
      <div className="text-xs font-medium text-muted-foreground">Step {step}</div>
      <div className="mt-1 text-sm font-medium">{title}</div>
      <p className="mt-1 text-sm text-muted-foreground">{detail}</p>
    </div>
  );
}
