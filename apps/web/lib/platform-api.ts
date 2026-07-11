import { cache } from "react";
import { cookies } from "next/headers";

import { participantSessionCookieName } from "@/lib/participant-session-cookie";
import { validateParticipantSessionWithAPI } from "@/lib/session-validation";
import {
  decodeJWTClaims,
  type ParticipantSessionClaims,
  verifyParticipantSessionToken,
} from "@/lib/session-token";

function trimBaseUrl(value: string) {
  return value.trim().replace(/\/+$/, "");
}

type Challenge = {
  id: number;
  name: string;
  has_source_download: boolean;
  maintenance?: boolean;
};

type ScoreRow = {
  rank: number;
  team: string;
  attack: number;
  defense: number;
  sla: number;
  total: number;
  delta: string;
  services?: Array<{
    challenge_id: number;
    service: string;
    attack: number;
    defense: number;
    sla: number;
    total: number;
  }>;
};

type AttackEvent = {
  id: string;
  attacker: string;
  victim: string;
  service: string;
  tick: number;
  verdict: string;
};

type AttackFeedQuery = {
  limit?: number;
  offset?: number;
  attacker?: string;
  victim?: string;
  service?: string;
  tick_from?: number;
  tick_to?: number;
};

type AttackFeedPage = {
  items: AttackEvent[];
  limit: number;
  offset: number;
  total_count: number;
  has_prev: boolean;
  has_next: boolean;
};

export type GameMatchStatus = {
  state: string;
  started_at?: string;
  ended_at?: string;
  scheduled_start_at?: string;
  scheduled_end_at?: string;
  schedule_configured?: boolean;
  accepting_submissions: boolean;
};

export type GameSchedulerStatus = {
  state: string;
  interval_seconds: number;
  last_run_at?: string;
  next_run_at?: string;
  last_tick_id?: number;
  last_error?: string;
};

export type GameTickStatus = {
  id: number;
  status: string;
};

export type GameStatus = {
  match?: GameMatchStatus;
  current_tick?: GameTickStatus;
  scheduler?: GameSchedulerStatus;
  total_ticks: number;
};

export type TeamServiceState = {
  challenge_id: number;
  team_id: number;
  name: string;
  endpoint: string;
  status: "stable" | "warming" | "degraded";
  checker: "passing" | "warning";
  unlocked: boolean;
  ssh_hint: string;
  last_event: string;
  reset_cooldown: string;
  maintenance?: boolean;
  sla_status?: "ok" | "recovering" | "flag_not_found" | "faulty" | "down" | "unknown";
  sla_phase?: string;
  sla_tick_id?: number;
  sla_message?: string;
};

export type ServicesResponseData = Record<string, Record<string, string[]>>;

import {
  authenticatedFetch,
  buildQueryString,
  parseApiError,
  PlatformAPIError,
} from "./api-utils";

export { PlatformAPIError } from "./api-utils";

type UnlockResponseData = {
  challenge_id: number;
  team_id: number;
  unlocked: boolean;
};

type SSHSessionResponseData = {
  challenge_id: number;
  host: string;
  port: number;
  username: "root";
  password: string;
  password_mode?: "stable";
  connection_hint: string;
};

type FactoryResetResponseData = {
  challenge_id: number;
  team_id: number;
  action: "factory_reset";
  unlock_preserved: true;
};

type RestartResponseData = {
  challenge_id: number;
  team_id: number;
  action: "restart";
};

type ParticipantSession = {
  authenticated: boolean;
  token?: string;
  teamID?: number;
  playerID?: number;
  teamName?: string;
  teamContactEmail?: string;
  displayName?: string;
  email?: string;
  role?: string;
  source: "cookie" | "env" | "none";
  reason?: "deactivated";
};

type AuthenticatedSessionSource = Exclude<ParticipantSession["source"], "none">;

function apiBaseUrl() {
  return trimBaseUrl(
    process.env.AD_PLATFORM_API_URL ?? "http://127.0.0.1:8080",
  );
}

export function participantApiBaseUrl() {
  return trimBaseUrl(process.env.AD_PLATFORM_PUBLIC_BASE_URL ?? apiBaseUrl());
}

export function participantRealtimeBaseUrl() {
  return "/api/platform/realtime";
}

