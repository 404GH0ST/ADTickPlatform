export type AdminTeam = {
  id: number;
  name: string;
  contact_email: string;
  player_count: number;
  deployed_challenges: number;
};

export type AdminPlayer = {
  id: number;
  team_id: number;
  team_name: string;
  display_name: string;
  email: string;
  role: string;
  wireguard_peer: string;
  wireguard_address: string;
  wireguard_status: string;
  wireguard_issued_at: string;
  wireguard_revoked_at?: string;
  created_at: string;
};

export type AdminWireGuardPeer = {
  player_id: number;
  team_id: number;
  team_name: string;
  display_name: string;
  wireguard_peer: string;
  address: string;
  status: string;
  server_endpoint: string;
  server_public_key: string;
  client_public_key: string;
  allowed_ips: string;
  dns: string;
  config: string;
  download_name: string;
  issued_at: string;
  revoked_at?: string;
};

export type AdminWireGuardGatewayStatus = {
  state: string;
  mode: string;
  interface?: string;
  firewall_backend?: string;
  config_path?: string;
  peers_path?: string;
  rules_path?: string;
  status_path?: string;
  peers_total: number;
  peers_active: number;
  peers_revoked: number;
  revision?: string;
  applied_at?: string;
  last_error?: string;
};

export type AdminControllerAccessStatus = {
  state: string;
  mode: string;
  interface?: string;
  firewall_backend?: string;
  rules_path?: string;
  status_path?: string;
  policies_total: number;
  ssh_open_services: number;
  ssh_locked_services: number;
  allowed_peers_total: number;
  revision?: string;
  applied_at?: string;
  last_error?: string;
};

export type AdminChallenge = {
  id: number;
  name: string;
  baseline_image: string;
  checker_image: string;
  source_bundle_path: string;
  weight: number;
  service_port: number;
  service_subnet_octet: number;
  published: boolean;
  deployed_teams: number;
  total_teams: number;
  runtime_status: string;
  queued_teams: number;
  ready_teams: number;
  created_at: string;
};

export type AdminChallengeValidationResult = {
  challenge_id: number;
  name: string;
  baseline_image: string;
  checker_image: string;
  status: string;
  baseline_ssh_contract_ok: boolean;
  checker_contract_ok: boolean;
  service_state_contract_ok: boolean;
  checked_at: string;
  message?: string;
};

export type AdminDeployment = {
  job_id: number;
  challenge_id: number;
  challenge_name: string;
  status: string;
  published: boolean;
  deployed_team_count: number;
  total_team_count: number;
  queued_team_count: number;
  ready_team_count: number;
  created_at: string;
  completed_at?: string;
};

export type AdminDeploymentJob = {
  id: number;
  challenge_id: number;
  challenge_name: string;
  status: string;
  target_team_count: number;
  queued_team_count: number;
  ready_team_count: number;
  failed_team_count: number;
  created_at: string;
  completed_at?: string;
};

export type AdminReconcileResult = {
  processed_jobs: number;
  processed_instances: number;
  completed_jobs: number;
};

export type AdminGameTickStatus = {
  id: number;
  status: string;
  total_checker_runs: number;
  successful_checker_runs: number;
  failed_checker_runs: number;
  skipped_checker_runs: number;
  started_at: string;
  completed_at?: string;
  message?: string;
};

export type AdminGameSchedulerStatus = {
  state: string;
  interval_seconds: number;
  last_run_at?: string;
  next_run_at?: string;
  last_tick_id?: number;
  last_error?: string;
};

export type AdminGameMatchStatus = {
  state: string;
  started_at?: string;
  ended_at?: string;
  scheduled_start_at?: string;
  scheduled_end_at?: string;
  accepting_submissions: boolean;
};

export type AdminGameSchedulerEvent = {
  id: number;
  event_type: string;
  source: string;
  state: string;
  tick_id?: number;
  message?: string;
  created_at: string;
};

export type AdminSchedulerEventPage = {
  items: AdminGameSchedulerEvent[];
  limit: number;
  offset: number;
  total_count: number;
  has_prev: boolean;
  has_next: boolean;
};

export type AdminGameStatus = {
  current_tick?: AdminGameTickStatus;
  match?: AdminGameMatchStatus;
  scheduler?: AdminGameSchedulerStatus;
  total_ticks: number;
  total_checker_runs: number;
  successful_checker_runs: number;
  failed_checker_runs: number;
  skipped_checker_runs: number;
};

export type AdminCheckerRun = {
  id: number;
  tick_id: number;
  team_id: number;
  team_name: string;
  challenge_id: number;
  challenge_name: string;
  phase: string;
  target: string;
  checker_image: string;
  status: string;
  exit_code: number;
  message?: string;
  output?: string;
  service_state?: "ok" | "recovering" | "flag_not_found" | "faulty" | "down" | "unknown";
  state_phase?: string;
  state_message?: string;
  checked_at: string;
};

