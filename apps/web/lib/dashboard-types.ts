import type { ChallengeCategory, ChallengeProtocol } from "@/lib/challenge-metadata";

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
  institution?: string;
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
  /** pwn | web | crypto, or "" when the organizer has not set one. */
  category: ChallengeCategory | "";
  /** How to connect: tcp (an nc command) or http (a URL). */
  protocol: ChallengeProtocol;
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

export type AttackFeedPage = {
  revision?: number;
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
  /** The API couldn't confirm the session; the person may still be signed in. */
  sessionUnavailable?: boolean;
  /** Service data failed to load, so an empty list says nothing about the team. */
  servicesUnavailable?: boolean;
  /** The scoreboard request failed, so an empty board says nothing about the match. */
  scoresUnavailable?: boolean;
  /** The attack feed request failed, so an empty feed says nothing about the match. */
  attacksUnavailable?: boolean;
  /** The freeze request failed, so "not frozen" below is only a default. */
  freezeUnavailable?: boolean;
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