function claimNumber(
  claims: ParticipantSessionClaims | null,
  key: "team_id" | "player_id",
): number | undefined {
  const value = claims?.[key];
  return typeof value === "number" ? value : undefined;
}

function claimString(
  claims: ParticipantSessionClaims | null,
  key: "team_name" | "team_contact_email" | "display_name" | "email" | "role",
): string | undefined {
  const value = claims?.[key];
  return typeof value === "string" ? value : undefined;
}

function authenticatedSession(
  token: string,
  source: AuthenticatedSessionSource,
  claims: ParticipantSessionClaims | null = decodeJWTClaims(token),
): ParticipantSession {
  return {
    authenticated: true,
    token,
    teamID: claimNumber(claims, "team_id"),
    playerID: claimNumber(claims, "player_id"),
    teamName: claimString(claims, "team_name"),
    teamContactEmail: claimString(claims, "team_contact_email"),
    displayName: claimString(claims, "display_name"),
    email: claimString(claims, "email"),
    role: claimString(claims, "role"),
    source,
  };
}

async function cookieParticipantToken(): Promise<string | undefined> {
  const cookieStore = await cookies();
  return cookieStore.get(participantSessionCookieName)?.value?.trim();
}

export const getParticipantSession = cache(
  async (): Promise<ParticipantSession> => {
    const directToken = process.env.AD_PLATFORM_TEAM_JWT?.trim();
    if (directToken) {
      const validated = await validateParticipantSessionWithAPI(directToken);
      if (validated === "deactivated") {
        return { authenticated: false, source: "none", reason: "deactivated" };
      }
      return validated
        ? authenticatedSession(directToken, "env", validated)
        : { authenticated: false, source: "none" };
    }

    const cookieToken = await cookieParticipantToken();
    if (!cookieToken) {
      return { authenticated: false, source: "none" };
    }

    const cookieSecret = process.env.TEAM_JWT_SECRET?.trim() ?? "";
    const claims = await verifyParticipantSessionToken(cookieToken, cookieSecret);
    if (!claims) {
      return { authenticated: false, source: "none" };
    }
    const validated = await validateParticipantSessionWithAPI(cookieToken);
    if (validated === "deactivated") {
      return { authenticated: false, source: "none", reason: "deactivated" };
    }
    return validated
      ? authenticatedSession(cookieToken, "cookie", validated)
      : { authenticated: false, source: "none" };
  },
);

const getParticipantToken = cache(async () => {
  const session = await getParticipantSession();
  if (!session.authenticated || !session.token) {
    throw new Error("participant session is not authenticated.");
  }

  return session.token;
});

type AuthenticateResponse = {
  token: string;
  token_type: string;
};

export async function authenticateParticipant(email: string, password: string) {
  const response = await fetch(`${apiBaseUrl()}/api/v2/authenticate`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ email, password }),
    cache: "no-store",
  });

  if (!response.ok) {
    throw new Error(await parseApiError(response, "/api/v2/authenticate"));
  }
  const payload = (await response.json()) as AuthenticateResponse;
  return payload.token;
}

export async function registerParticipant(input: {
  displayName: string;
  email: string;
  password: string;
}) {
  const response = await fetch(`${apiBaseUrl()}/api/v2/register`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      display_name: input.displayName,
      email: input.email,
      password: input.password,
    }),
    cache: "no-store",
  });

  if (!response.ok) {
    throw new Error(await parseApiError(response, "/api/v2/register"));
  }
  const payload = (await response.json()) as AuthenticateResponse;
  return payload.token;
}

export async function joinCurrentParticipantTeam(teamKey: string) {
  const token = await getParticipantToken();
  const response = await fetch(`${apiBaseUrl()}/api/v2/me/team`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({
      team_key: teamKey,
    }),
    cache: "no-store",
  });

  if (!response.ok) {
    throw new Error(await parseApiError(response, "/api/v2/me/team"));
  }
  const payload = (await response.json()) as AuthenticateResponse;
  return payload.token;
}

