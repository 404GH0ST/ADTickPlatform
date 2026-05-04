import type {
  AttackFeedPage,
  PlatformOverview,
  ScoreRow,
  ServiceRow,
} from "@/lib/dashboard-types";
import {
  type GameStatus,
  type ServicesResponseData,
  type TeamServiceState,
  getParticipantSession,
  getGameStatus,
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
};

type DashboardLiveData = {
  challenges: ChallengeList;
  serviceMap: ServicesResponseData;
  serviceStates: TeamServiceState[];
  gameStatus: GameStatus | null;
  scores: ScoreList;
  attackPage: AttackFeedPage;
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
  const results = await fetchDashboardResults(session.authenticated, options);
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
  ] = await Promise.allSettled([
    listChallenges(),
    authenticated ? listServices() : Promise.resolve(emptyServiceMap),
    authenticated ? listTeamServices() : Promise.resolve(emptyServiceStates),
    getGameStatus(),
    listScoreboard(),
    listAttackFeed(options.attackQuery ?? { limit: 12 }),
  ]);

  return {
    challengesResult,
    serviceMapResult,
    serviceStatesResult,
    gameStatusResult,
    scoresResult,
    attackPageResult,
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
  };
}

function buildServiceRows(data: DashboardLiveData, ownID: string): ServiceRow[] {
  const challengeNameByID = new Map(
    data.challenges.map((challenge) => [String(challenge.id), challenge.name]),
  );

  if (data.serviceStates.length > 0) {
    return data.serviceStates.map((state) =>
      serviceStateToRow(state, challengeNameByID),
    );
  }

  return Object.entries(data.serviceMap)
    .flatMap(([challengeID, teams]) =>
      fallbackServiceRows(challengeID, teams[ownID] ?? [], ownID, challengeNameByID),
    )
    .filter((service) => service.endpoint.length > 0);
}

function serviceStateToRow(
  state: TeamServiceState,
  challengeNameByID: Map<string, string>,
): ServiceRow {
  const port = Number(state.endpoint.split(":").at(-1) ?? 0);
  return {
    id: `svc-${state.challenge_id}`,
    challengeId: state.challenge_id,
    teamId: state.team_id,
    name:
      state.name ??
      challengeNameByID.get(String(state.challenge_id)) ??
      `challenge-${state.challenge_id}`,
    endpoint: state.endpoint,
    port,
    status: state.status,
    checker: state.checker,
    unlocked: state.unlocked,
    sshHint: state.ssh_hint,
    lastEvent: state.last_event,
    resetCooldown: state.reset_cooldown,
  };
}

function fallbackServiceRows(
  challengeID: string,
  endpoints: string[],
  ownID: string,
  challengeNameByID: Map<string, string>,
): ServiceRow[] {
  return endpoints.map((endpoint, index) => ({
    id: `svc-${challengeID}-${index + 1}`,
    challengeId: Number(challengeID),
    teamId: Number(ownID),
    name: challengeNameByID.get(challengeID) ?? `challenge-${challengeID}`,
    endpoint,
    port: Number(endpoint.split(":").at(-1) ?? 0),
    status: "warming",
    checker: "warning",
    unlocked: false,
    sshHint: "service state endpoint unavailable",
    lastEvent: "service state endpoint unavailable",
    resetCooldown: "unknown",
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
    message: dashboardMessage(session.authenticated, source),
    authenticated: session.authenticated,
    teamID: session.teamID,
    teamName: session.teamName,
    displayName: session.displayName,
    role: session.role,
    challengeCount: data.challenges.length,
    ownServiceCount: services.length,
    enemyTargetCount: countEnemyTargets(data.serviceMap, ownID),
    matchState: data.gameStatus?.match?.state,
    schedulerState: data.gameStatus?.scheduler?.state,
    currentTick: data.gameStatus?.current_tick?.id,
    acceptingSubmissions: data.gameStatus?.match?.accepting_submissions,
    apiBaseUrl: participantApiBaseUrl(),
    realtimeBaseUrl: participantRealtimeBaseUrl(),
  };
}

function dashboardMessage(
  authenticated: boolean,
  source: PlatformOverview["source"],
): string | undefined {
  const messageParts: string[] = [];
  if (!authenticated) {
    messageParts.push(
      "Participant login is required for owned services, unlock, SSH, and reset actions.",
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