export type AdminCheckerRunPage = {
  items: AdminCheckerRun[];
  limit: number;
  offset: number;
  total_count: number;
  has_prev: boolean;
  has_next: boolean;
};

export type AdminCheckerRunQuery = {
  limit?: number;
  offset?: number;
  tick_id?: number;
  team_id?: number;
  challenge_id?: number;
  phase?: string;
  status?: string;
};

export type AdminSchedulerEventQuery = {
  limit?: number;
  offset?: number;
  event_type?: string;
  source?: string;
  state?: string;
};

export type AdminGameScoreServiceRow = {
  challenge_id: number;
  service: string;
  attack: number;
  defense: number;
  sla: number;
  total: number;
};

export type AdminGameScoreRow = {
  rank: number;
  team: string;
  attack: number;
  defense: number;
  sla: number;
  total: number;
  delta: string;
  services?: AdminGameScoreServiceRow[];
};

export type AdminAttackEvent = {
  id: string;
  attacker: string;
  victim: string;
  service: string;
  tick: number;
  verdict: string;
};

export type AdminAttackFeedPage = {
  items: AdminAttackEvent[];
  limit: number;
  offset: number;
  total_count: number;
  has_prev: boolean;
  has_next: boolean;
};

export type AdminAttackFeedQuery = {
  limit?: number;
  offset?: number;
  attacker?: string;
  victim?: string;
  service?: string;
  tick_from?: number;
  tick_to?: number;
};

export type AdminAuditLog = {
  id: number;
  actor_type: string;
  actor: string;
  action: string;
  target_type: string;
  target: string;
  status: string;
  message: string;
  metadata: string;
  created_at: string;
};

export type AdminAuditLogPage = {
  items: AdminAuditLog[];
  limit: number;
  offset: number;
  total_count: number;
  has_prev: boolean;
  has_next: boolean;
};

export type AdminAuditLogQuery = {
  limit?: number;
  offset?: number;
  actor_type?: string;
  action?: string;
  target_type?: string;
  status?: string;
};

export type AdminOverview = {
  source: 'live' | 'degraded';
  message?: string;
  operationsStatus?: AdminOperationsStatus;
  operationsAlertCount: number;
  teamCount: number;
  playerCount: number;
  challengeCount: number;
  publishedChallengeCount: number;
  deploymentCount: number;
  pendingDeploymentCount: number;
  totalTicks: number;
  scoreboardRows: number;
  apiBaseUrl: string;
};

export type AdminOperationsAlert = {
  id: string;
  severity: 'critical' | 'warning';
  source: string;
  summary: string;
  detail?: string;
};

export type AdminOperationsStatus = {
  healthy: boolean;
  generated_at: string;
  alerts: AdminOperationsAlert[];
};

export type AdminServiceMetricSnapshot = {
  generated_at: string;
  game_core: {
    match_state: string;
    total_ticks: number | null;
    checker_runs_total: number | null;
    checker_runs_failed: number | null;
    scheduler_running: boolean | null;
  };
  submission_service: {
    submit_requests_total: number | null;
    submit_failures_total: number | null;
    attack_feed_requests_total: number | null;
    verdicts: {
      correct: number | null;
      duplicate: number | null;
      invalid: number | null;
      unknown: number | null;
    };
  };
  controller_service: {
    deployment_reconcile_requests: number | null;
    access_reconcile_requests: number | null;
    service_access_reconcile_requests: number | null;
    ssh_credential_requests: number | null;
    access_policies_total: number | null;
    access_last_apply_success: boolean | null;
  };
  realtime_gateway: {
    last_sync_success: boolean | null;
    sync_errors_total: number | null;
    subscribers_total: number | null;
    snapshot_bytes_total: number | null;
  };
  wireguard_gateway: {
    reconcile_requests: number | null;
    peers_total: number | null;
    peers_active: number | null;
    peers_revoked: number | null;
    last_apply_success: boolean | null;
  };
};

export type AdminRuntimeEvidenceFailure = {
  section: string;
  message: string;
};

export type AdminRuntimeEvidenceReport = {
  generated_at: string;
  failures: AdminRuntimeEvidenceFailure[];
  summary: string;
  access_status: AdminControllerAccessStatus | null;
  deployments: AdminDeploymentJob[] | null;
  game_status: AdminGameStatus | null;
  operations_status: AdminOperationsStatus | null;
  service_metrics: AdminServiceMetricSnapshot | null;
  wireguard_status: AdminWireGuardGatewayStatus | null;
};
