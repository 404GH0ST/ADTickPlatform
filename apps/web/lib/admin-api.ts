import { cache } from "react";

import type {
  AdminAttackFeedPage,
  AdminAttackFeedQuery,
  AdminAuditLogPage,
  AdminAuditLogQuery,
  AdminBulkImportResult,
  AdminChallenge,
  AdminChallengeValidationResult,
  AdminControllerAccessStatus,
  AdminCheckerRunPage,
  AdminCheckerRunQuery,
  AdminDeployment,
  AdminDeploymentJob,
  AdminGameScoreRow,
  AdminMatchAnnouncement,
  AdminPlatformSettings,
  AdminPlatformSettingsInput,
  AdminScoringAudit,
  AdminGameMatchStatus,
  AdminOperationsStatus,
  AdminServiceMetricSnapshot,
  AdminSchedulerEventPage,
  AdminSchedulerEventQuery,
  AdminGameSchedulerStatus,
  AdminGameStatus,
  AdminGameTickStatus,
  AdminPlayer,
  AdminReconcileResult,
  AdminScoreboardFreeze,
  AdminTeam,
  AdminWireGuardGatewayStatus,
  AdminWireGuardPeer,
} from "@/lib/admin-dashboard-types";

import {
  authenticatedFetch,
  buildQueryString,
  parseApiError,
} from "./api-utils";

export type CreateTeamInput = {
  name: string;
  contact_email: string;
};

export type CreatePlayerInput = {
  team_id: number;
  display_name: string;
  email: string;
  password: string;
  role: string;
};

export type CreateChallengeInput = {
  name: string;
  baseline_image: string;
  checker_image: string;
  source_bundle_path?: string;
  service_port?: number;
  service_subnet_octet?: number;
  egress_enabled?: boolean;
};

export type UpdateTeamInput = {
  name: string;
  contact_email: string;
};

export type UpdatePlayerInput = {
  display_name: string;
  email: string;
  role: string;
};

export type UpdateChallengeInput = {
  name: string;
  baseline_image: string;
  checker_image: string;
  source_bundle_path?: string;
  egress_enabled?: boolean;
};

export type UpdateGameMatchScheduleInput = {
  scheduled_start_at?: string;
  scheduled_end_at?: string;
};

function adminBaseUrl() {
  return trimBaseUrl(
    process.env.AD_PLATFORM_API_URL ?? "http://127.0.0.1:8080",
  );
}

function gameCoreBaseUrl() {
  return trimBaseUrl(
    process.env.AD_PLATFORM_GAME_CORE_URL ?? "http://127.0.0.1:8081",
  );
}

function submissionServiceBaseUrl() {
  return trimBaseUrl(
    process.env.AD_PLATFORM_SUBMISSION_SERVICE_URL ?? "http://127.0.0.1:8082",
  );
}

function realtimeGatewayBaseUrl() {
  return trimBaseUrl(
    process.env.AD_PLATFORM_REALTIME_URL ?? "http://127.0.0.1:8086",
  );
}

function controllerMetricsBaseUrl() {
  return trimBaseUrl(
    process.env.AD_PLATFORM_CONTROLLER_METRICS_URL ??
      process.env.AD_PLATFORM_CONTROLLER_URL ??
      "http://127.0.0.1:8084",
  );
}

function wireGuardGatewayBaseUrl() {
  return trimBaseUrl(
    process.env.AD_PLATFORM_WIREGUARD_GATEWAY_URL ?? "http://127.0.0.1:8087",
  );
}

export function organizerApiBaseUrl() {
  return trimBaseUrl(process.env.AD_PLATFORM_PUBLIC_BASE_URL ?? adminBaseUrl());
}

function trimBaseUrl(value: string) {
  return value.trim().replace(/\/+$/, "");
}

const getAdminToken = cache(async () => {
  const token = process.env.ADMIN_API_TOKEN?.trim();
  if (!token) {
    throw new Error("ADMIN_API_TOKEN must be set for server-side admin API calls");
  }
  return token;
});

