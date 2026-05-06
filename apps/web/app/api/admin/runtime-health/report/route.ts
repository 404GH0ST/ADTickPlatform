import { NextResponse } from "next/server";

import type {
  AdminControllerAccessStatus,
  AdminDeploymentJob,
  AdminGameStatus,
  AdminOperationsStatus,
  AdminRuntimeEvidenceFailure,
  AdminRuntimeEvidenceReport,
  AdminServiceMetricSnapshot,
  AdminWireGuardGatewayStatus,
} from "@/lib/admin-dashboard-types";
import {
  getAdminAccessStatus,
  getAdminGameStatus,
  getAdminOperationsMetrics,
  getAdminOperationsStatus,
  getAdminWireGuardGatewayStatus,
  listAdminDeployments,
} from "@/lib/admin-api";

type SettledValue<T> = PromiseSettledResult<T>;

function readSettled<T>(
  section: string,
  result: SettledValue<T>,
  failures: AdminRuntimeEvidenceFailure[],
): T | null {
  if (result.status === "fulfilled") {
    return result.value;
  }

  failures.push({
    section,
    message:
      result.reason instanceof Error
        ? result.reason.message
        : `${section} fetch failed`,
  });
  return null;
}

function buildSummary(report: Omit<AdminRuntimeEvidenceReport, "summary">): string {
  const deployments = report.deployments ?? [];
  const pendingDeployments = deployments.filter(
    (deployment) =>
      !["completed", "failed", "superseded"].includes(deployment.status),
  ).length;
  const failedDeployments = deployments.filter(
    (deployment) =>
      deployment.status === "failed" || deployment.failed_team_count > 0,
  ).length;
  const operationsAlerts = report.operations_status?.alerts ?? [];
  const criticalAlerts = operationsAlerts.filter(
    (alert) => alert.severity === "critical",
  ).length;
  const warningAlerts = operationsAlerts.length - criticalAlerts;
  const failedSections = report.failures.map((failure) => failure.section);

  return [
    `Runtime summary generated ${report.generated_at}`,
    `Deployments: ${pendingDeployments} pending, ${failedDeployments} failed`,
    `Access: ${report.access_status?.state ?? "unavailable"}${report.access_status?.revision ? ` (${report.access_status.revision})` : ""}`,
    `WireGuard: ${report.wireguard_status?.state ?? "unavailable"}${report.wireguard_status?.revision ? ` (${report.wireguard_status.revision})` : ""}`,
    `Operations alerts: ${criticalAlerts} critical, ${warningAlerts} warning`,
    report.failures.length === 0
      ? "Report completeness: all sections loaded"
      : `Report completeness: partial, missing ${failedSections.join(", ")}`,
  ].join("\n");
}

export async function GET() {
  const [
    deploymentsResult,
    accessStatusResult,
    wireguardStatusResult,
    operationsStatusResult,
    serviceMetricsResult,
    gameStatusResult,
  ] = await Promise.allSettled([
    listAdminDeployments(),
    getAdminAccessStatus(),
    getAdminWireGuardGatewayStatus(),
    getAdminOperationsStatus(),
    getAdminOperationsMetrics(),
    getAdminGameStatus(),
  ]);

  const failures: AdminRuntimeEvidenceFailure[] = [];

  const partialReport = {
    generated_at: new Date().toISOString(),
    failures,
    deployments: readSettled<AdminDeploymentJob[]>(
      "deployments",
      deploymentsResult,
      failures,
    ),
    access_status: readSettled<AdminControllerAccessStatus>(
      "access_status",
      accessStatusResult,
      failures,
    ),
    wireguard_status: readSettled<AdminWireGuardGatewayStatus>(
      "wireguard_status",
      wireguardStatusResult,
      failures,
    ),
    operations_status: readSettled<AdminOperationsStatus>(
      "operations_status",
      operationsStatusResult,
      failures,
    ),
    service_metrics: readSettled<AdminServiceMetricSnapshot>(
      "service_metrics",
      serviceMetricsResult,
      failures,
    ),
    game_status: readSettled<AdminGameStatus>(
      "game_status",
      gameStatusResult,
      failures,
    ),
  };

  const report: AdminRuntimeEvidenceReport = {
    ...partialReport,
    summary: buildSummary(partialReport),
  };

  const timestamp = report.generated_at.replace(/[:.]/g, "-");
  return NextResponse.json(report, {
    headers: {
      "Content-Disposition": `attachment; filename=\"runtime-health-${timestamp}.json\"`,
    },
  });
}
