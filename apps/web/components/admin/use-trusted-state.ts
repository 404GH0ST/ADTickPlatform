"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import {
  browserRequestErrorMessage,
  processApiResponse,
} from "@/lib/api-utils";
import type {
  AdminControllerAccessStatus,
  AdminDeploymentReconcileJob,
  AdminWireGuardGatewayStatus,
} from "@/lib/admin-dashboard-types";

export type TrustTone = "ok" | "warn" | "bad" | "unknown";

const reconcileJobPath = "/api/admin/deployments/reconcile/job";

function reconcileProgress(job: AdminDeploymentReconcileJob | null): string | null {
  if (job?.status !== "running") return null;
  switch (job.phase) {
    case "runtime":
      return `Team instances checked or started: ${job.ensured_instances}/${job.total_instances}.`;
    case "access":
      return "Applying host access rules…";
    case "wireguard":
      return "Verifying WireGuard access…";
    default:
      return "Starting reconciliation…";
  }
}

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
  progress: string | null;
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
  const [starting, setStarting] = useState(false);
  const [job, setJob] = useState<AdminDeploymentReconcileJob | null>(null);
  const lastJob = useRef<AdminDeploymentReconcileJob | null>(null);
  const [jobError, setJobError] = useState<string | null>(null);

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
        browserRequestErrorMessage(loadError, "trusted state refresh failed"),
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

  const applyJob = useCallback((next: AdminDeploymentReconcileJob) => {
    const previous = lastJob.current;
    lastJob.current = next;
    setJob(next);
    if (previous?.status === "running" && next.status === "idle") {
      setJobError("The controller restarted during reconciliation. Check the deployment status before retrying.");
    } else if (next.status === "running" || previous?.status === "running") {
      setJobError(null);
    }
    if (next.status === "completed" && (previous?.status !== "completed" || previous.id !== next.id)) {
      void refresh();
      window.dispatchEvent(new Event("ad-platform:deployments-reconciled"));
    }
  }, [refresh]);

  useEffect(() => {
    if (starting) return;
    const controller = new AbortController();
    let timer: number;
    async function poll() {
      let delay = 5000;
      try {
        const response = await fetch(reconcileJobPath, { cache: "no-store", signal: controller.signal });
        const latest = await processApiResponse<AdminDeploymentReconcileJob>(response, reconcileJobPath);
        if (controller.signal.aborted) return;
        applyJob(latest);
        delay = latest.status === "running" ? 2000 : 30000;
      } catch (pollError) {
        if (!controller.signal.aborted && lastJob.current?.status === "running") {
          setJobError(browserRequestErrorMessage(pollError, "Could not read reconciliation progress. Retrying."));
        }
      } finally {
        if (!controller.signal.aborted) timer = window.setTimeout(poll, delay);
      }
    }
    void poll();
    return () => {
      controller.abort();
      window.clearTimeout(timer);
    };
  }, [applyJob, starting]);

  async function runTrustedReconcile() {
    setStarting(true);
    setJobError(null);
    try {
      const response = await fetch(reconcileJobPath, { method: "POST" });
      applyJob(await processApiResponse<AdminDeploymentReconcileJob>(response, reconcileJobPath));
    } catch (reconcileError) {
      setJobError(
        browserRequestErrorMessage(reconcileError, "trusted reconcile failed"),
      );
    } finally {
      setStarting(false);
    }
  }

  const pending = starting || job?.status === "running";
  const healthy =
    !pending &&
    toneForState(access?.state) === "ok" &&
    toneForState(wireguard?.state) === "ok" &&
    (pendingDeployments ?? 0) === 0;

  return {
    access,
    wireguard,
    pendingDeployments,
    healthy,
    summary: healthy
      ? "Host state looks healthy: deployments, SSH access, and WireGuard all match."
      : "Host truth is incomplete or degraded — do not assume SSH/access is correct until reconcile succeeds.",
    pending,
    progress: starting ? "Starting reconciliation…" : reconcileProgress(job),
    error: jobError ?? (job?.status === "failed" ? job.error || "Deployment reconciliation failed." : error),
    note: job?.status === "completed" && !starting
      ? "Reconcile finished: deployments, SSH access, and WireGuard should match host truth."
      : null,
    onReconcile: () => void runTrustedReconcile(),
    onRefresh: () => void refresh(),
  };
}
