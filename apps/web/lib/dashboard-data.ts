import type {
  AttackFeedPage,
  PlatformOverview,
  ScoreRow,
  ServiceRow,
} from "@/lib/dashboard-types";
import {
  type GameStatus,
  type PublicScoreboardFreeze,
  type ServicesResponseData,
  type TeamServiceState,
  getParticipantSession,
  getGameStatus,
  getScoreboardFreezeStatus,
  listAttackFeed,
  listChallenges,
  listScoreboard,
  listServices,
  listTeamServices,
  participantApiBaseUrl,
  participantRealtimeBaseUrl,
} from "@/lib/platform-api";

export type DashboardData = {
  scores: ScoreRow[];
  attackPage: AttackFeedPage;
  services: ServiceRow[];
  platform: PlatformOverview;
};

type DashboardDataOptions = {
  attackQuery?: {
    limit?: number;
    offset?: number;
    attacker?: string;
    victim?: string;
    service?: string;
    tick_from?: number;
    tick_to?: number;
  };
};

type ChallengeList = Awaited<ReturnType<typeof listChallenges>>;
type ScoreList = Awaited<ReturnType<typeof listScoreboard>>;

type DashboardResultSet = {
  challengesResult: PromiseSettledResult<ChallengeList>;
  serviceMapResult: PromiseSettledResult<ServicesResponseData>;
  serviceStatesResult: PromiseSettledResult<TeamServiceState[]>;
  gameStatusResult: PromiseSettledResult<GameStatus>;
  scoresResult: PromiseSettledResult<ScoreList>;
  attackPageResult: PromiseSettledResult<AttackFeedPage>;
  freezeResult: PromiseSettledResult<PublicScoreboardFreeze>;
};

type DashboardLiveData = {
  challenges: ChallengeList;
  serviceMap: ServicesResponseData;
  serviceStates: TeamServiceState[];
  gameStatus: GameStatus | null;
  scores: ScoreList;
  attackPage: AttackFeedPage;
  freeze: PublicScoreboardFreeze;
};

import { fulfilledValue } from "./promise-utils";

function emptyAttackPage(limit: number): AttackFeedPage {
  return {
    items: [],
    limit,
    offset: 0,
    total_count: 0,
    has_prev: false,
    has_next: false,
  };
}

export async function loadDashboardData(
  options: DashboardDataOptions = {},
): Promise<DashboardData> {
  const session = await getParticipantSession();
  const ownID = session.teamID ? String(session.teamID) : "";
  const hasTeam = session.authenticated && (session.teamID ?? 0) > 0;
  const results = await fetchDashboardResults(hasTeam, options);
  const data = readDashboardResults(results, options);
  const services = buildServiceRows(data, ownID);
  const source = dashboardSource(results);

  return {
    scores: data.scores,
    attackPage: data.attackPage,
    services,
    platform: buildPlatformOverview(session, data, services, ownID, source),
  };
}

async function fetchDashboardResults(
  authenticated: boolean,
  options: DashboardDataOptions,
): Promise<DashboardResultSet> {
  const emptyServiceMap: ServicesResponseData = {};
  const emptyServiceStates: TeamServiceState[] = [];
  const [
    challengesResult,
    serviceMapResult,
    serviceStatesResult,
    gameStatusResult,
    scoresResult,
    attackPageResult,
    freezeResult,
  ] = await Promise.allSettled([
    listChallenges(),
    authenticated ? listServices() : Promise.resolve(emptyServiceMap),
    authenticated ? listTeamServices() : Promise.resolve(emptyServiceStates),
    getGameStatus(),
    listScoreboard(),
    listAttackFeed(options.attackQuery ?? { limit: 12 }),
    getScoreboardFreezeStatus(),
  ]);

  return {
    challengesResult,
    serviceMapResult,
    serviceStatesResult,
    gameStatusResult,
    scoresResult,
    attackPageResult,
    freezeResult,
  };
}

function readDashboardResults(
  results: DashboardResultSet,
  options: DashboardDataOptions,
): DashboardLiveData {
  return {
    challenges: fulfilledValue(results.challengesResult, []),
    serviceMap: fulfilledValue(results.serviceMapResult, {}),
    serviceStates: fulfilledValue(results.serviceStatesResult, []),
    gameStatus: fulfilledValue<GameStatus | null>(results.gameStatusResult, null),
    scores: fulfilledValue(results.scoresResult, []),
    attackPage: fulfilledValue(
      results.attackPageResult,
      emptyAttackPage(options.attackQuery?.limit ?? 12),
    ),
    freeze: fulfilledValue<PublicScoreboardFreeze>(results.freezeResult, {
      frozen: false,
      configured: false,
    }),
  };
}

function buildServiceRows(data: DashboardLiveData, ownID: string): ServiceRow[] {
  const challengeByID = new Map(
    data.challenges.map((challenge) => [String(challenge.id), challenge] as const),
  );

  if (data.serviceStates.length > 0) {
    return data.serviceStates.map((state) =>
      serviceStateToRow(state, challengeByID),
    );
  }

  return Object.entries(data.serviceMap)
    .flatMap(([challengeID, teams]) =>
      fallbackServiceRows(challengeID, teams[ownID] ?? [], ownID, challengeByID),
    )
    .filter((service) => service.endpoint.length > 0);
}

