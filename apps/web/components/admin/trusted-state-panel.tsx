"use client";

import type { ReactElement } from "react";
import { LoaderCircle, ShieldCheck } from "lucide-react";

import { toneClass, toneForState, type TrustedState, useTrustedState } from "@/components/admin/use-trusted-state";
import { ThemeSlot } from "@/components/theme/theme-slot";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { StatusBanner } from "@/components/ui/status-banner";

/** Trusted runtime state. Logic in useTrustedState; themeable via OrganizerTrustedState. */
export function TrustedStatePanel(): ReactElement {
  return <ThemeSlot name="OrganizerTrustedState" fallback={DefaultTrustedStateView} props={useTrustedState()} />;
}

function TruthRow({
  label,
  state,
  detail,
}: {
  label: string;
  state?: string;
  detail?: string;
}) {
  const tone = toneForState(state);
  return (
    <div className="rounded-sm border border-border/60 px-3 py-2">
      <dt className="text-xs font-medium text-muted-foreground">{label}</dt>
      <dd className={`mt-1 text-sm font-semibold ${toneClass(tone)}`}>
        {state?.trim() || "unknown"}
      </dd>
      {detail ? (
        <p className="mt-1 text-xs text-muted-foreground">{detail}</p>
      ) : null}
    </div>
  );
}

export function DefaultTrustedStateView({
  access,
  wireguard,
  pendingDeployments,
  summary,
  pending,
  error,
  note,
  onReconcile,
  onRefresh,
}: TrustedState): ReactElement {
  return (
    <Card data-testid="trusted-state-panel">
      <CardHeader>
        <div className="flex items-center gap-2">
          <ShieldCheck className="h-4 w-4 text-primary" />
          <CardTitle className="text-base">Trusted runtime state</CardTitle>
        </div>
        <CardDescription>
          Host truth after deploy: queued instances, SSH access policy, and
          WireGuard peers. One action advances all of them.
        </CardDescription>
      </CardHeader>
      <CardContent className="grid gap-3">
        <StatusBanner
          message={summary}
          variant={
            toneForState(access?.state) === "ok" &&
            toneForState(wireguard?.state) === "ok"
              ? "success"
              : "warning"
          }
        />
        <div className="grid gap-2 sm:grid-cols-3">
          <TruthRow
            label="Pending deployments"
            state={
              pendingDeployments === null
                ? "unknown"
                : pendingDeployments === 0
                  ? "ready"
                  : `${pendingDeployments} active`
            }
            detail="Queued or in-flight rollout jobs"
          />
          <TruthRow
            label="Controller access"
            state={access?.state}
            detail={
              access
                ? `SSH open ${access.ssh_open_services} / locked ${access.ssh_locked_services}`
                : undefined
            }
          />
          <TruthRow
            label="WireGuard gateway"
            state={wireguard?.state}
            detail={
              wireguard
                ? `Active peers ${wireguard.peers_active} / total ${wireguard.peers_total}`
                : undefined
            }
          />
        </div>
        {access?.last_error ? (
          <StatusBanner message={`Access: ${access.last_error}`} variant="error" />
        ) : null}
        {wireguard?.last_error ? (
          <StatusBanner
            message={`WireGuard: ${wireguard.last_error}`}
            variant="error"
          />
        ) : null}
        {error ? <StatusBanner message={error} variant="error" /> : null}
        {note ? <StatusBanner message={note} variant="success" /> : null}
        <div className="flex flex-wrap items-center gap-2">
          <Button
            type="button"
            data-testid="run-trusted-reconcile"
            onClick={onReconcile}
            disabled={pending}
          >
            {pending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
            Reconcile
          </Button>
          <Button
            type="button"
            variant="outline"
            data-testid="refresh-trusted-status"
            onClick={onRefresh}
            disabled={pending}
          >
            Refresh
          </Button>
          <p className="text-xs text-muted-foreground sm:ml-1">
            Use after Deploy, or when access / WireGuard looks stale.
          </p>
        </div>
      </CardContent>
    </Card>
  );
}