export async function updateCurrentParticipantProfile(input: {
  displayName: string;
  email: string;
  teamName: string;
  teamContactEmail: string;
}) {
  const token = await getParticipantToken();
  const response = await fetch(`${apiBaseUrl()}/api/v2/me/profile`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({
      display_name: input.displayName,
      email: input.email,
      team_name: input.teamName,
      team_contact_email: input.teamContactEmail,
    }),
    cache: "no-store",
  });

  if (!response.ok) {
    throw new Error(await parseApiError(response, "/api/v2/me/profile"));
  }
  const payload = (await response.json()) as AuthenticateResponse;
  return payload.token;
}

async function participantFetch<T>(
  path: string,
  init?: RequestInit,
): Promise<T> {
  const token = await getParticipantToken();
  return authenticatedFetch<T>(apiBaseUrl(), path, token, init);
}

export async function participantFetchResponse(
  path: string,
  init?: RequestInit,
): Promise<Response> {
  const token = await getParticipantToken();
  const response = await fetch(`${apiBaseUrl()}${path}`, {
    ...init,
    headers: {
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...init?.headers,
    },
    cache: "no-store",
  });
  if (!response.ok) {
    throw new Error(await parseApiError(response, path));
  }
  return response;
}

async function publicFetch<T>(path: string): Promise<T> {
  return authenticatedFetch<T>(apiBaseUrl(), path, null);
}

export async function listChallenges() {
  return publicFetch<Challenge[]>("/api/v2/challenges");
}

export async function listServices() {
  return participantFetch<ServicesResponseData>("/api/v2/services");
}

export async function listScoreboard() {
  return publicFetch<ScoreRow[]>("/api/v2/scoreboard");
}

export type PublicScoreboardFreeze = {
  frozen: boolean;
  configured: boolean;
  freeze_at?: string;
  unfreeze_at?: string;
  snapshot_taken_at?: string;
};

export async function getScoreboardFreezeStatus() {
  return publicFetch<PublicScoreboardFreeze>("/api/v2/scoreboard/freeze");
}

export async function getGameStatus() {
  return publicFetch<GameStatus>("/api/v2/game/status");
}



export async function listAttackFeed(query: AttackFeedQuery = { limit: 12 }) {
  return publicFetch<AttackFeedPage>(
    `/api/v2/attacks${buildQueryString(query)}`,
  );
}

export async function listTeamServices() {
  return participantFetch<TeamServiceState[]>("/api/v2/team/services");
}

export async function unlockService(challengeID: number, proof: string) {
  return participantFetch<UnlockResponseData>(
    `/api/v2/services/${challengeID}/unlock`,
    {
      method: "POST",
      body: JSON.stringify({ proof }),
    },
  );
}

export async function createSSHSession(challengeID: number) {
  return participantFetch<SSHSessionResponseData>(
    `/api/v2/services/${challengeID}/ssh-session`,
    {
      method: "POST",
    },
  );
}

export async function factoryResetService(challengeID: number) {
  return participantFetch<FactoryResetResponseData>(
    `/api/v2/services/${challengeID}/reset/factory`,
    {
      method: "POST",
    },
  );
}

export async function restartService(challengeID: number) {
  return participantFetch<RestartResponseData>(
    `/api/v2/services/${challengeID}/reset/restart`,
    {
      method: "POST",
    },
  );
}

export async function downloadChallengeSource(challengeID: number) {
  return participantFetchResponse(`/api/v2/challenges/${challengeID}/source`);
}

export async function downloadParticipantWireGuard() {
  return participantFetchResponse("/api/v2/me/wireguard");
}

export type SubmissionVerdict = {
  flag: string;
  status: string;
  detail?: string;
  message?: string;
};

export type SubmissionResult = {
  results: SubmissionVerdict[];
  accepted_count: number;
  rejected_count: number;
};

export async function submitFlags(flags: string[]) {
  return participantFetch<SubmissionResult>("/api/v2/submit", {
    method: "POST",
    body: JSON.stringify({ flags }),
  });
}

export async function changeParticipantPassword(input: {
  currentPassword: string;
  newPassword: string;
}) {
  return participantFetch<{ updated: boolean }>("/api/v2/me/password", {
    method: "PUT",
    body: JSON.stringify({
      current_password: input.currentPassword,
      new_password: input.newPassword,
    }),
  });
}

export type MatchAnnouncement = {
  id: number;
  body: string;
  created_by?: string;
  created_at: string;
};

export async function listAnnouncements() {
  return publicFetch<MatchAnnouncement[]>("/api/v2/announcements");
}
