export type ScoreServiceRow = {
  challenge_id: number;
  service: string;
  attack: number;
  defense: number;
  sla: number;
  total: number;
};

export type ScoreRow = {
  rank: number;
  team: string;
  attack: number;
  defense: number;
  sla: number;
  total: number;
  delta: string;
  services?: ScoreServiceRow[];
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
  hasSourceDownload: boolean;
  unlocked: boolean;
  sshHint: string;
  lastEvent: string;
  resetCooldown: string;
  maintenance: boolean;
  /** Why actions are locked: maintenance | match_paused | match_not_started | deferred */
  lockReason?: string;
  slaStatus: "ok" | "recovering" | "flag_not_found" | "faulty" | "down" | "unknown";
  slaPhase: string;
  slaTickId: number | null;
  slaMessage: string;
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
  playerID?: number;
  teamName?: string;
  teamContactEmail?: string;
  displayName?: string;
  email?: string;
  role?: string;
  challengeCount: number;
  ownServiceCount: number;
  enemyTargetCount: number;
  matchState?: string;
  schedulerState?: string;
  currentTick?: number;
  acceptingSubmissions?: boolean;
  nextTickAt?: string;
  tickInterval?: number;
  lastTickAt?: string;
  apiBaseUrl: string;
  realtimeBaseUrl: string;
  scoreboardFrozen?: boolean;
  scoreboardFreezeAt?: string;
  scoreboardUnfreezeAt?: string;
};
