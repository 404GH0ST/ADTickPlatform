"use client";

import { useCallback, useEffect, useState } from "react";

import { processApiResponse } from "@/lib/api-utils";
import type {
  AdminControllerAccessStatus,
  AdminWireGuardGatewayStatus,
} from "@/lib/admin-dashboard-types";

export type TrustTone = "ok" | "warn" | "bad" | "unknown";

export function toneForState(state?: string): TrustTone {
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

export function toneClass(tone: TrustTone): string {
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

/**
 * Host-truth status (deployments, SSH access, WireGuard) with a 30 s poll and
 * the one-click reconcile. Feeds the OrganizerTrustedState slot.
 */
export type TrustedState = {
  access: AdminControllerAccessStatus | null;
  wireguard: AdminWireGuardGatewayStatus | null;
  /** Active rollout jobs, or null while unknown. */
  pendingDeployments: number | null;
  /** True when access and WireGuard are healthy and nothing is deploying. */
  healthy: boolean;
  /** One-line summary of the above. */
  summary: string;
  pending: boolean;
  error: string | null;
  note: string | null;
  onReconcile: () => void;
  onRefresh: () => void;
};

export function useTrustedState(): TrustedState {
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

  const healthy =
    toneForState(access?.state) === "ok" &&
    toneForState(wireguard?.state) === "ok" &&
    (pendingDeployments ?? 0) === 0;

  return {
    access,
    wireguard,
    pendingDeployments,
    healthy,
    summary: healthy
      ? "Trusted host truth looks healthy."
      : "Host truth is incomplete or degraded — do not assume SSH/access is correct until reconcile succeeds.",
    pending,
    error,
    note,
    onReconcile: () => void runTrustedReconcile(),
    onRefresh: () => void refresh(),
  };
}
