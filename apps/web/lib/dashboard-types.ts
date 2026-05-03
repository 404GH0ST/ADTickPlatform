export type ScoreRow = {
  rank: number;
  team: string;
  attack: number;
  defense: number;
  sla: number;
  total: number;
  delta: string;
};

export type ServiceRow = {
  id: string;
  challengeId: number;
  teamId: number;
  name: string;
  endpoint: string;
  port: number;
  status: 'stable' | 'warming' | 'degraded';
  checker: 'passing' | 'warning';
  unlocked: boolean;
  sshHint: string;
  lastEvent: string;
  resetCooldown: string;
};

export type AttackEvent = {
  id: string;
  attacker: string;
  victim: string;
  service: string;
  tick: number;
  verdict: string;
};

export type AttackFeedPage = {
  items: AttackEvent[];
  limit: number;
  offset: number;
  total_count: number;
  has_prev: boolean;
  has_next: boolean;
};

export type PlatformOverview = {
  source: 'live' | 'degraded';
  message?: string;
  authenticated: boolean;
  teamID?: number;
  teamName?: string;
  displayName?: string;
  role?: string;
  challengeCount: number;
  ownServiceCount: number;
  enemyTargetCount: number;
  matchState?: string;
  schedulerState?: string;
  currentTick?: number;
  acceptingSubmissions?: boolean;
  apiBaseUrl: string;
  realtimeBaseUrl: string;
};
