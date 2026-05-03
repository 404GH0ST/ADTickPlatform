import type {
  AdminAttackFeedPage,
  AdminCheckerRunPage,
  AdminDeploymentJob,
  AdminGameScoreRow,
  AdminGameStatus,
  AdminOverview,
  AdminOperationsStatus,
  AdminServiceMetricSnapshot,
  AdminPlayer,
  AdminSchedulerEventPage,
  AdminTeam,
  AdminChallenge,
} from '@/lib/admin-dashboard-types';
import {
  getAdminGameStatus,
  getAdminOperationsMetrics,
  getAdminOperationsStatus,
  listAdminChallenges,
  listAdminCheckerRuns,
  listAdminDeployments,
  listAdminGameAttacks,
  listAdminGameSchedulerEvents,
  listAdminGameScoreboard,
  listAdminPlayers,
  listAdminTeams,
  organizerApiBaseUrl,
} from '@/lib/admin-api';

export type AdminDashboardData = {
  teams: AdminTeam[];
  players: AdminPlayer[];
  challenges: AdminChallenge[];
  deployments: AdminDeploymentJob[];
  gameStatus: AdminGameStatus;
  schedulerEventPage: AdminSchedulerEventPage;
  checkerRunPage: AdminCheckerRunPage;
  attackPage: AdminAttackFeedPage;
  scoreboard: AdminGameScoreRow[];
  serviceMetrics: AdminServiceMetricSnapshot | null;
  overview: AdminOverview;
};

