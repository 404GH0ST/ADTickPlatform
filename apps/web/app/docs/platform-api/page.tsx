import Link from 'next/link';

import { ParticipantShell } from '@/components/dashboard/participant-shell';
import { SwaggerUIBrowser } from '@/components/docs/swagger-ui-browser';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { loadDashboardData } from '@/lib/dashboard-data';

export default async function ParticipantSwaggerPage() {
  const { platform } = await loadDashboardData();

  return (
    <ParticipantShell
      activePath="/docs/platform-api"
      overview={platform}
      title="Participant API"
      description="Interactive Swagger reference for the participant API."
    >
      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_20rem]">
        <div className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Swagger Explorer</CardTitle>
              <CardDescription>
                Interactive view of the participant API using the same OpenAPI YAML served by this app.
              </CardDescription>
            </CardHeader>
            <CardContent className="flex flex-wrap gap-2">
              <Button asChild variant="outline">
                <Link href="/docs/platform-api-v2.openapi.yaml">Open Raw YAML</Link>
              </Button>
              <Button asChild variant="outline">
                <Link href="/docs/participant">Open Manual</Link>
              </Button>
            </CardContent>
          </Card>

          <SwaggerUIBrowser specURL="/docs/platform-api-v2.openapi.yaml" />
        </div>

        <div className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Bearer Auth</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3 text-sm text-muted-foreground">
              <p>Call <code>POST /api/v2/authenticate</code> first to get a participant JWT.</p>
              <p>In Swagger, click <strong>Authorize</strong> and paste only the JWT token value. Swagger adds the <code>Bearer</code> prefix automatically for this security scheme.</p>
              <p>Only authenticated participant routes need that token. Public discovery routes such as challenge catalog, scoreboard, game status, and attack feed intentionally do not show Bearer auth.</p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Scope</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3 text-sm text-muted-foreground">
              <p>This page covers the participant API only.</p>
              <p>Use it for target discovery, submission, unlock, SSH session, and reset flows.</p>
              <p>The OpenAPI source is the same YAML served at <code>/docs/platform-api-v2.openapi.yaml</code>.</p>
              <p>Swagger UI assets load from a CDN at runtime. If that CDN is blocked, use the raw YAML route instead.</p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Practical Use</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3 text-sm text-muted-foreground">
              <p>Use this page to inspect request bodies, raw success payloads, Problem Details errors, and authentication requirements.</p>
              <p>Use the raw YAML route for code generation in Python, Go, Rust, or TypeScript bots.</p>
            </CardContent>
          </Card>
        </div>
      </div>
    </ParticipantShell>
  );
}
