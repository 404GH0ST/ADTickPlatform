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

function isFulfilled<T>(
  result: PromiseSettledResult<T>,
): result is PromiseFulfilledResult<T> {
  return result.status === "fulfilled";
}

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
    session.authenticated ? listServices() : Promise.resolve(emptyServiceMap),
    session.authenticated
      ? listTeamServices()
      : Promise.resolve(emptyServiceStates),
    getGameStatus(),
    listScoreboard(),
    listAttackFeed(options.attackQuery ?? { limit: 12 }),
  ]);

  const challenges = isFulfilled(challengesResult)
    ? challengesResult.value
    : [];
  const serviceMap = isFulfilled(serviceMapResult)
    ? serviceMapResult.value
    : {};
  const serviceStates = isFulfilled(serviceStatesResult)
    ? serviceStatesResult.value
    : [];
  const gameStatus: GameStatus | null = isFulfilled(gameStatusResult)
    ? gameStatusResult.value
    : null;
  const scores = isFulfilled(scoresResult) ? scoresResult.value : [];
  const attackPage = isFulfilled(attackPageResult)
    ? attackPageResult.value
    : emptyAttackPage(options.attackQuery?.limit ?? 12);

  const challengeNameByID = new Map(
    challenges.map((challenge) => [String(challenge.id), challenge.name]),
  );

  let enemyTargetCount = 0;
  if (ownID !== "") {
    for (const teams of Object.values(serviceMap)) {
      enemyTargetCount += Object.entries(teams)
        .filter(([teamID]) => teamID !== ownID)
        .reduce((count, [, endpoints]) => count + endpoints.length, 0);
    }
  }

  const services =
    serviceStates.length > 0
      ? serviceStates.map((state) => {
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
          } satisfies ServiceRow;
        })
      : Object.entries(serviceMap)
          .flatMap(([challengeID, teams]) => {
            const endpoints = teams[ownID] ?? [];
            return endpoints.map(
              (endpoint, index) =>
                ({
                  id: `svc-${challengeID}-${index + 1}`,
                  challengeId: Number(challengeID),
                  teamId: Number(ownID),
                  name:
                    challengeNameByID.get(challengeID) ??
                    `challenge-${challengeID}`,
                  endpoint,
                  port: Number(endpoint.split(":").at(-1) ?? 0),
                  status: "warming",
                  checker: "warning",
                  unlocked: false,
                  sshHint: "service state endpoint unavailable",
                  lastEvent: "service state endpoint unavailable",
                  resetCooldown: "unknown",
                }) satisfies ServiceRow,
            );
          })
          .filter((service) => service.endpoint.length > 0);

  const source: PlatformOverview["source"] =
    challengesResult.status === "fulfilled" &&
    serviceMapResult.status === "fulfilled" &&
    serviceStatesResult.status === "fulfilled" &&
    gameStatusResult.status === "fulfilled" &&
    scoresResult.status === "fulfilled" &&
    attackPageResult.status === "fulfilled"
      ? "live"
      : "degraded";

  const messageParts: string[] = [];
  if (!session.authenticated) {
    messageParts.push(
      "Participant login is required for owned services, unlock, SSH, and reset actions.",
    );
  }
  if (source === "degraded") {
    messageParts.push(
      "Participant data is partially unavailable. Only live responses that succeeded are shown. No sample data is injected.",
    );
  }

  return {
    scores,
    attackPage,
    services,
    platform: {
      source,
      message: messageParts.length > 0 ? messageParts.join(" ") : undefined,
      authenticated: session.authenticated,
      teamID: session.teamID,
      teamName: session.teamName,
      displayName: session.displayName,
      role: session.role,
      challengeCount: challenges.length,
      ownServiceCount: services.length,
      enemyTargetCount,
      matchState: gameStatus?.match?.state,
      schedulerState: gameStatus?.scheduler?.state,
      currentTick: gameStatus?.current_tick?.id,
      acceptingSubmissions: gameStatus?.match?.accepting_submissions,
      apiBaseUrl: participantApiBaseUrl(),
      realtimeBaseUrl: participantRealtimeBaseUrl(),
    },
  };
}
