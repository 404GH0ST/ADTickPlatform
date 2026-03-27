import type {
  AdminAttackFeedPage,
  AdminCheckerRunPage,
  AdminDeploymentJob,
  AdminGameScoreRow,
  AdminGameStatus,
  AdminOverview,
  AdminOperationsStatus,
  AdminPlayer,
  AdminSchedulerEventPage,
  AdminTeam,
  AdminChallenge,
} from '@/lib/admin-dashboard-types';
import {
  getAdminGameStatus,
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

function isFulfilled<T>(
  result: PromiseSettledResult<T>,
): result is PromiseFulfilledResult<T> {
  return result.status === 'fulfilled';
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

export async function loadAdminDashboardData(
  options: AdminDashboardDataOptions = {},
): Promise<AdminDashboardData> {
  const [teamsResult, playersResult, challengesResult, deploymentsResult, operationsStatusResult, gameStatusResult, schedulerEventPageResult, checkerRunPageResult, attackPageResult, scoreboardResult] =
    await Promise.allSettled([
      listAdminTeams(),
      listAdminPlayers(),
      listAdminChallenges(),
      listAdminDeployments(),
      getAdminOperationsStatus(),
      getAdminGameStatus(),
      listAdminGameSchedulerEvents({ limit: 12 }),
      listAdminCheckerRuns({ limit: 18 }),
      listAdminGameAttacks(options.attackQuery ?? { limit: 12 }),
      listAdminGameScoreboard(),
    ]);

  const teams = isFulfilled(teamsResult) ? teamsResult.value : [];
  const players = isFulfilled(playersResult) ? playersResult.value : [];
  const challenges = isFulfilled(challengesResult) ? challengesResult.value : [];
  const deployments = isFulfilled(deploymentsResult) ? deploymentsResult.value : [];
  const operationsStatus = isFulfilled(operationsStatusResult)
    ? operationsStatusResult.value
    : emptyOperationsStatus();
  const gameStatus = isFulfilled(gameStatusResult)
    ? gameStatusResult.value
    : emptyGameStatus();
  const schedulerEventPage = isFulfilled(schedulerEventPageResult)
    ? schedulerEventPageResult.value
    : emptySchedulerEventPage(12);
  const checkerRunPage = isFulfilled(checkerRunPageResult)
    ? checkerRunPageResult.value
    : emptyCheckerRunPage(18);
  const attackPage = isFulfilled(attackPageResult)
    ? attackPageResult.value
    : emptyAttackPage(options.attackQuery?.limit ?? 12);
  const scoreboard = isFulfilled(scoreboardResult) ? scoreboardResult.value : [];

  const source: AdminOverview['source'] =
    teamsResult.status === 'fulfilled' &&
    playersResult.status === 'fulfilled' &&
    challengesResult.status === 'fulfilled' &&
    deploymentsResult.status === 'fulfilled' &&
    operationsStatusResult.status === 'fulfilled' &&
    gameStatusResult.status === 'fulfilled' &&
    schedulerEventPageResult.status === 'fulfilled' &&
    checkerRunPageResult.status === 'fulfilled' &&
    attackPageResult.status === 'fulfilled' &&
    scoreboardResult.status === 'fulfilled'
      ? 'live'
      : 'degraded';

  return {
    teams,
    players,
    challenges,
    deployments,
    gameStatus,
    schedulerEventPage,
    checkerRunPage,
    attackPage,
    scoreboard,
    overview: {
      source,
      message:
        source === 'degraded'
          ? 'Organizer data is partially unavailable. Only live responses that succeeded are shown. No sample data is injected.'
          : undefined,
      operationsStatus,
      operationsAlertCount: operationsStatus.alerts.length,
      teamCount: teams.length,
      playerCount: players.length,
      challengeCount: challenges.length,
      publishedChallengeCount: challenges.filter((challenge) => challenge.published).length,
      deploymentCount: deployments.length,
      pendingDeploymentCount: deployments.filter((deployment) => deployment.status !== 'completed').length,
      totalTicks: gameStatus.total_ticks,
      scoreboardRows: scoreboard.length,
      apiBaseUrl: organizerApiBaseUrl(),
    },
  };
}