type AdminDashboardDataOptions = {
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

type TeamList = Awaited<ReturnType<typeof listAdminTeams>>;
type PlayerList = Awaited<ReturnType<typeof listAdminPlayers>>;
type ChallengeList = Awaited<ReturnType<typeof listAdminChallenges>>;
type DeploymentList = Awaited<ReturnType<typeof listAdminDeployments>>;
type Scoreboard = Awaited<ReturnType<typeof listAdminGameScoreboard>>;

type AdminResultSet = {
  teamsResult: PromiseSettledResult<TeamList>;
  playersResult: PromiseSettledResult<PlayerList>;
  challengesResult: PromiseSettledResult<ChallengeList>;
  deploymentsResult: PromiseSettledResult<DeploymentList>;
  operationsStatusResult: PromiseSettledResult<AdminOperationsStatus>;
  serviceMetricsResult: PromiseSettledResult<AdminServiceMetricSnapshot>;
  gameStatusResult: PromiseSettledResult<AdminGameStatus>;
  schedulerEventPageResult: PromiseSettledResult<AdminSchedulerEventPage>;
  checkerRunPageResult: PromiseSettledResult<AdminCheckerRunPage>;
  attackPageResult: PromiseSettledResult<AdminAttackFeedPage>;
  scoreboardResult: PromiseSettledResult<Scoreboard>;
};

type AdminLiveData = {
  teams: TeamList;
  players: PlayerList;
  challenges: ChallengeList;
  deployments: DeploymentList;
  operationsStatus: AdminOperationsStatus;
  serviceMetrics: AdminServiceMetricSnapshot | null;
  gameStatus: AdminGameStatus;
  schedulerEventPage: AdminSchedulerEventPage;
  checkerRunPage: AdminCheckerRunPage;
  attackPage: AdminAttackFeedPage;
  scoreboard: Scoreboard;
};

function isFulfilled<T>(
  result: PromiseSettledResult<T>,
): result is PromiseFulfilledResult<T> {
  return result.status === 'fulfilled';
}

function fulfilledValue<T>(result: PromiseSettledResult<T>, fallback: T): T {
  return isFulfilled(result) ? result.value : fallback;
}

function emptySchedulerEventPage(limit: number): AdminSchedulerEventPage {
  return {
    items: [],
    limit,
    offset: 0,
    total_count: 0,
    has_prev: false,
    has_next: false,
  };
}

function emptyCheckerRunPage(limit: number): AdminCheckerRunPage {
  return {
    items: [],
    limit,
    offset: 0,
    total_count: 0,
    has_prev: false,
    has_next: false,
  };
}

function emptyAttackPage(limit: number): AdminAttackFeedPage {
  return {
    items: [],
    limit,
    offset: 0,
    total_count: 0,
    has_prev: false,
    has_next: false,
  };
}

function emptyGameStatus(): AdminGameStatus {
  return {
    total_ticks: 0,
    total_checker_runs: 0,
    successful_checker_runs: 0,
    failed_checker_runs: 0,
    skipped_checker_runs: 0,
  };
}

function emptyOperationsStatus(): AdminOperationsStatus {
  return {
    healthy: true,
    generated_at: '',
    alerts: [],
  };
}

function emptyServiceMetrics(): AdminServiceMetricSnapshot | null {
  return null;
}

export async function loadAdminDashboardData(
  options: AdminDashboardDataOptions = {},
): Promise<AdminDashboardData> {
  const results = await fetchAdminDashboardResults(options);
  const data = readAdminDashboardResults(results, options);
  const source = adminDashboardSource(results);

  return {
    teams: data.teams,
    players: data.players,
    challenges: data.challenges,
    deployments: data.deployments,
    gameStatus: data.gameStatus,
    schedulerEventPage: data.schedulerEventPage,
    checkerRunPage: data.checkerRunPage,
    attackPage: data.attackPage,
    scoreboard: data.scoreboard,
    serviceMetrics: data.serviceMetrics,
    overview: buildAdminOverview(data, source),
  };
}

async function fetchAdminDashboardResults(
  options: AdminDashboardDataOptions,
): Promise<AdminResultSet> {
  const [
    teamsResult,
    playersResult,
    challengesResult,
    deploymentsResult,
    operationsStatusResult,
    serviceMetricsResult,
    gameStatusResult,
    schedulerEventPageResult,
    checkerRunPageResult,
    attackPageResult,
    scoreboardResult,
  ] = await Promise.allSettled([
    listAdminTeams(),
    listAdminPlayers(),
    listAdminChallenges(),
    listAdminDeployments(),
    getAdminOperationsStatus(),
    getAdminOperationsMetrics(),
    getAdminGameStatus(),
    listAdminGameSchedulerEvents({ limit: 12 }),
    listAdminCheckerRuns({ limit: 18 }),
    listAdminGameAttacks(options.attackQuery ?? { limit: 12 }),
    listAdminGameScoreboard(),
  ]);

  return {
    teamsResult,
    playersResult,
    challengesResult,
    deploymentsResult,
    operationsStatusResult,
    serviceMetricsResult,
    gameStatusResult,
    schedulerEventPageResult,
    checkerRunPageResult,
    attackPageResult,
    scoreboardResult,
  };
}

function readAdminDashboardResults(
  results: AdminResultSet,
  options: AdminDashboardDataOptions,
): AdminLiveData {
  return {
    teams: fulfilledValue(results.teamsResult, []),
    players: fulfilledValue(results.playersResult, []),
    challenges: fulfilledValue(results.challengesResult, []),
    deployments: fulfilledValue(results.deploymentsResult, []),
    operationsStatus: fulfilledValue(
      results.operationsStatusResult,
      emptyOperationsStatus(),
    ),
    serviceMetrics: fulfilledValue<AdminServiceMetricSnapshot | null>(
      results.serviceMetricsResult,
      emptyServiceMetrics(),
    ),
    gameStatus: fulfilledValue(results.gameStatusResult, emptyGameStatus()),
    schedulerEventPage: fulfilledValue(
      results.schedulerEventPageResult,
      emptySchedulerEventPage(12),
    ),
    checkerRunPage: fulfilledValue(
      results.checkerRunPageResult,
      emptyCheckerRunPage(18),
    ),
    attackPage: fulfilledValue(
      results.attackPageResult,
      emptyAttackPage(options.attackQuery?.limit ?? 12),
    ),
    scoreboard: fulfilledValue(results.scoreboardResult, []),
  };
}

function adminDashboardSource(
  results: AdminResultSet,
): AdminOverview['source'] {
  return Object.values(results).every((result) => result.status === 'fulfilled')
    ? 'live'
    : 'degraded';
}

function buildAdminOverview(
  data: AdminLiveData,
  source: AdminOverview['source'],
): AdminOverview {
  return {
    source,
    message:
      source === 'degraded'
        ? 'Organizer data is partially unavailable. Only live responses that succeeded are shown. No sample data is injected.'
        : undefined,
    operationsStatus: data.operationsStatus,
    operationsAlertCount: data.operationsStatus.alerts.length,
    teamCount: data.teams.length,
    playerCount: data.players.length,
    challengeCount: data.challenges.length,
    publishedChallengeCount: data.challenges.filter(
      (challenge) => challenge.published,
    ).length,
    deploymentCount: data.deployments.length,
    pendingDeploymentCount: data.deployments.filter(
      (deployment) => deployment.status !== 'completed',
    ).length,
    totalTicks: data.gameStatus.total_ticks,
    scoreboardRows: data.scoreboard.length,
    apiBaseUrl: organizerApiBaseUrl(),
  };
}
