import { cache } from "react";
import { cookies, headers } from "next/headers";

import { participantSessionCookieName } from "@/lib/participant-session-cookie";
import {
  type ParticipantSessionValidation,
  validateParticipantSessionWithAPI,
} from "@/lib/session-validation";
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
  /** pwn | web | crypto, or "" when uncategorized. Older gateways omit it. */
  category?: string;
  /** tcp | http. Older gateways omit it (treat as tcp). */
  protocol?: string;
};

type ScoreRow = {
  rank: number;
  team: string;
  institution?: string;
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
  first_blood?: boolean;
  id: string;
  attacker: string;
  victim: string;
  attacker_institution?: string;
  victim_institution?: string;
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
  revision?: number;
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
  lock_reason?: string;
  sla_status?: "ok" | "recovering" | "flag_not_found" | "faulty" | "down" | "unknown";
  sla_phase?: string;
  sla_tick_id?: number;
  sla_message?: string;
};

export type ServicesResponseData = Record<string, Record<string, string[]>>;

import {
  authenticatedFetch,
  buildQueryString,
  firstForwardedAddress,
  parseApiError,
  PlatformAPIError,
  SESSION_ENDED_MESSAGE,
} from "./api-utils";

export { PlatformAPIError } from "./api-utils";

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
  penalty_points?: number;
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
  /** Why there is no session. "unavailable" still grants no access. */
  reason?: "deactivated" | "unavailable";
};

type AuthenticatedSessionSource = Exclude<ParticipantSession["source"], "none">;

// Pages wait on reads, so a silent gateway must not hold them open. Actions
// such as restart and factory reset stay unbounded: they can legitimately run
// long, and abandoning one would leave its outcome unknown.
const READ_TIMEOUT_MS = 5_000;
const SIGN_IN_TIMEOUT_MS = 15_000;
// Safe to bound: resubmitting a flag that already counted reports a duplicate.
const FLAG_SUBMIT_TIMEOUT_MS = 15_000;

function apiBaseUrl() {
  return trimBaseUrl(
    process.env.AD_PLATFORM_API_URL ?? "http://127.0.0.1:8080",
  );
}

