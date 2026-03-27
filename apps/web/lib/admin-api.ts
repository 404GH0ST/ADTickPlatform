import { cache } from "react";

import type {
  AdminAttackFeedPage,
  AdminAttackFeedQuery,
  AdminAuditLogPage,
  AdminAuditLogQuery,
  AdminChallenge,
  AdminChallengeValidationResult,
  AdminControllerAccessStatus,
  AdminCheckerRunPage,
  AdminCheckerRunQuery,
  AdminDeployment,
  AdminDeploymentJob,
  AdminGameScoreRow,
  AdminGameMatchStatus,
  AdminOperationsStatus,
  AdminSchedulerEventPage,
  AdminSchedulerEventQuery,
  AdminGameSchedulerStatus,
  AdminGameStatus,
  AdminGameTickStatus,
  AdminPlayer,
  AdminReconcileResult,
  AdminTeam,
  AdminWireGuardGatewayStatus,
  AdminWireGuardPeer,
} from "@/lib/admin-dashboard-types";

type SuccessEnvelope<T> = {
  status: "success";
  data: T;
};

type ErrorEnvelope = {
  status: "failed" | "forbidden";
  message: string;
};

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
  weight: number;
  service_port?: number;
  service_subnet_octet?: number;
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
  weight: number;
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

function controllerBaseUrl() {
  return trimBaseUrl(
    process.env.AD_PLATFORM_CONTROLLER_URL ?? "http://127.0.0.1:8084",
  );
}

export function organizerApiBaseUrl() {
  return trimBaseUrl(process.env.AD_PLATFORM_PUBLIC_BASE_URL ?? adminBaseUrl());
}

function trimBaseUrl(value: string) {
  return value.trim().replace(/\/+$/, "");
}

const getAdminToken = cache(async () => {
  return process.env.ADMIN_API_TOKEN?.trim() || "dev-admin-token";
});

async function adminFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const token = await getAdminToken();
  const response = await fetch(`${adminBaseUrl()}${path}`, {
    ...init,
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
      ...(init?.headers ?? {}),
    },
    cache: "no-store",
  });

  const payload = (await response.json()) as SuccessEnvelope<T> | ErrorEnvelope;
  if (!response.ok || payload.status !== "success") {
    throw new Error(
      "message" in payload ? payload.message : `request to ${path} failed`,
    );
  }

  return payload.data;
}

async function controllerFetch<T>(
  path: string,
  init?: RequestInit,
): Promise<T> {
  const token = await getAdminToken();
  const response = await fetch(`${controllerBaseUrl()}${path}`, {
    ...init,
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
      ...(init?.headers ?? {}),
    },
    cache: "no-store",
  });

  const payload = (await response.json()) as SuccessEnvelope<T> | ErrorEnvelope;
  if (!response.ok || payload.status !== "success") {
    throw new Error(
      "message" in payload ? payload.message : `request to ${path} failed`,
    );
  }

  return payload.data;
}

async function publicFetch<T>(path: string): Promise<T> {
  const response = await fetch(`${adminBaseUrl()}${path}`, {
    cache: "no-store",
  });

  const payload = (await response.json()) as SuccessEnvelope<T> | ErrorEnvelope;
  if (!response.ok || payload.status !== "success") {
    throw new Error(
      "message" in payload ? payload.message : `request to ${path} failed`,
    );
  }

  return payload.data;
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
  return adminFetch<{ status: "success" }>(`/api/v2/admin/teams/${teamID}`, {
    method: "DELETE",
  });
}

export async function updateAdminTeam(teamID: number, input: UpdateTeamInput) {
  return adminFetch<AdminTeam>(`/api/v2/admin/teams/${teamID}`, {
    method: "PUT",
    body: JSON.stringify(input),
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
  return adminFetch<{ status: "success" }>(
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
  return adminFetch<{ status: "success" }>("/api/v2/admin/wireguard/teardown", {
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
  return adminFetch<{ status: "success" }>("/api/v2/admin/access/teardown", {
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
  return adminFetch<{ status: "success" }>(
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
  return adminFetch<{ status: "success" }>(
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
  try {
    return await controllerFetch<AdminReconcileResult>(
      "/internal/v1/deployments/reconcile",
      {
        method: "POST",
      },
    );
  } catch {
    return adminFetch<AdminReconcileResult>(
      "/api/v2/admin/deployments/reconcile",
      {
        method: "POST",
      },
    );
  }
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

function buildQueryString(query: Record<string, string | number | undefined>) {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value === undefined || value === "" || Number.isNaN(value)) {
      continue;
    }
    params.set(key, String(value));
  }
  const encoded = params.toString();
  return encoded ? `?${encoded}` : "";
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

export async function recomputeAdminGameScoring() {
  return adminFetch<AdminGameScoreRow[]>(
    "/api/v2/admin/game/scoring/recompute",
    {
      method: "POST",
    },
  );
}