function serviceStateToRow(
  state: TeamServiceState,
  challengeByID: Map<
    string,
    { id: number; name: string; has_source_download: boolean; maintenance?: boolean }
  >,
): ServiceRow {
  const port = Number(state.endpoint.split(":").at(-1) ?? 0);
  const challenge = challengeByID.get(String(state.challenge_id));
  return {
    id: `svc-${state.challenge_id}`,
    challengeId: state.challenge_id,
    teamId: state.team_id,
    name:
      state.name ??
      challenge?.name ??
      `challenge-${state.challenge_id}`,
    endpoint: state.endpoint,
    port,
    status: state.status,
    checker: state.checker,
    hasSourceDownload: challenge?.has_source_download ?? false,
    unlocked: state.unlocked,
    sshHint: state.ssh_hint,
    lastEvent: state.last_event,
    resetCooldown: state.reset_cooldown,
    maintenance: state.maintenance ?? challenge?.maintenance ?? false,
    lockReason: state.lock_reason ?? (challenge?.maintenance ? "maintenance" : undefined),
    slaStatus: state.sla_status ?? "unknown",
    slaPhase: state.sla_phase ?? "",
    slaTickId: state.sla_tick_id ?? null,
    slaMessage: state.sla_message ?? "awaiting first checker run",
  };
}

function fallbackServiceRows(
  challengeID: string,
  endpoints: string[],
  ownID: string,
  challengeByID: Map<
    string,
    { id: number; name: string; has_source_download: boolean; maintenance?: boolean }
  >,
): ServiceRow[] {
  const challenge = challengeByID.get(challengeID);
  return endpoints.map((endpoint, index) => ({
    id: `svc-${challengeID}-${index + 1}`,
    challengeId: Number(challengeID),
    teamId: Number(ownID),
    name: challenge?.name ?? `challenge-${challengeID}`,
    endpoint,
    port: Number(endpoint.split(":").at(-1) ?? 0),
    status: "warming",
    checker: "warning",
    hasSourceDownload: challenge?.has_source_download ?? false,
    unlocked: false,
    sshHint: "service state endpoint unavailable",
    lastEvent: "service state endpoint unavailable",
    resetCooldown: "unknown",
    maintenance: challenge?.maintenance ?? false,
    lockReason: challenge?.maintenance ? "maintenance" : undefined,
    slaStatus: "unknown",
    slaPhase: "",
    slaTickId: null,
    slaMessage: "service state endpoint unavailable",
  }));
}

function dashboardSource(results: DashboardResultSet): PlatformOverview["source"] {
  return Object.values(results).every((result) => result.status === "fulfilled")
    ? "live"
    : "degraded";
}

function buildPlatformOverview(
  session: Awaited<ReturnType<typeof getParticipantSession>>,
  data: DashboardLiveData,
  services: ServiceRow[],
  ownID: string,
  source: PlatformOverview["source"],
): PlatformOverview {
  return {
    source,
    message: dashboardMessage(session, source),
    authenticated: session.authenticated,
    teamID: session.teamID,
    playerID: session.playerID,
    teamName: session.teamName,
    teamContactEmail: session.teamContactEmail,
    displayName: session.displayName,
    email: session.email,
    role: session.role,
    challengeCount: data.challenges.length,
    ownServiceCount: services.length,
    enemyTargetCount: countEnemyTargets(data.serviceMap, ownID),
    matchState: data.gameStatus?.match?.state,
    schedulerState: data.gameStatus?.scheduler?.state,
    currentTick: data.gameStatus?.current_tick?.id,
    acceptingSubmissions: data.gameStatus?.match?.accepting_submissions,
    nextTickAt: data.gameStatus?.scheduler?.next_run_at,
    tickInterval: data.gameStatus?.scheduler?.interval_seconds,
    lastTickAt: data.gameStatus?.scheduler?.last_run_at,
    apiBaseUrl: participantApiBaseUrl(),
    realtimeBaseUrl: participantRealtimeBaseUrl(),
    scoreboardFrozen: data.freeze.frozen,
    scoreboardFreezeAt: data.freeze.freeze_at,
    scoreboardUnfreezeAt: data.freeze.unfreeze_at,
  };
}

function dashboardMessage(
  session: Awaited<ReturnType<typeof getParticipantSession>>,
  source: PlatformOverview["source"],
): string | undefined {
  const messageParts: string[] = [];
  if (session.reason === "deactivated") {
    messageParts.push(
      "Your account or team has been deactivated by the organizers. You are signed out and removed from play. Contact the organizers if you believe this is a mistake.",
    );
  } else if (!session.authenticated) {
    messageParts.push(
      "Participant login is required for owned services, unlock, SSH, and reset actions.",
    );
  } else if ((session.teamID ?? 0) <= 0 && session.role !== "organizer") {
    messageParts.push(
      "Join a team before accessing owned services, unlock, SSH, reset actions, or VPN config.",
    );
  }
  if (source === "degraded") {
    messageParts.push(
      "Participant data is partially unavailable. Only live responses that succeeded are shown. No sample data is injected.",
    );
  }
  return messageParts.length > 0 ? messageParts.join(" ") : undefined;
}

function countEnemyTargets(serviceMap: ServicesResponseData, ownID: string): number {
  if (ownID === "") {
    return 0;
  }

  return Object.values(serviceMap).reduce(
    (total, teams) =>
      total +
      Object.entries(teams)
        .filter(([teamID]) => teamID !== ownID)
        .reduce((count, [, endpoints]) => count + endpoints.length, 0),
    0,
  );
}
