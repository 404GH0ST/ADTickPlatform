"use client";

import Link from "next/link";
import type { ReactElement } from "react";

import { ThemeSlot } from "@/components/theme/theme-slot";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export type ManualStep = { step: string; title: string; detail: string };

export type ParticipantManualViewProps = {
  steps: readonly ManualStep[];
  /** Swagger UI page and the raw OpenAPI document. */
  apiExplorerHref: string;
  openApiHref: string;
};

export const PARTICIPANT_MANUAL_STEPS: readonly ManualStep[] = [
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
    detail: 'Submit a correct flag from another team to automatically open SSH for your team on that challenge.',
  },
  {
    step: '4',
    title: 'Patch Or Recover',
    detail: 'Open the stable team SSH credential, patch live files inside the owned container, restart to keep changes, or factory reset to discard them.',
  },
  {
    step: '5',
    title: 'Submit Flags',
    detail: 'Send stolen enemy flags to POST /api/v2/submit. The HTTP request returns a batch result with per-flag status and detail fields.',
  },
];

/** Participant manual body (/docs/participant). Themeable via the ParticipantManual slot. */
export function ParticipantManual(props: ParticipantManualViewProps): ReactElement {
  return <ThemeSlot name="ParticipantManual" fallback={DefaultParticipantManualView} props={props} />;
}

export function DefaultParticipantManualView({ steps, apiExplorerHref, openApiHref }: ParticipantManualViewProps): ReactElement {
  return (
    <div className="grid grid-cols-1 gap-4 xl:grid-cols-[minmax(0,1fr)_20rem]">
      <div className="min-w-0 space-y-4 [&_code]:wrap-anywhere">
        <Card>
          <CardHeader>
            <CardTitle as="h2">Participant Flow</CardTitle>
            <CardDescription>
              Start here for the normal participant workflow. Use Swagger for the full request and response contract.
            </CardDescription>
          </CardHeader>
          <CardContent className="grid grid-cols-1 gap-0 md:grid-cols-2">
            {steps.map((item) => (
              <StepRow key={item.step} step={item.step} title={item.title} detail={item.detail} />
            ))}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle as="h2">Direct Patch Workflow</CardTitle>
            <CardDescription>
              Participant patching uses direct container access after unlock. There is no participant image redeploy endpoint.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3 text-sm text-muted-foreground">
            <p>Download the whitebox source bundle, submit a correct enemy flag on the challenge, and open SSH Access to view the stable team credential.</p>
            <p>Patch files directly inside the running service container, then use <code>POST /api/v2/services/{'{challenge_id}'}/reset/restart</code> when you want to keep the current filesystem changes.</p>
            <p>Use <code>POST /api/v2/services/{'{challenge_id}'}/reset/factory</code> only when you want to throw away the current patch state and return to the organizer baseline image. If an issued flag is still outstanding, the reset forfeits its tick SLA and incurs a severe defense penalty.</p>
          </CardContent>
        </Card>
      </div>

      <div className="min-w-0 space-y-4 [&_code]:wrap-anywhere">
        <Card>
          <CardHeader>
            <CardTitle as="h2">API Reference</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-2">
            <Button asChild variant="outline">
              <Link href={apiExplorerHref}>Open Swagger</Link>
            </Button>
            <Button asChild variant="outline">
              <Link href={openApiHref}>Open Raw OpenAPI YAML</Link>
            </Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle as="h2">Bearer Auth</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 text-sm text-muted-foreground">
            <p>Authenticate first with <code>POST /api/v2/authenticate</code>.</p>
            <p>For scripts and bots, send <code>Authorization: Bearer &lt;team JWT&gt;</code> on authenticated routes.</p>
            <p>In Swagger, click <strong>Authorize</strong> and paste only the JWT value. Swagger adds the <code>Bearer</code> prefix automatically.</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle as="h2">Practical Notes</CardTitle>
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
  );
}

function StepRow({ step, title, detail }: { step: string; title: string; detail: string }) {
  return (
    <div className="border-b px-4 py-4 last:border-b-0 md:[&:nth-child(odd)]:border-r md:[&:nth-last-child(2):nth-child(odd)]:border-b-0 md:[&:last-child:nth-child(odd)]:col-span-2 md:[&:last-child:nth-child(odd)]:border-r-0">
      <div className="text-xs font-medium text-muted-foreground">Step {step}</div>
      <div className="mt-1 text-sm font-medium">{title}</div>
      <p className="mt-1 wrap-anywhere text-sm text-muted-foreground">{detail}</p>
    </div>
  );
}