async function platformRequestHeaders(initial?: HeadersInit): Promise<Headers> {
  const result = new Headers(initial);
  try {
    const incoming = await headers();
    const clientAddress = firstForwardedAddress(
      incoming.get("x-forwarded-for"),
    );
    if (clientAddress) {
      result.set("X-Forwarded-For", clientAddress);
    }
  } catch {
    // Static generation and background calls do not have an incoming request.
  }
  return result;
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

function sessionFromValidation(
  token: string,
  source: AuthenticatedSessionSource,
  validated: ParticipantSessionValidation,
): ParticipantSession {
  if (validated === "deactivated" || validated === "unavailable") {
    return { authenticated: false, source: "none", reason: validated };
  }
  return validated
    ? authenticatedSession(token, source, validated)
    : { authenticated: false, source: "none" };
}

export const getParticipantSession = cache(
  async (): Promise<ParticipantSession> => {
    const directToken = process.env.AD_PLATFORM_TEAM_JWT?.trim();
    if (directToken) {
      const validated = await validateParticipantSessionWithAPI(directToken);
      return sessionFromValidation(directToken, "env", validated);
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
    return sessionFromValidation(cookieToken, "cookie", validated);
  },
);

export const getParticipantToken = cache(async () => {
  const session = await getParticipantSession();
  if (session.reason === "unavailable") {
    throw new PlatformAPIError(
      "The platform API can't be reached, so this action couldn't be checked. Try again in a moment.",
      503,
    );
  }
  if (!session.authenticated || !session.token) {
    throw new Error(SESSION_ENDED_MESSAGE);
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
    headers: await platformRequestHeaders({
      "Content-Type": "application/json",
    }),
    body: JSON.stringify({ email, password }),
    cache: "no-store",
    signal: AbortSignal.timeout(SIGN_IN_TIMEOUT_MS),
  });

  if (!response.ok) {
    throw new PlatformAPIError(
      await parseApiError(response, "/api/v2/authenticate"),
      response.status,
      response.headers.get("retry-after"),
    );
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
    headers: await platformRequestHeaders({
      "Content-Type": "application/json",
    }),
    body: JSON.stringify({
      display_name: input.displayName,
      email: input.email,
      password: input.password,
    }),
    cache: "no-store",
    signal: AbortSignal.timeout(SIGN_IN_TIMEOUT_MS),
  });

  if (!response.ok) {
    throw new PlatformAPIError(
      await parseApiError(response, "/api/v2/register"),
      response.status,
      response.headers.get("retry-after"),
    );
  }
  const payload = (await response.json()) as AuthenticateResponse;
  return payload.token;
}

export async function joinCurrentParticipantTeam(teamKey: string) {
  const token = await getParticipantToken();
  const response = await fetch(`${apiBaseUrl()}/api/v2/me/team`, {
    method: "POST",
    headers: await platformRequestHeaders({
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
    }),
    body: JSON.stringify({
      team_key: teamKey,
    }),
    cache: "no-store",
  });

  if (!response.ok) {
    throw new PlatformAPIError(
      await parseApiError(response, "/api/v2/me/team"),
      response.status,
      response.headers.get("retry-after"),
    );
  }
  const payload = (await response.json()) as AuthenticateResponse;
  return payload.token;
}

export type TeamMember = {
  player_id: number;
  display_name: string;
  email: string;
  role: string;
};

export async function listCurrentTeamMembers() {
  return participantFetch<TeamMember[]>("/api/v2/me/team/members");
}

export async function transferCurrentTeamCaptain(playerID: number) {
  const token = await getParticipantToken();
  const response = await fetch(`${apiBaseUrl()}/api/v2/me/team/captain`, {
    method: "POST",
    headers: await platformRequestHeaders({
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
    }),
    body: JSON.stringify({
      player_id: playerID,
    }),
    cache: "no-store",
  });

  if (!response.ok) {
    throw new PlatformAPIError(
      await parseApiError(response, "/api/v2/me/team/captain"),
      response.status,
      response.headers.get("retry-after"),
    );
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
    headers: await platformRequestHeaders({
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
    }),
    body: JSON.stringify({
      display_name: input.displayName,
      email: input.email,
      team_name: input.teamName,
      team_contact_email: input.teamContactEmail,
    }),
    cache: "no-store",
  });

  if (!response.ok) {
    throw new PlatformAPIError(
      await parseApiError(response, "/api/v2/me/profile"),
      response.status,
      response.headers.get("retry-after"),
    );
  }
  const payload = (await response.json()) as AuthenticateResponse;
  return payload.token;
}

async function participantFetch<T>(
  path: string,
  init?: RequestInit,
): Promise<T> {
  const token = await getParticipantToken();
  const isRead = (init?.method ?? "GET").toUpperCase() === "GET";
  return authenticatedFetch<T>(apiBaseUrl(), path, token, {
    ...init,
    headers: await platformRequestHeaders(init?.headers),
    signal: init?.signal ?? (isRead ? AbortSignal.timeout(READ_TIMEOUT_MS) : undefined),
  });
}

export async function participantFetchResponse(
  path: string,
  init?: RequestInit,
): Promise<Response> {
  const token = await getParticipantToken();
  const requestHeaders = await platformRequestHeaders(init?.headers);
  requestHeaders.set("Authorization", `Bearer ${token}`);
  const response = await fetch(`${apiBaseUrl()}${path}`, {
    ...init,
    headers: requestHeaders,
    cache: "no-store",
  });
  if (!response.ok) {
    throw new PlatformAPIError(
      await parseApiError(response, path),
      response.status,
      response.headers.get("retry-after"),
    );
  }
  return response;
}

async function publicFetch<T>(path: string): Promise<T> {
  return authenticatedFetch<T>(apiBaseUrl(), path, null, {
    headers: await platformRequestHeaders(),
    signal: AbortSignal.timeout(READ_TIMEOUT_MS),
  });
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

export async function submitFlags(token: string, flags: string[]) {
  return authenticatedFetch<SubmissionResult>(apiBaseUrl(), "/api/v2/submit", token, {
    method: "POST",
    headers: await platformRequestHeaders(),
    body: JSON.stringify({ flags }),
    signal: AbortSignal.timeout(FLAG_SUBMIT_TIMEOUT_MS),
  });
}

export async function changeParticipantPassword(input: {
  currentPassword: string;
  newPassword: string;
}) {
  return participantFetch<{ updated: boolean; token: string; token_type: string }>("/api/v2/me/password", {
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
