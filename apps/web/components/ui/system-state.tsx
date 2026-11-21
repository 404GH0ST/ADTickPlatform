import type { ReactNode } from "react";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { WorkspaceFrame } from "@/components/ui/workspace-frame";

/**
 * Full-page state for when the requested page can't be shown (not found,
 * crashed, platform unreachable). Renders inside the root layout, so it follows
 * the active theme and color mode. Always give it a way forward in `actions`.
 */
export function SystemState({
  title,
  description,
  actions,
}: {
  title: string;
  description: ReactNode;
  actions: ReactNode;
}) {
  return (
    <main
      data-testid="system-state"
      className="workspace-canvas flex items-center justify-center text-foreground"
    >
      <WorkspaceFrame className="min-h-0 w-full max-w-md" bodyClassName="gap-3">
        <Card className="w-full border-border bg-background shadow-none">
          <CardHeader>
            <CardTitle as="h1" tabIndex={-1} data-focus-target className="outline-none">
              {title}
            </CardTitle>
            <CardDescription>{description}</CardDescription>
          </CardHeader>
          <CardContent className="flex flex-wrap gap-2">{actions}</CardContent>
        </Card>
      </WorkspaceFrame>
    </main>
  );
}
