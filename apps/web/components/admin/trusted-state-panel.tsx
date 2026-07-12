"use client";

import { useCallback, useEffect, useState, type ReactElement } from "react";
import { LoaderCircle, ShieldCheck } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { StatusBanner } from "@/components/ui/status-banner";
import { processApiResponse } from "@/lib/api-utils";
import type {
  AdminControllerAccessStatus,
  AdminWireGuardGatewayStatus,
} from "@/lib/admin-dashboard-types";

type TrustTone = "ok" | "warn" | "bad" | "unknown";

function toneForState(state?: string): TrustTone {
  const value = (state ?? "").toLowerCase();
  if (value === "applied" || value === "ready" || value === "ok") {
    return "ok";
  }
  if (value === "loading" || value === "" || value === "unknown") {
    return "unknown";
  }
  if (value.includes("error") || value.includes("fail") || value.includes("unknown")) {
    return "bad";
  }
  return "warn";
}

function toneClass(tone: TrustTone): string {
  switch (tone) {
    case "ok":
      return "text-positive";
    case "bad":
      return "text-negative";
    case "warn":
      return "text-highlight";
    default:
      return "text-muted-foreground";
  }
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

export function TrustedStatePanel(): ReactElement {
  const [access, setAccess] = useState<AdminControllerAccessStatus | null>(null);
  const [wireguard, setWireguard] =
    useState<AdminWireGuardGatewayStatus | null>(null);
  const [pendingDeployments, setPendingDeployments] = useState<number | null>(
    null,
  );
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);
  const [note, setNote] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setError(null);
    try {
      const [accessRes, wgRes, deployRes] = await Promise.all([
        fetch("/api/admin/access/status", { cache: "no-store" }),
        fetch("/api/admin/wireguard/status", { cache: "no-store" }),
        fetch("/api/admin/deployments", { cache: "no-store" }),
      ]);
      if (accessRes.ok) {
        setAccess(
          await processApiResponse<AdminControllerAccessStatus>(
            accessRes,
            "/api/admin/access/status",
          ),
        );
      }
      if (wgRes.ok) {
        setWireguard(
          await processApiResponse<AdminWireGuardGatewayStatus>(
            wgRes,
            "/api/admin/wireguard/status",
          ),
        );
      }
      if (deployRes.ok) {
        const jobs = await processApiResponse<
          Array<{ status?: string; state?: string }>
        >(deployRes, "/api/admin/deployments");
        setPendingDeployments(
          jobs.filter((job) => {
            const status = (job.status ?? job.state ?? "").toLowerCase();
            return (
              status.includes("pending") ||
              status.includes("queued") ||
              status.includes("running")
            );
          }).length,
        );
      }
    } catch (loadError) {
      setError(
        loadError instanceof Error
          ? loadError.message
          : "trusted state refresh failed",
      );
    }
  }, []);

  useEffect(() => {
    void refresh();
    const timer = window.setInterval(() => {
      void refresh();
    }, 30000);
    return () => window.clearInterval(timer);
  }, [refresh]);

  async function runTrustedReconcile() {
    setPending(true);
    setNote(null);
    setError(null);
    try {
      const response = await fetch("/api/admin/deployments/reconcile", {
        method: "POST",
      });
      if (!response.ok) {
        throw new Error(
          await response
            .json()
            .then((payload: { detail?: string }) => payload.detail)
            .catch(() => "trusted reconcile failed"),
        );
      }
      setNote(
        "Reconcile finished: deployments, SSH access, and WireGuard should match host truth.",
      );
      await refresh();
      window.dispatchEvent(new Event("ad-platform:deployments-reconciled"));
    } catch (reconcileError) {
      setError(
        reconcileError instanceof Error
          ? reconcileError.message
          : "trusted reconcile failed",
      );
    } finally {
      setPending(false);
    }
  }

  const overall =
    toneForState(access?.state) === "ok" &&
    toneForState(wireguard?.state) === "ok" &&
    (pendingDeployments ?? 0) === 0
      ? "Trusted host truth looks healthy."
      : "Host truth is incomplete or degraded — do not assume SSH/access is correct until reconcile succeeds.";

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
          message={overall}
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
            onClick={() => void runTrustedReconcile()}
            disabled={pending}
          >
            {pending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
            Reconcile
          </Button>
          <Button
            type="button"
            variant="outline"
            data-testid="refresh-trusted-status"
            onClick={() => void refresh()}
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
