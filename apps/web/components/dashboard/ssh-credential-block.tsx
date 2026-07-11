"use client";

import type { ReactElement } from "react";
import { Wrench } from "lucide-react";

import { Button } from "@/components/ui/button";
import { InfoLine, InfoPanel } from "@/components/ui/info-panel";

export type SSHSessionData = {
  challenge_id: number;
  host: string;
  port: number;
  username: "root";
  password: string;
  password_mode?: "stable";
  connection_hint: string;
};

export function IssuedRootCredentialBlock({
  issuedSession,
}: {
  issuedSession: SSHSessionData;
}): ReactElement {
  return (
    <div className="space-y-4">
      <InfoPanel compact>
        <p className="font-mono text-sm text-foreground">
          {issuedSession.connection_hint}
        </p>
        <InfoLine
          className="mt-2"
          label="password"
          value={issuedSession.password}
          valueClassName="font-mono"
        />
        <div className="mt-3 flex flex-wrap gap-2">
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={() => {
              void navigator.clipboard?.writeText(issuedSession.connection_hint);
            }}
          >
            Copy SSH command
          </Button>
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={() => {
              void navigator.clipboard?.writeText(issuedSession.password);
            }}
          >
            Copy password
          </Button>
        </div>
      </InfoPanel>
      <PatchWorkflowBlock issuedSession={issuedSession} />
    </div>
  );
}

function PatchWorkflowBlock({
  issuedSession,
}: {
  issuedSession: SSHSessionData;
}): ReactElement {
  return (
    <div className="space-y-3 rounded-sm border border-border/70 bg-muted/20 p-4">
      <div className="flex items-center gap-2 text-sm font-medium text-foreground">
        <Wrench className="h-4 w-4" />
        Patch Workflow
      </div>
      <p className="text-sm text-muted-foreground">
        Participant patching happens directly inside the owned service
        container. There is no participant image redeploy path.
      </p>
      <ol className="space-y-2 text-sm text-muted-foreground">
        <li>
          <span className="font-medium text-foreground">1.</span> Use the
          service card source download and identify the file or config you need
          to change.
        </li>
        <li>
          <span className="font-medium text-foreground">2.</span> Connect with{" "}
          <span className="font-mono text-foreground">
            {issuedSession.connection_hint}
          </span>{" "}
          and edit files directly inside the running container.
        </li>
        <li>
          <span className="font-medium text-foreground">3.</span> Use{" "}
          <span className="font-medium text-foreground">Restart</span> after a
          live patch when you want to keep the current filesystem changes.
        </li>
        <li>
          <span className="font-medium text-foreground">4.</span> Use{" "}
          <span className="font-medium text-foreground">Factory Reset</span> to
          discard the current patch state and restore the organizer baseline
          image.
        </li>
      </ol>
    </div>
  );
}