async function adminFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const token = await getAdminToken();
  return authenticatedFetch<T>(adminBaseUrl(), path, token, init);
}

export async function adminProxyFetch(
  path: string,
  init?: RequestInit,
): Promise<Response> {
  const token = await getAdminToken();
  return fetch(`${adminBaseUrl()}${path}`, {
    ...init,
    headers: {
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      "Content-Type": "application/json",
      ...init?.headers,
    },
    cache: "no-store",
  });
}

async function publicFetch<T>(path: string): Promise<T> {
  return authenticatedFetch<T>(adminBaseUrl(), path, null);
}

async function fetchText(baseUrl: string, path: string): Promise<string> {
  const response = await fetch(`${baseUrl}${path}`, {
    cache: "no-store",
  });
  if (!response.ok) {
    throw new Error(`request to ${path} failed`);
  }
  return response.text();
}

type PromLine = {
  name: string;
  labels: Record<string, string>;
  value: number;
};

function parsePrometheus(text: string): PromLine[] {
  const rows: PromLine[] = [];
  for (const rawLine of text.split("\n")) {
    const line = rawLine.trim();
    if (!line || line.startsWith("#")) {
      continue;
    }

    const match = line.match(
      /^([a-zA-Z_:][a-zA-Z0-9_:]*)(?:\{([^}]*)\})?\s+(-?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?)$/,
    );
    if (!match) {
      continue;
    }

    const [, name, rawLabels = "", rawValue] = match;
    const value = Number(rawValue);
    if (Number.isNaN(value)) {
      continue;
    }

    const labels: Record<string, string> = {};
    if (rawLabels) {
      for (const entry of rawLabels.split(/,(?=(?:[^"]*"[^"]*")*[^"]*$)/)) {
        const [rawKey, rawLabelValue] = entry.split("=");
        if (!rawKey || !rawLabelValue) {
          continue;
        }
        labels[rawKey.trim()] = rawLabelValue.trim().replace(/^"|"$/g, "");
      }
    }

    rows.push({ name, labels, value });
  }
  return rows;
}

function metricValue(
  rows: PromLine[],
  metricName: string,
  expectedLabels?: Record<string, string>,
): number | null {
  for (let index = rows.length - 1; index >= 0; index -= 1) {
    const row = rows[index];
    if (row.name !== metricName) {
      continue;
    }
    if (
      expectedLabels &&
      !Object.entries(expectedLabels).every(
        ([key, value]) => row.labels[key] === value,
      )
    ) {
      continue;
    }
    return row.value;
  }
  return null;
}

function metricSum(rows: PromLine[], metricName: string): number | null {
  const matching = rows.filter((row) => row.name === metricName);
  if (matching.length === 0) {
    return null;
  }
  return matching.reduce((sum, row) => sum + row.value, 0);
}

function metricBool(value: number | null): boolean | null {
  if (value === null) {
    return null;
  }
  return value >= 1;
}

export async function getAdminOperationsMetrics() {
  const [
    gameCoreText,
    submissionText,
    controllerText,
    realtimeText,
    wireguardText,
  ] = await Promise.all([
    fetchText(gameCoreBaseUrl(), "/metrics"),
    fetchText(submissionServiceBaseUrl(), "/metrics"),
    fetchText(controllerMetricsBaseUrl(), "/metrics"),
    fetchText(realtimeGatewayBaseUrl(), "/metrics"),
    fetchText(wireGuardGatewayBaseUrl(), "/metrics"),
  ]);

  const gameCore = parsePrometheus(gameCoreText);
  const submission = parsePrometheus(submissionText);
  const controller = parsePrometheus(controllerText);
  const realtime = parsePrometheus(realtimeText);
  const wireguard = parsePrometheus(wireguardText);

  return {
    generated_at: new Date().toISOString(),
    game_core: {
      match_state:
        metricValue(gameCore, "adplatform_game_core_match_state", {
          state: "running",
        }) === 1
          ? "running"
          : metricValue(gameCore, "adplatform_game_core_match_state", {
                state: "finished",
              }) === 1
            ? "finished"
            : metricValue(gameCore, "adplatform_game_core_match_state", {
                  state: "not_started",
                }) === 1
              ? "not_started"
              : "unknown",
      total_ticks: metricValue(gameCore, "adplatform_game_core_total_ticks"),
      checker_runs_total: metricValue(
        gameCore,
        "adplatform_game_core_checker_runs_total",
        { status: "all" },
      ),
      checker_runs_failed: metricValue(
        gameCore,
        "adplatform_game_core_checker_runs_total",
        { status: "failed" },
      ),
      scheduler_running: metricBool(
        metricValue(gameCore, "adplatform_game_core_scheduler_running"),
      ),
    },
    submission_service: {
      submit_requests_total: metricValue(
        submission,
        "adplatform_submission_service_submit_requests_total",
      ),
      submit_failures_total: metricValue(
        submission,
        "adplatform_submission_service_submit_failures_total",
      ),
      attack_feed_requests_total: metricValue(
        submission,
        "adplatform_submission_service_attack_feed_requests_total",
      ),
      verdicts: {
        correct: metricValue(
          submission,
          "adplatform_submission_service_submit_verdicts_total",
          { class: "correct" },
        ),
        duplicate: metricValue(
          submission,
          "adplatform_submission_service_submit_verdicts_total",
          { class: "duplicate" },
        ),
        invalid: metricValue(
          submission,
          "adplatform_submission_service_submit_verdicts_total",
          { class: "invalid" },
        ),
        unknown: metricValue(
          submission,
          "adplatform_submission_service_submit_verdicts_total",
          { class: "unknown" },
        ),
      },
    },
    controller_service: {
      deployment_reconcile_requests: metricValue(
        controller,
        "adplatform_controller_service_operation_requests_total",
        { operation: "deployment_reconcile" },
      ),
      access_reconcile_requests: metricValue(
        controller,
        "adplatform_controller_service_operation_requests_total",
        { operation: "access_reconcile" },
      ),
      service_access_reconcile_requests: metricValue(
        controller,
        "adplatform_controller_service_operation_requests_total",
        { operation: "service_access_reconcile" },
      ),
      ssh_credential_requests: metricValue(
        controller,
        "adplatform_controller_service_operation_requests_total",
        { operation: "ssh_credential" },
      ),
      access_policies_total: metricValue(
        controller,
        "adplatform_controller_service_access_policies_total",
      ),
      access_last_apply_success: metricBool(
        metricValue(
          controller,
          "adplatform_controller_service_access_last_apply_success",
        ),
      ),
    },
    realtime_gateway: {
      last_sync_success: metricBool(
        metricValue(realtime, "adplatform_realtime_gateway_last_sync_success"),
      ),
      sync_errors_total: metricValue(
        realtime,
        "adplatform_realtime_gateway_sync_errors_total",
      ),
      subscribers_total: metricSum(
        realtime,
        "adplatform_realtime_gateway_subscribers",
      ),
      snapshot_bytes_total: metricSum(
        realtime,
        "adplatform_realtime_gateway_snapshot_bytes",
      ),
    },
    wireguard_gateway: {
      reconcile_requests: metricValue(
        wireguard,
        "adplatform_wireguard_gateway_operation_requests_total",
        { operation: "reconcile" },
      ),
      peers_total: metricValue(
        wireguard,
        "adplatform_wireguard_gateway_peer_counts",
        { status: "total" },
      ),
      peers_active: metricValue(
        wireguard,
        "adplatform_wireguard_gateway_peer_counts",
        { status: "active" },
      ),
      peers_revoked: metricValue(
        wireguard,
        "adplatform_wireguard_gateway_peer_counts",
        { status: "revoked" },
      ),
      last_apply_success: metricBool(
        metricValue(
          wireguard,
          "adplatform_wireguard_gateway_last_apply_success",
        ),
      ),
    },
  } satisfies AdminServiceMetricSnapshot;
}

export async function listAdminTeams() {
  return adminFetch<AdminTeam[]>("/api/v2/admin/teams");
}

export async function createAdminTeam(input: CreateTeamInput) {
  return adminFetch<AdminTeam>("/api/v2/admin/teams", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function deleteAdminTeam(teamID: number) {
  return adminFetch<void>(`/api/v2/admin/teams/${teamID}`, {
    method: "DELETE",
  });
}

export async function updateAdminTeam(teamID: number, input: UpdateTeamInput) {
  return adminFetch<AdminTeam>(`/api/v2/admin/teams/${teamID}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deactivateAdminTeam(teamID: number) {
  return adminFetch<AdminTeam>(`/api/v2/admin/teams/${teamID}/deactivate`, {
    method: "POST",
  });
}

export async function reactivateAdminTeam(teamID: number) {
  return adminFetch<AdminTeam>(`/api/v2/admin/teams/${teamID}/reactivate`, {
    method: "POST",
  });
}

export async function listAdminPlayers() {
  return adminFetch<AdminPlayer[]>("/api/v2/admin/players");
}

export async function createAdminPlayer(input: CreatePlayerInput) {
  return adminFetch<AdminPlayer>("/api/v2/admin/players", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function deleteAdminPlayer(playerID: number) {
  return adminFetch<void>(
    `/api/v2/admin/players/${playerID}`,
    {
      method: "DELETE",
    },
  );
}

export async function updateAdminPlayer(
  playerID: number,
  input: UpdatePlayerInput,
) {
  return adminFetch<AdminPlayer>(`/api/v2/admin/players/${playerID}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deactivateAdminPlayer(playerID: number) {
  return adminFetch<AdminPlayer>(
    `/api/v2/admin/players/${playerID}/deactivate`,
    {
      method: "POST",
    },
  );
}

export async function reactivateAdminPlayer(playerID: number) {
  return adminFetch<AdminPlayer>(
    `/api/v2/admin/players/${playerID}/reactivate`,
    {
      method: "POST",
    },
  );
}

export async function getAdminPlayerWireGuard(playerID: number) {
  return adminFetch<AdminWireGuardPeer>(
    `/api/v2/admin/players/${playerID}/wireguard`,
  );
}

export async function rotateAdminPlayerWireGuard(playerID: number) {
  return adminFetch<AdminWireGuardPeer>(
    `/api/v2/admin/players/${playerID}/wireguard/rotate`,
    {
      method: "POST",
    },
  );
}

export async function revokeAdminPlayerWireGuard(playerID: number) {
  return adminFetch<AdminWireGuardPeer>(
    `/api/v2/admin/players/${playerID}/wireguard/revoke`,
    {
      method: "POST",
    },
  );
}

export async function getAdminWireGuardGatewayStatus() {
  return adminFetch<AdminWireGuardGatewayStatus>(
    "/api/v2/admin/wireguard/status",
  );
}

export async function reconcileAdminWireGuardGateway() {
  return adminFetch<AdminWireGuardGatewayStatus>(
    "/api/v2/admin/wireguard/reconcile",
    {
      method: "POST",
    },
  );
}

export async function teardownAdminWireGuardGateway() {
  return adminFetch<void>("/api/v2/admin/wireguard/teardown", {
    method: "POST",
  });
}

export async function getAdminAccessStatus() {
  return adminFetch<AdminControllerAccessStatus>("/api/v2/admin/access/status");
}

export async function reconcileAdminAccess() {
  return adminFetch<AdminControllerAccessStatus>(
    "/api/v2/admin/access/reconcile",
    {
      method: "POST",
    },
  );
}

export async function teardownAdminAccess() {
  return adminFetch<void>("/api/v2/admin/access/teardown", {
    method: "POST",
  });
}

export async function listAdminChallenges() {
  return adminFetch<AdminChallenge[]>("/api/v2/admin/challenges");
}

export async function createAdminChallenge(input: CreateChallengeInput) {
  return adminFetch<AdminChallenge>("/api/v2/admin/challenges", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function deleteAdminChallenge(challengeID: number) {
  return adminFetch<void>(
    `/api/v2/admin/challenges/${challengeID}`,
    {
      method: "DELETE",
    },
  );
}

export async function updateAdminChallenge(
  challengeID: number,
  input: UpdateChallengeInput,
) {
  return adminFetch<AdminChallenge>(`/api/v2/admin/challenges/${challengeID}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deployAdminChallenge(challengeID: number) {
  return adminFetch<AdminDeployment>(
    `/api/v2/admin/challenges/${challengeID}/deploy`,
    {
      method: "POST",
    },
  );
}

export async function validateAdminChallenge(challengeID: number) {
  return adminFetch<AdminChallengeValidationResult>(
    `/api/v2/admin/challenges/${challengeID}/validate`,
    {
      method: "POST",
    },
  );
}

export async function listAdminDeployments() {
  return adminFetch<AdminDeploymentJob[]>("/api/v2/admin/deployments");
}

export async function deleteAdminDeployment(deploymentID: number) {
  return adminFetch<void>(
    `/api/v2/admin/deployments/${deploymentID}`,
    {
      method: "DELETE",
    },
  );
}

export async function listAdminAuditLogs(
  query: AdminAuditLogQuery = { limit: 25 },
) {
  return adminFetch<AdminAuditLogPage>(
    `/api/v2/admin/audit-logs${buildQueryString(query)}`,
  );
}

export async function reconcileAdminDeployments() {
  return adminFetch<AdminReconcileResult>("/api/v2/admin/deployments/reconcile", {
    method: "POST",
  });
}

export async function getAdminGameStatus() {
  return adminFetch<AdminGameStatus>("/api/v2/admin/game/status");
}

export async function getAdminOperationsStatus() {
  return adminFetch<AdminOperationsStatus>("/api/v2/admin/operations/status");
}

export async function getAdminGameMatchStatus() {
  return adminFetch<AdminGameMatchStatus>("/api/v2/admin/game/match");
}

export async function startAdminGameMatch() {
  return adminFetch<AdminGameMatchStatus>("/api/v2/admin/game/match/start", {
    method: "POST",
  });
}

export async function pauseAdminGameMatch() {
  return adminFetch<AdminGameMatchStatus>("/api/v2/admin/game/match/pause", {
    method: "POST",
  });
}

export async function resumeAdminGameMatch() {
  return adminFetch<AdminGameMatchStatus>("/api/v2/admin/game/match/resume", {
    method: "POST",
  });
}

export async function stopAdminGameMatch() {
  return adminFetch<AdminGameMatchStatus>("/api/v2/admin/game/match/stop", {
    method: "POST",
  });
}

export async function updateAdminGameMatchSchedule(
  input: UpdateGameMatchScheduleInput,
) {
  return adminFetch<AdminGameMatchStatus>("/api/v2/admin/game/match/schedule", {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function advanceAdminGameTick() {
  return adminFetch<AdminGameTickStatus>("/api/v2/admin/game/ticks/advance", {
    method: "POST",
  });
}



export async function listAdminCheckerRuns(
  query: AdminCheckerRunQuery = { limit: 25 },
) {
  return adminFetch<AdminCheckerRunPage>(
    `/api/v2/admin/game/checker-runs${buildQueryString(query)}`,
  );
}

export async function listAdminGameAttacks(
  query: AdminAttackFeedQuery = { limit: 12 },
) {
  return publicFetch<AdminAttackFeedPage>(
    `/api/v2/attacks${buildQueryString(query)}`,
  );
}

export async function getAdminGameSchedulerStatus() {
  return adminFetch<AdminGameSchedulerStatus>("/api/v2/admin/game/scheduler");
}

export async function listAdminGameSchedulerEvents(
  query: AdminSchedulerEventQuery = { limit: 12 },
) {
  return adminFetch<AdminSchedulerEventPage>(
    `/api/v2/admin/game/scheduler/events${buildQueryString(query)}`,
  );
}

export async function startAdminGameScheduler() {
  return adminFetch<AdminGameSchedulerStatus>(
    "/api/v2/admin/game/scheduler/start",
    {
      method: "POST",
    },
  );
}

export async function stopAdminGameScheduler() {
  return adminFetch<AdminGameSchedulerStatus>(
    "/api/v2/admin/game/scheduler/stop",
    {
      method: "POST",
    },
  );
}

export async function updateAdminGameScheduler(intervalSeconds: number) {
  return adminFetch<AdminGameSchedulerStatus>(
    "/api/v2/admin/game/scheduler/interval",
    {
      method: "PUT",
      body: JSON.stringify({ interval_seconds: intervalSeconds }),
    },
  );
}

export async function listAdminGameScoreboard() {
  return adminFetch<AdminGameScoreRow[]>("/api/v2/admin/game/scoreboard");
}

export async function getAdminScoreboardFreeze() {
  return adminFetch<AdminScoreboardFreeze>(
    "/api/v2/admin/game/scoreboard/freeze",
  );
}

export async function setAdminScoreboardFreeze(input: {
  freeze_at: string;
  unfreeze_at?: string;
}) {
  return adminFetch<AdminScoreboardFreeze>(
    "/api/v2/admin/game/scoreboard/freeze",
    {
      method: "POST",
      body: JSON.stringify(input),
    },
  );
}

export async function clearAdminScoreboardFreeze() {
  return adminFetch<AdminScoreboardFreeze>(
    "/api/v2/admin/game/scoreboard/unfreeze",
    {
      method: "POST",
    },
  );
}

export async function recomputeAdminGameScoring() {
  return adminFetch<AdminGameScoreRow[]>(
    "/api/v2/admin/game/scoring/recompute",
    {
      method: "POST",
    },
  );
}

export async function auditAdminGameScoring() {
  return adminFetch<AdminScoringAudit>("/api/v2/admin/game/scoring/audit");
}

export async function getAdminPlatformSettings() {
  return adminFetch<AdminPlatformSettings>(
    "/api/v2/admin/platform/settings",
  );
}

export async function updateAdminPlatformSettings(
  input: AdminPlatformSettingsInput,
) {
  return adminFetch<AdminPlatformSettings>(
    "/api/v2/admin/platform/settings",
    {
      method: "PUT",
      body: JSON.stringify(input),
    },
  );
}

export async function reloadAdminFlagFormat() {
  return adminFetch<AdminPlatformSettings>(
    "/api/v2/admin/platform/settings/reload",
    {
      method: "POST",
    },
  );
}

export async function listAdminAnnouncements() {
  return adminFetch<AdminMatchAnnouncement[]>("/api/v2/admin/announcements");
}

export async function createAdminAnnouncement(body: string) {
  return adminFetch<AdminMatchAnnouncement>("/api/v2/admin/announcements", {
    method: "POST",
    body: JSON.stringify({ body }),
  });
}

export async function deleteAdminAnnouncement(announcementID: number) {
  return adminFetch<{ deleted: boolean }>(
    `/api/v2/admin/announcements/${announcementID}`,
    { method: "DELETE" },
  );
}

export async function bulkImportAdminTeams(teams: Array<{
  name: string;
  contact_email: string;
  players?: Array<{
    display_name: string;
    email: string;
    password: string;
    role?: string;
  }>;
}>): Promise<{ status: number; body: AdminBulkImportResult }> {
  const token = await getAdminToken();
  const response = await fetch(`${adminBaseUrl()}/api/v2/admin/import/teams`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ teams }),
    cache: "no-store",
  });
  if (response.status !== 200 && response.status !== 422) {
    throw new Error(await parseApiError(response, "/api/v2/admin/import/teams"));
  }
  const body = (await response.json()) as AdminBulkImportResult;
  return { status: response.status, body };
}
