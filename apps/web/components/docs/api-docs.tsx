"use client";

import Link from "next/link";
import type { ReactElement, ReactNode } from "react";

import { ThemeSlot } from "@/components/theme/theme-slot";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export type ApiDocsViewProps = {
  /** URL of the OpenAPI YAML. */
  specURL: string;
  /** Rendered interactive Swagger UI for specURL; place it anywhere. */
  explorer: ReactNode;
};

/** Participant API docs body (/docs/platform-api). Themeable via the ApiDocs slot. */
export function ApiDocs(props: ApiDocsViewProps): ReactElement {
  return <ThemeSlot name="ApiDocs" fallback={DefaultApiDocsView} props={props} />;
}

export function DefaultApiDocsView({ specURL, explorer }: ApiDocsViewProps): ReactElement {
  return (
    <div className="grid grid-cols-1 gap-4 xl:grid-cols-[minmax(0,1fr)_20rem]">
      <div className="min-w-0 space-y-4">
        <Card>
          <CardHeader>
            <CardTitle as="h2">Swagger Explorer</CardTitle>
            <CardDescription>
              Interactive view of the participant API using the same OpenAPI YAML served by this app.
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-wrap gap-2">
            <Button asChild variant="outline">
              <Link href={specURL}>Open Raw YAML</Link>
            </Button>
            <Button asChild variant="outline">
              <Link href="/docs/participant">Open Manual</Link>
            </Button>
          </CardContent>
        </Card>

        {explorer}
      </div>

      <div className="min-w-0 space-y-4 wrap-anywhere">
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
            <p>Use it for target discovery, submission, SSH session, and reset flows.</p>
            <p>The OpenAPI source is the same YAML served at <code>{specURL}</code>.</p>
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
  );
}
