import { cache } from "react";
import { cookies } from "next/headers";

function trimBaseUrl(value: string) {
  return value.trim().replace(/\/+$/, "");
}

export type Challenge = {
  id: number;
  name: string;
};

export type ScoreRow = {
  rank: number;
  team: string;
  attack: number;
  defense: number;
  sla: number;
  total: number;
  delta: string;
};

export type AttackEvent = {
  id: string;
  attacker: string;
  victim: string;
  service: string;
  tick: number;
  verdict: string;
};

export type AttackFeedQuery = {
  limit?: number;
  offset?: number;
  attacker?: string;
  victim?: string;
  service?: string;
  tick_from?: number;
  tick_to?: number;
};

export type AttackFeedPage = {
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
};

export type ServicesResponseData = Record<string, Record<string, string[]>>;

export type SuccessEnvelope<T> = {
  status: "success";
  data: T;
};

export type ErrorEnvelope = {
  status: "failed" | "forbidden" | "too many request";
  message: string;
};

export type UnlockResponseData = {
  challenge_id: number;
  team_id: number;
  unlocked: boolean;
  ssh_credential_ttl_seconds: number;
};

export type SSHSessionResponseData = {
  host: string;
  port: number;
  username: "root";
  password: string;
  expires_at: string;
  connection_hint: string;
};

export type FactoryResetResponseData = {
  challenge_id: number;
  team_id: number;
  action: "factory_reset";
  unlock_preserved: true;
};

export type RestartResponseData = {
  challenge_id: number;
  team_id: number;
  action: "restart";
};

export type ParticipantSession = {
  authenticated: boolean;
  token?: string;
  teamID?: number;
  playerID?: number;
  teamName?: string;
  displayName?: string;
  email?: string;
  role?: string;
  source: "cookie" | "env" | "none";
};

const participantSessionCookieName = "ad_platform_team_jwt";

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

function decodeJWTClaims(token: string): Partial<{
  team_id: number;
  player_id: number;
  team_name: string;
  display_name: string;
  email: string;
  role: string;
}> | null {
  const parts = token.split(".");
  if (parts.length !== 3) {
    return null;
  }

  try {
    const encodedPayload = parts[1].replace(/-/g, "+").replace(/_/g, "/");
    const padding =
      encodedPayload.length % 4 === 0
        ? ""
        : "=".repeat(4 - (encodedPayload.length % 4));
    const payload = Buffer.from(
      `${encodedPayload}${padding}`,
      "base64",
    ).toString("utf8");
    return JSON.parse(payload) as Partial<{
      team_id: number;
      player_id: number;
      team_name: string;
      display_name: string;
      email: string;
      role: string;
    }>;
  } catch {
    return null;
  }
}

export const getParticipantSession = cache(
  async (): Promise<ParticipantSession> => {
    const directToken = process.env.AD_PLATFORM_TEAM_JWT?.trim();
    if (directToken) {
      const claims = decodeJWTClaims(directToken);
      return {
        authenticated: true,
        token: directToken,
        teamID:
          typeof claims?.team_id === "number" ? claims.team_id : undefined,
        playerID:
          typeof claims?.player_id === "number" ? claims.player_id : undefined,
        teamName:
          typeof claims?.team_name === "string" ? claims.team_name : undefined,
        displayName:
          typeof claims?.display_name === "string"
            ? claims.display_name
            : undefined,
        email: typeof claims?.email === "string" ? claims.email : undefined,
        role: typeof claims?.role === "string" ? claims.role : undefined,
        source: "env",
      };
    }

    const cookieStore = await cookies();
    const cookieToken = cookieStore
      .get(participantSessionCookieName)
      ?.value?.trim();
    if (!cookieToken) {
      return { authenticated: false, source: "none" };
    }

    const claims = decodeJWTClaims(cookieToken);
    return {
      authenticated: true,
      token: cookieToken,
      teamID: typeof claims?.team_id === "number" ? claims.team_id : undefined,
      playerID:
        typeof claims?.player_id === "number" ? claims.player_id : undefined,
      teamName:
        typeof claims?.team_name === "string" ? claims.team_name : undefined,
      displayName:
        typeof claims?.display_name === "string"
          ? claims.display_name
          : undefined,
      email: typeof claims?.email === "string" ? claims.email : undefined,
      role: typeof claims?.role === "string" ? claims.role : undefined,
      source: "cookie",
    };
  },
);

const getParticipantToken = cache(async () => {
  const session = await getParticipantSession();
  if (!session.authenticated || !session.token) {
    throw new Error("participant session is not authenticated.");
  }

  return session.token;
});

export async function authenticateParticipant(email: string, password: string) {
  const response = await fetch(`${apiBaseUrl()}/api/v2/authenticate`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ email, password }),
    cache: "no-store",
  });

  const payload = (await response.json()) as
    | SuccessEnvelope<string>
    | ErrorEnvelope;
  if (!response.ok || payload.status !== "success") {
    throw new Error(
      "message" in payload
        ? payload.message
        : "participant authentication failed",
    );
  }

  return payload.data;
}

async function participantFetch<T>(
  path: string,
  init?: RequestInit,
): Promise<T> {
  const token = await getParticipantToken();

  const response = await fetch(`${apiBaseUrl()}${path}`, {
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
  const response = await fetch(`${apiBaseUrl()}${path}`, { cache: "no-store" });
  const payload = (await response.json()) as SuccessEnvelope<T> | ErrorEnvelope;
  if (!response.ok || payload.status !== "success") {
    throw new Error(
      "message" in payload ? payload.message : `request to ${path} failed`,
    );
  }
  return payload.data;
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

export async function getGameStatus() {
  return publicFetch<GameStatus>("/api/v2/game/status");
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
