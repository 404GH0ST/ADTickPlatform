import http from "node:http";

const port = Number(process.env.MOCK_PLATFORM_API_PORT || "4010");

const challenges = [{ id: 1, name: "college-http", has_source_download: true }];

const scoreboard = [
  {
    rank: 1,
    team: "College Alpha",
    attack: 180,
    defense: 140,
    sla: 120,
    total: 440,
    delta: "+2",
    services: [
      { challenge_id: 1, service: "Floppcraft", attack: 70, defense: 50, sla: 40, total: 160 },
      { challenge_id: 2, service: "QuickR Maps", attack: 55, defense: 45, sla: 42, total: 142 },
      { challenge_id: 3, service: "SecretChannel", attack: 55, defense: 45, sla: 38, total: 138 },
    ],
  },
  {
    rank: 2,
    team: "College Beta",
    attack: 130,
    defense: 120,
    sla: 110,
    total: 360,
    delta: "-1",
    services: [
      { challenge_id: 1, service: "Floppcraft", attack: 40, defense: 46, sla: 34, total: 120 },
      { challenge_id: 2, service: "QuickR Maps", attack: 45, defense: 38, sla: 41, total: 124 },
      { challenge_id: 3, service: "SecretChannel", attack: 45, defense: 36, sla: 35, total: 116 },
    ],
  },
  {
    rank: 3,
    team: "College Gamma",
    attack: 90,
    defense: 105,
    sla: 100,
    total: 295,
    delta: "+0",
    services: [
      { challenge_id: 1, service: "Floppcraft", attack: 30, defense: 38, sla: 32, total: 100 },
      { challenge_id: 2, service: "QuickR Maps", attack: 28, defense: 35, sla: 35, total: 98 },
      { challenge_id: 3, service: "SecretChannel", attack: 32, defense: 32, sla: 33, total: 97 },
    ],
  },
];

const attackItems = [
  {
    id: "atk-01",
    attacker: "College Alpha",
    victim: "College Beta",
    service: "college-http",
    tick: 8,
    verdict: "first valid submission accepted",
  },
  {
    id: "atk-02",
    attacker: "College Beta",
    victim: "College Gamma",
    service: "college-http",
    tick: 8,
    verdict: "first valid submission accepted",
  },
  {
    id: "atk-03",
    attacker: "College Delta",
    victim: "College Alpha",
    service: "college-http",
    tick: 9,
    verdict: "first valid submission accepted",
  },
  {
    id: "atk-04",
    attacker: "College Epsilon",
    victim: "College Zeta",
    service: "college-http",
    tick: 9,
    verdict: "first valid submission accepted",
  },
  {
    id: "atk-05",
    attacker: "College Eta",
    victim: "College Theta",
    service: "college-http",
    tick: 10,
    verdict: "first valid submission accepted",
  },
  {
    id: "atk-06",
    attacker: "College Alpha",
    victim: "College Eta",
    service: "college-http",
    tick: 10,
    verdict: "first valid submission accepted",
  },
  {
    id: "atk-07",
    attacker: "College Iota",
    victim: "College Kappa",
    service: "college-http",
    tick: 11,
    verdict: "first valid submission accepted",
  },
  {
    id: "atk-08",
    attacker: "College Lambda",
    victim: "College Mu",
    service: "college-http",
    tick: 11,
    verdict: "first valid submission accepted",
  },
  {
    id: "atk-09",
    attacker: "College Alpha",
    victim: "College Gamma",
    service: "college-http",
    tick: 12,
    verdict: "first valid submission accepted",
  },
  {
    id: "atk-10",
    attacker: "College Beta",
    victim: "College Delta",
    service: "college-http",
    tick: 12,
    verdict: "first valid submission accepted",
  },
  {
    id: "atk-11",
    attacker: "College Theta",
    victim: "College Alpha",
    service: "college-http",
    tick: 12,
    verdict: "first valid submission accepted",
  },
  {
    id: "atk-12",
    attacker: "College Nu",
    victim: "College Xi",
    service: "college-http",
    tick: 12,
    verdict: "first valid submission accepted",
  },
];

const teams = [
  {
    id: 101,
    name: "College Alpha",
    contact_email: "alpha@college.local",
    player_count: 2,
    deployed_challenges: 1,
  },
  {
    id: 102,
    name: "College Beta",
    contact_email: "beta@college.local",
    player_count: 2,
    deployed_challenges: 1,
  },
  {
    id: 103,
    name: "College Gamma",
    contact_email: "gamma@college.local",
    player_count: 1,
    deployed_challenges: 1,
  },
];

const players = [
  {
    id: 1001,
    team_id: 101,
    team_name: "College Alpha",
    display_name: "Alpha Captain",
    email: "alpha.captain@college.local",
    role: "captain",
    wireguard_peer: "wg-alpha",
    wireguard_address: "10.70.11.20/32",
    wireguard_status: "active",
    wireguard_issued_at: "2026-03-20T09:00:00Z",
    created_at: "2026-03-20T08:00:00Z",
  },
  {
    id: 1002,
    team_id: 102,
    team_name: "College Beta",
    display_name: "Beta Member",
    email: "beta.member@college.local",
    role: "member",
    wireguard_peer: "wg-beta",
    wireguard_address: "10.70.11.21/32",
    wireguard_status: "active",
    wireguard_issued_at: "2026-03-20T09:05:00Z",
    created_at: "2026-03-20T08:05:00Z",
  },
];

const adminChallenges = [
  {
    id: 1,
    name: "college-http",
    baseline_image: "adplatform/sample-http:baseline",
    checker_image: "adplatform/sample-http-checker:latest",
    source_bundle_path: "examples/sample-lfi-challenge",
    weight: 10,
    service_port: 30050,
    service_subnet_octet: 50,
    published: true,
    deployed_teams: 3,
    total_teams: 3,
    runtime_status: "ready",
    queued_teams: 0,
    ready_teams: 3,
    created_at: "2026-03-20T08:10:00Z",
  },
];

const challengeValidationResult = {
  challenge_id: 1,
  name: "college-http",
  baseline_image: "adplatform/sample-http:baseline",
  checker_image: "adplatform/sample-http-checker:latest",
  status: "valid",
  baseline_ssh_contract_ok: true,
  checker_contract_ok: true,
  checked_at: "2026-03-20T10:19:00Z",
  message: "challenge package satisfies runtime policy.",
};

const deployments = [
  {
    id: 77,
    challenge_id: 1,
    challenge_name: "college-http",
    status: "completed",
    target_team_count: 3,
    queued_team_count: 0,
    ready_team_count: 3,
    failed_team_count: 0,
    created_at: "2026-03-20T08:15:00Z",
    completed_at: "2026-03-20T08:17:00Z",
  },
];

const pendingDeployments = [
  {
    id: 88,
    challenge_id: 1,
    challenge_name: "college-http",
    status: "queued",
    target_team_count: 3,
    queued_team_count: 2,
    ready_team_count: 1,
    failed_team_count: 0,
    created_at: "2026-03-20T10:18:00Z",
    completed_at: "",
  },
];

const checkerRuns = [
  {
    id: 9001,
    tick_id: 12,
    team_id: 101,
    team_name: "College Alpha",
    challenge_id: 1,
    challenge_name: "college-http",
    phase: "check",
    target: "10.80.50.11:30050",
    checker_image: "adplatform/sample-http-checker:latest",
    status: "success",
    exit_code: 0,
    service_state: "ok",
    state_phase: "check",
    state_message: "service passed storage, retrieval, and functionality checks",
    checked_at: "2026-03-20T10:12:10Z",
    message: "checker phase completed successfully.",
  },
  {
    id: 9002,
    tick_id: 12,
    team_id: 102,
    team_name: "College Beta",
    challenge_id: 1,
    challenge_name: "college-http",
    phase: "check",
    target: "10.80.50.12:30050",
    checker_image: "adplatform/sample-http-checker:latest",
    status: "success",
    exit_code: 0,
    service_state: "ok",
    state_phase: "check",
    state_message: "service passed storage, retrieval, and functionality checks",
    checked_at: "2026-03-20T10:12:12Z",
    message: "checker phase completed successfully.",
  },
];

const auditLogs = [
  {
    id: 301,
    actor_type: "team",
    actor: "College Alpha",
    action: "service.unlock",
    target_type: "service",
    target: "team:101 challenge:1",
    status: "success",
    message: "unlocked service",
    metadata: '{"challenge_id":1,"team_id":101}',
    created_at: "2026-03-20T10:18:30Z",
  },
  {
    id: 302,
    actor_type: "team",
    actor: "College Beta",
    action: "service.unlock",
    target_type: "service",
    target: "team:102 challenge:1",
    status: "success",
    message: "unlocked service",
    metadata: '{"challenge_id":1,"team_id":102}',
    created_at: "2026-03-20T10:17:30Z",
  },
  {
    id: 303,
    actor_type: "admin",
    actor: "Organizer",
    action: "challenge.deploy",
    target_type: "challenge",
    target: "challenge:1 college-http",
    status: "success",
    message: "queued challenge deployment",
    metadata: '{"challenge_id":1}',
    created_at: "2026-03-20T10:16:30Z",
  },
];

const wireguardStatus = {
  state: "applied",
  mode: "host",
  interface: "wg0",
  firewall_backend: "nftables",
  peers_total: 2,
  peers_active: 2,
  peers_revoked: 0,
  revision: "mock-wireguard-revision",
  applied_at: "2026-03-20T10:16:00Z",
};

const accessStatus = {
  state: "applied",
  mode: "host",
  interface: "wg0",
  firewall_backend: "iptables",
  policies_total: 1,
  ssh_open_services: 1,
  ssh_locked_services: 0,
  allowed_peers_total: 2,
  revision: "mock-access-revision",
  applied_at: "2026-03-20T10:16:00Z",
};

const operationsStatus = {
  healthy: true,
  generated_at: "2026-03-20T10:20:00Z",
  alerts: [],
};

function buildWireGuardPeer(player) {
  const suffix = player.id;
  return {
    player_id: player.id,
    team_id: player.team_id,
    team_name: player.team_name,
    display_name: player.display_name,
    wireguard_peer: player.wireguard_peer,
    address: player.wireguard_address,
    status: player.wireguard_status,
    server_endpoint: "vpn.college.local:51820",
    server_public_key: "server-public-key",
    client_public_key: `client-public-key-${suffix}`,
    allowed_ips: "10.70.0.0/16",
    dns: "10.70.0.1",
    config: `[Interface]
PrivateKey = mock-private-key-${suffix}
Address = ${player.wireguard_address}
DNS = 10.70.0.1

[Peer]
PublicKey = server-public-key
Endpoint = vpn.college.local:51820
AllowedIPs = 10.70.0.0/16
`,
    download_name: `${player.display_name.toLowerCase().replace(/\s+/g, "-")}.conf`,
    issued_at: player.wireguard_issued_at,
    revoked_at: player.wireguard_revoked_at,
  };
}

function encodeBase64Url(value) {
  return Buffer.from(value, "utf8")
    .toString("base64")
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/g, "");
}

function buildParticipantToken(player) {
  const header = encodeBase64Url(JSON.stringify({ alg: "none", typ: "JWT" }));
  const payload = encodeBase64Url(
    JSON.stringify({
      team_id: player.team_id,
      player_id: player.id,
      team_name: player.team_name,
      display_name: player.display_name,
      email: player.email,
      role: player.role,
    }),
  );
  return `${header}.${payload}.`;
}

function createInitialState() {
  return {
    failedRoutes: [],
    streamScenario: "default",
    attackItems: attackItems.map((item) => ({ ...item })),
    gameStatus: {
      current_tick: {
        id: 12,
        status: "completed",
        total_checker_runs: 6,
        successful_checker_runs: 6,
        failed_checker_runs: 0,
        skipped_checker_runs: 0,
        started_at: "2026-03-20T10:12:00Z",
        completed_at: "2026-03-20T10:12:15Z",
      },
      match: {
        state: "finished",
        started_at: "2026-03-20T10:00:00Z",
        ended_at: "2026-03-20T10:15:00Z",
        scheduled_start_at: "2026-03-20T10:00:00Z",
        scheduled_end_at: "2026-03-20T10:15:00Z",
        accepting_submissions: false,
      },
      scheduler: {
        state: "stopped",
        interval_seconds: 60,
        last_run_at: "2026-03-20T10:12:15Z",
        next_run_at: "",
        last_tick_id: 12,
      },
      total_ticks: 12,
      total_checker_runs: 6,
      successful_checker_runs: 6,
      failed_checker_runs: 0,
      skipped_checker_runs: 0,
    },
    schedulerEvents: [
      {
        id: 1,
        event_type: "started",
        source: "organizer",
        state: "running",
        created_at: "2026-03-20T10:00:00Z",
        message: "scheduler started",
      },
      {
        id: 2,
        event_type: "tick_completed",
        source: "scheduler",
        state: "running",
        tick_id: 12,
        created_at: "2026-03-20T10:12:00Z",
        message: "tick #12 completed",
      },
      {
        id: 3,
        event_type: "stopped",
        source: "organizer",
        state: "stopped",
        created_at: "2026-03-20T10:15:00Z",
        message: "scheduler stopped after match end",
      },
    ],
    serviceMap: {
      "1": {
        "101": ["10.80.50.11:30050"],
        "102": ["10.80.50.12:30050"],
        "103": ["10.80.50.13:30050"],
      },
    },
    teamServiceStates: [
      {
        challenge_id: 1,
        team_id: 101,
        name: "college-http",
        endpoint: "10.80.50.11:30050",
        status: "stable",
        checker: "passing",
        unlocked: false,
        ssh_hint: "unlock required before requesting root access",
        last_event: "service stable",
        reset_cooldown: "ready",
        sla_status: "ok",
        sla_phase: "check",
        sla_tick_id: 12,
        sla_message: "service passed storage, retrieval, and functionality checks",
      },
    ],
    teams: teams.map((team) => ({ ...team })),
    players: players.map((player) => ({ ...player })),
    challenges: adminChallenges.map((challenge) => ({ ...challenge })),
    deployments: deployments.map((deployment) => ({ ...deployment })),
    checkerRuns: checkerRuns.map((run) => ({ ...run })),
    scoreboard: scoreboard.map((row) => ({ ...row })),
    wireguardStatus: { ...wireguardStatus },
    accessStatus: { ...accessStatus },
    operationsStatus: { ...operationsStatus },
    serviceMetrics: {
      game_core: {
        match: null,
        scheduler: null,
        total_ticks: 12,
        total_checker_runs: 72,
        failed_checker_runs: 0,
      },
      submission_service: {
        submit_requests_total: 25,
        submit_failures_total: 0,
        attack_feed_requests_total: 47,
        verdicts: {
          correct: 25,
          duplicate: 0,
          invalid: 0,
          unknown: 0,
        },
      },
      controller_service: {
        deployment_reconcile_requests: 1,
        access_reconcile_requests: 3,
        service_access_reconcile_requests: 1,
        ssh_credential_requests: 0,
        access_policies_total: 12,
        access_last_apply_success: true,
      },
      realtime_gateway: {
        last_sync_success: true,
        sync_errors_total: 0,
        subscribers_total: 0,
        snapshot_bytes_total: 2048,
      },
      wireguard_gateway: {
        reconcile_requests: 2,
        peers_total: 12,
        peers_active: 12,
        peers_revoked: 0,
        last_apply_success: true,
      },
    },
    sshPasswordCounter: 1,
    nextTeamID: 104,
    nextPlayerID: 1003,
    nextChallengeID: 2,
    nextDeploymentJobID: 89,
    nextSchedulerEventID: 4,
    deploymentReconcileProblem: null,
  };
}

function createStateForScenario(scenario = "default") {
  const nextState = createInitialState();

  if (scenario === "prestart") {
    nextState.gameStatus.match = {
      ...nextState.gameStatus.match,
      state: "stopped",
      started_at: "",
      ended_at: "",
      accepting_submissions: false,
    };
    nextState.gameStatus.scheduler = {
      ...nextState.gameStatus.scheduler,
      state: "stopped",
      next_run_at: "",
    };
    nextState.schedulerEvents = [
      {
        id: 1,
        event_type: "stopped",
        source: "organizer",
        state: "stopped",
        created_at: "2026-03-20T09:55:00Z",
        message: "scheduler is ready for organizer start",
      },
    ];
    nextState.nextSchedulerEventID = 2;
  }

  if (scenario === "stopped-running-scheduler") {
    nextState.gameStatus.match = {
      ...nextState.gameStatus.match,
      state: "stopped",
      accepting_submissions: false,
    };
    nextState.gameStatus.scheduler = {
      ...nextState.gameStatus.scheduler,
      state: "running",
      next_run_at: "2026-03-20T10:21:00Z",
    };
    nextState.schedulerEvents = [
      {
        id: 1,
        event_type: "match_stopped",
        source: "organizer",
        state: "stopped",
        created_at: "2026-03-20T10:20:00Z",
        message: "match stopped while scheduler was still active",
      },
      {
        id: 2,
        event_type: "started",
        source: "scheduler",
        state: "running",
        created_at: "2026-03-20T10:20:10Z",
        message: "scheduler still reporting as running",
      },
    ];
    nextState.nextSchedulerEventID = 3;
  }

  if (scenario === "pending-deployments") {
    nextState.deployments = pendingDeployments.map((deployment) => ({
      ...deployment,
    }));
    nextState.challenges = nextState.challenges.map((challenge) =>
      challenge.id === 1
        ? {
            ...challenge,
            runtime_status: "deploying",
            queued_teams: 2,
            ready_teams: 1,
          }
        : challenge,
    );
  }

  if (scenario === "deployment-reconcile-access-failure") {
    nextState.deployments = pendingDeployments.map((deployment) => ({
      ...deployment,
    }));
    nextState.deploymentReconcileProblem = {
      statusCode: 502,
      title: "Deployment reconcile failed",
      detail:
        "runtime converge completed but controller access reconcile failed, so host access truth was not established.",
    };
  }

  if (scenario === "redeploy") {
    nextState.deployments = pendingDeployments.map((deployment) => ({
      ...deployment,
    }));
    nextState.challenges = nextState.challenges.map((challenge) =>
      challenge.id === 1
        ? {
            ...challenge,
            published: true,
            deployed_teams: 3,
            total_teams: 3,
            runtime_status: "deploying",
            queued_teams: 2,
            ready_teams: 1,
          }
        : challenge,
    );
  }

  if (scenario === "degraded-participant") {
    nextState.failedRoutes = [
      "GET /api/v2/scoreboard",
      "GET /api/v2/attacks",
    ];
  }

  if (scenario === "degraded-service-states") {
    nextState.failedRoutes = ["GET /api/v2/team/services"];
  }

  if (scenario === "degraded-participant-attacks") {
    nextState.failedRoutes = [
      "GET /api/v2/attacks",
      "GET /public/v1/attacks/stream",
    ];
  }

  if (scenario === "degraded-admin") {
    nextState.failedRoutes = [
      "GET /api/v2/admin/game/scoreboard",
      "GET /api/v2/attacks",
      "GET /admin/v1/game/scoreboard/stream",
    ];
  }

  if (scenario === "ops-alerts") {
    nextState.operationsStatus = {
      healthy: false,
      generated_at: "2026-03-20T10:20:00Z",
      alerts: [
        {
          id: "scheduler-overdue",
          severity: "critical",
          source: "scheduler",
          summary: "Scheduler next run is overdue.",
          detail: "Expected the next run around 2026-03-20T10:18:00Z with a 30s interval.",
        },
        {
          id: "deployment-queue-stalled",
          severity: "warning",
          source: "deployments",
          summary: "2 deployment job(s) are still active.",
          detail: "The oldest active job has been 14m in status queued.",
        },
      ],
    };
  }

  if (scenario === "runtime-health-drift") {
    nextState.deployments = pendingDeployments.map((deployment) => ({
      ...deployment,
    }));
    nextState.operationsStatus = {
      healthy: false,
      generated_at: "2026-03-20T10:24:00Z",
      alerts: [
        {
          id: "deployment-queue-stalled",
          severity: "warning",
          source: "deployments",
          summary: "2 deployment job(s) are still active.",
          detail: "The oldest active job has been 14m in status queued.",
        },
      ],
    };
    nextState.accessStatus = {
      ...nextState.accessStatus,
      state: "idle",
      applied_at: "2026-03-20T10:10:00Z",
    };
    nextState.wireguardStatus = {
      ...nextState.wireguardStatus,
      state: "idle",
      applied_at: "2026-03-20T10:10:00Z",
    };
    nextState.serviceMetrics = {
      ...nextState.serviceMetrics,
      controller_service: {
        ...nextState.serviceMetrics.controller_service,
        access_last_apply_success: false,
      },
      wireguard_gateway: {
        ...nextState.serviceMetrics.wireguard_gateway,
        last_apply_success: false,
      },
    };
  }

  if (scenario === "runtime-report-partial") {
    nextState.failedRoutes = ["GET /api/v2/admin/wireguard/status"];
  }

  if (scenario === "metrics-attention") {
    nextState.gameStatus.match = {
      ...nextState.gameStatus.match,
      state: "running",
      accepting_submissions: true,
      ended_at: "",
    };
    nextState.gameStatus.scheduler = {
      ...nextState.gameStatus.scheduler,
      state: "stopped",
      next_run_at: "",
    };
    nextState.serviceMetrics.game_core.failed_checker_runs = 2;
    nextState.serviceMetrics.submission_service.submit_failures_total = 3;
    nextState.serviceMetrics.realtime_gateway.last_sync_success = false;
    nextState.serviceMetrics.realtime_gateway.sync_errors_total = 4;
    nextState.serviceMetrics.controller_service.access_last_apply_success = false;
    nextState.serviceMetrics.wireguard_gateway.last_apply_success = false;
  }

  if (scenario === "degraded-admin-attacks") {
    nextState.failedRoutes = [
      "GET /api/v2/attacks",
      "GET /public/v1/attacks/stream",
    ];
  }

  if (scenario === "degraded-game-history") {
    nextState.failedRoutes = [
      "GET /api/v2/admin/game/scheduler/events",
      "GET /api/v2/admin/game/checker-runs",
      "GET /admin/v1/game/scheduler/events/stream",
      "GET /admin/v1/game/checker-runs/stream",
    ];
  }

  if (scenario === "degraded-participant-scoreboard") {
    nextState.failedRoutes = [
      "GET /api/v2/scoreboard",
      "GET /public/v1/scoreboard/stream",
    ];
  }

  if (scenario === "realtime-updates") {
    nextState.streamScenario = "realtime-updates";
  }

  if (scenario === "empty-attacks") {
    nextState.attackItems = [];
  }

  if (scenario === "empty-services") {
    nextState.serviceMap = {};
    nextState.teamServiceStates = [];
  }

  if (scenario === "empty-scoreboard") {
    nextState.scoreboard = [];
  }

  if (scenario === "empty-game-history") {
    nextState.schedulerEvents = [];
    nextState.checkerRuns = [];
    nextState.nextSchedulerEventID = 1;
  }

  nextState.serviceMetrics.game_core.match = nextState.gameStatus.match;
  nextState.serviceMetrics.game_core.scheduler = nextState.gameStatus.scheduler;

  return nextState;
}

let state = createStateForScenario();
/** @type {Set<import("http").ServerResponse>} */
const activeSSEConnections = new Set();

function success(data) {
  return JSON.stringify(data);
}

function nowIso() {
  return "2026-03-20T10:20:00Z";
}

function nextRunAt(intervalSeconds) {
  return `2026-03-20T10:${String(intervalSeconds).padStart(2, "0")}:00Z`;
}

function parseInteger(value, fallback) {
  const parsed = Number.parseInt(value ?? "", 10);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function paginate(items, searchParams) {
  const limit = Math.max(1, parseInteger(searchParams.get("limit"), 12));
  const offset = Math.max(0, parseInteger(searchParams.get("offset"), 0));
  const pagedItems = items.slice(offset, offset + limit);

  return {
    items: pagedItems,
    limit,
    offset,
    total_count: items.length,
    has_prev: offset > 0,
    has_next: offset + limit < items.length,
  };
}

function filterAttackItems(items, searchParams) {
  const attacker = (searchParams.get("attacker") || "").trim().toLowerCase();
  const victim = (searchParams.get("victim") || "").trim().toLowerCase();
  const service = (searchParams.get("service") || "").trim().toLowerCase();
  const tickFrom = parseInteger(searchParams.get("tick_from"), 0);
  const tickTo = parseInteger(
    searchParams.get("tick_to"),
    Number.MAX_SAFE_INTEGER,
  );

  return items.filter((item) => {
    if (attacker && !item.attacker.toLowerCase().includes(attacker)) {
      return false;
    }
    if (victim && !item.victim.toLowerCase().includes(victim)) {
      return false;
    }
    if (service && !item.service.toLowerCase().includes(service)) {
      return false;
    }
    return item.tick >= tickFrom && item.tick <= tickTo;
  });
}

function filterSchedulerEvents(items, searchParams) {
  const eventType = (searchParams.get("event_type") || "").trim().toLowerCase();
  const source = (searchParams.get("source") || "").trim().toLowerCase();
  const routeState = (searchParams.get("state") || "").trim().toLowerCase();

  return items.filter((item) => {
    if (eventType && !item.event_type.toLowerCase().includes(eventType)) {
      return false;
    }
    if (source && !item.source.toLowerCase().includes(source)) {
      return false;
    }
    if (routeState && !item.state.toLowerCase().includes(routeState)) {
      return false;
    }
    return true;
  });
}

function filterCheckerRuns(items, searchParams) {
  const tickId = parseInteger(searchParams.get("tick_id"), 0);
  const teamId = parseInteger(searchParams.get("team_id"), 0);
  const challengeId = parseInteger(searchParams.get("challenge_id"), 0);
  const phase = (searchParams.get("phase") || "").trim().toLowerCase();
  const routeStatus = (searchParams.get("status") || "").trim().toLowerCase();

  return items.filter((item) => {
    if (tickId > 0 && item.tick_id !== tickId) {
      return false;
    }
    if (teamId > 0 && item.team_id !== teamId) {
      return false;
    }
    if (challengeId > 0 && item.challenge_id !== challengeId) {
      return false;
    }
    if (phase && !item.phase.toLowerCase().includes(phase)) {
      return false;
    }
    if (routeStatus && !item.status.toLowerCase().includes(routeStatus)) {
      return false;
    }
    return true;
  });
}

function filterAuditLogs(items, searchParams) {
  const actorType = (searchParams.get("actor_type") || "").trim().toLowerCase();
  const action = (searchParams.get("action") || "").trim().toLowerCase();
  const targetType = (searchParams.get("target_type") || "")
    .trim()
    .toLowerCase();
  const routeStatus = (searchParams.get("status") || "").trim().toLowerCase();

  return items.filter((item) => {
    if (actorType && !item.actor_type.toLowerCase().includes(actorType)) {
      return false;
    }
    if (action && !item.action.toLowerCase().includes(action)) {
      return false;
    }
    if (targetType && !item.target_type.toLowerCase().includes(targetType)) {
      return false;
    }
    if (routeStatus && !item.status.toLowerCase().includes(routeStatus)) {
      return false;
    }
    return true;
  });
}

function writeJson(res, statusCode, payload) {
  res.writeHead(statusCode, { "Content-Type": "application/json" });
  res.end(payload);
}

function writeText(res, statusCode, payload, contentType = "text/plain; charset=utf-8") {
  res.writeHead(statusCode, { "Content-Type": contentType });
  res.end(payload);
}

function metricBool(value) {
  return value ? 1 : 0;
}

function renderGameCoreMetrics(snapshot) {
  const matchState = snapshot.match?.state || "unknown";
  const states = ["not_started", "running", "finished", "stopped", "unknown"];
  return `${[
    "# TYPE adplatform_game_core_match_state gauge",
    ...states.map((state) =>
      `adplatform_game_core_match_state{state="${state}"} ${metricBool(matchState === state)}`,
    ),
    "# TYPE adplatform_game_core_total_ticks gauge",
    `adplatform_game_core_total_ticks ${snapshot.total_ticks}`,
    "# TYPE adplatform_game_core_checker_runs_total counter",
    `adplatform_game_core_checker_runs_total{status="all"} ${snapshot.total_checker_runs}`,
    `adplatform_game_core_checker_runs_total{status="failed"} ${snapshot.failed_checker_runs}`,
    "# TYPE adplatform_game_core_scheduler_running gauge",
    `adplatform_game_core_scheduler_running ${metricBool(snapshot.scheduler?.state === "running")}`,
  ].join("\n")}\n`;
}

function renderSubmissionMetrics(snapshot) {
  return `${[
    "# TYPE adplatform_submission_service_submit_requests_total counter",
    `adplatform_submission_service_submit_requests_total ${snapshot.submit_requests_total}`,
    "# TYPE adplatform_submission_service_submit_failures_total counter",
    `adplatform_submission_service_submit_failures_total ${snapshot.submit_failures_total}`,
    "# TYPE adplatform_submission_service_attack_feed_requests_total counter",
    `adplatform_submission_service_attack_feed_requests_total ${snapshot.attack_feed_requests_total}`,
    "# TYPE adplatform_submission_service_submit_verdicts_total counter",
    `adplatform_submission_service_submit_verdicts_total{class="correct"} ${snapshot.verdicts.correct}`,
    `adplatform_submission_service_submit_verdicts_total{class="duplicate"} ${snapshot.verdicts.duplicate}`,
    `adplatform_submission_service_submit_verdicts_total{class="invalid"} ${snapshot.verdicts.invalid}`,
    `adplatform_submission_service_submit_verdicts_total{class="unknown"} ${snapshot.verdicts.unknown}`,
  ].join("\n")}\n`;
}

function renderControllerMetrics(snapshot) {
  return `${[
    "# TYPE adplatform_controller_service_operation_requests_total counter",
    `adplatform_controller_service_operation_requests_total{operation="deployment_reconcile"} ${snapshot.deployment_reconcile_requests}`,
    `adplatform_controller_service_operation_requests_total{operation="access_reconcile"} ${snapshot.access_reconcile_requests}`,
    `adplatform_controller_service_operation_requests_total{operation="service_access_reconcile"} ${snapshot.service_access_reconcile_requests}`,
    `adplatform_controller_service_operation_requests_total{operation="ssh_credential"} ${snapshot.ssh_credential_requests}`,
    "# TYPE adplatform_controller_service_access_policies_total gauge",
    `adplatform_controller_service_access_policies_total ${snapshot.access_policies_total}`,
    "# TYPE adplatform_controller_service_access_last_apply_success gauge",
    `adplatform_controller_service_access_last_apply_success ${metricBool(snapshot.access_last_apply_success)}`,
  ].join("\n")}\n`;
}

function renderRealtimeMetrics(snapshot) {
  return `${[
    "# TYPE adplatform_realtime_gateway_last_sync_success gauge",
    `adplatform_realtime_gateway_last_sync_success ${metricBool(snapshot.last_sync_success)}`,
    "# TYPE adplatform_realtime_gateway_sync_errors_total counter",
    `adplatform_realtime_gateway_sync_errors_total ${snapshot.sync_errors_total}`,
    "# TYPE adplatform_realtime_gateway_subscribers gauge",
    `adplatform_realtime_gateway_subscribers{stream="scoreboard"} ${snapshot.subscribers_total}`,
    "# TYPE adplatform_realtime_gateway_snapshot_bytes gauge",
    `adplatform_realtime_gateway_snapshot_bytes{stream="scoreboard"} ${snapshot.snapshot_bytes_total}`,
  ].join("\n")}\n`;
}

function renderWireguardMetrics(snapshot) {
  return `${[
    "# TYPE adplatform_wireguard_gateway_operation_requests_total counter",
    `adplatform_wireguard_gateway_operation_requests_total{operation="reconcile"} ${snapshot.reconcile_requests}`,
    "# TYPE adplatform_wireguard_gateway_peer_counts gauge",
    `adplatform_wireguard_gateway_peer_counts{status="total"} ${snapshot.peers_total}`,
    `adplatform_wireguard_gateway_peer_counts{status="active"} ${snapshot.peers_active}`,
    `adplatform_wireguard_gateway_peer_counts{status="revoked"} ${snapshot.peers_revoked}`,
    "# TYPE adplatform_wireguard_gateway_last_apply_success gauge",
    `adplatform_wireguard_gateway_last_apply_success ${metricBool(snapshot.last_apply_success)}`,
  ].join("\n")}\n`;
}

function writeSSE(res, data) {
  res.writeHead(200, {
    "Content-Type": "text/event-stream",
    "Cache-Control": "no-cache, no-transform",
    Connection: "keep-alive",
  });
  res.write(`data: ${JSON.stringify(data)}\n\n`);
  const heartbeat = setInterval(() => {
    res.write(": keep-alive\n\n");
  }, 15_000);

  activeSSEConnections.add(res);
  res.on("close", () => {
    clearInterval(heartbeat);
    activeSSEConnections.delete(res);
  });
}

function makeRealtimeAttackPage() {
  const items = [
    {
      id: "atk-13",
      attacker: "College Gamma",
      victim: "College Alpha",
      service: "college-http",
      tick: 13,
      verdict: "first valid submission accepted",
    },
    ...state.attackItems,
  ];

  return paginate(items, new URLSearchParams("limit=12&offset=0"));
}

function makeRealtimeScoreboard() {
  return state.scoreboard.map((row, index) =>
    index === 0 ? { ...row, total: row.total + 15, delta: "+3" } : row,
  );
}

function makeRealtimeGameStatus() {
  return {
    ...state.gameStatus,
    current_tick: {
      id: 13,
      status: "completed",
      total_checker_runs: 6,
      successful_checker_runs: 6,
      failed_checker_runs: 0,
      skipped_checker_runs: 0,
      started_at: "2026-03-20T10:13:00Z",
      completed_at: "2026-03-20T10:13:12Z",
      message: "tick #13 completed",
    },
    total_ticks: 13,
    total_checker_runs: (state.gameStatus.total_checker_runs ?? 0) + 6,
    successful_checker_runs:
      (state.gameStatus.successful_checker_runs ?? 0) + 6,
  };
}

function writeScenarioSSE(res, pathname, initialData) {
  writeSSE(res, initialData);

  if (state.streamScenario !== "realtime-updates") {
    return;
  }

  let followUpData = null;
  if (pathname === "/public/v1/attacks/stream") {
    followUpData = makeRealtimeAttackPage();
  } else if (pathname === "/public/v1/scoreboard/stream") {
    followUpData = makeRealtimeScoreboard();
  } else if (pathname === "/admin/v1/game/scoreboard/stream") {
    followUpData = makeRealtimeScoreboard();
  } else if (pathname === "/admin/v1/game/status/stream") {
    followUpData = makeRealtimeGameStatus();
  }

  if (!followUpData) {
    return;
  }

  const timer = setTimeout(() => {
    res.write(`data: ${JSON.stringify(followUpData)}\n\n`);
  }, 250);

  res.on("close", () => {
    clearTimeout(timer);
  });
}

async function readJsonBody(req) {
  const chunks = [];
  for await (const chunk of req) {
    chunks.push(Buffer.from(chunk));
  }

  if (chunks.length === 0) {
    return null;
  }

  try {
    return JSON.parse(Buffer.concat(chunks).toString("utf8"));
  } catch {
    return null;
  }
}

function addSchedulerEvent(event_type, source, routeState, message, tick_id) {
  state.schedulerEvents.unshift({
    id: state.nextSchedulerEventID,
    event_type,
    source,
    state: routeState,
    created_at: nowIso(),
    message,
    ...(typeof tick_id === "number" ? { tick_id } : {}),
  });
  state.nextSchedulerEventID += 1;
}

function findServiceState(challengeID) {
  return (
    state.teamServiceStates.find((entry) => entry.challenge_id === challengeID) ??
    null
  );
}

function shouldFailRoute(method, pathname) {
  return state.failedRoutes.includes(`${method} ${pathname}`);
}

function writeSuccess(res, data) {
  writeJson(res, 200, success(data));
  return true;
}

function writeFailure(res, statusCode, message, status = "failed") {
  const title =
    status === "forbidden"
      ? "Forbidden"
      : status === "too many request"
        ? "Too many requests"
        : "Request failed";
  writeJson(
    res,
    statusCode,
    JSON.stringify({ title, status: statusCode, detail: message }),
  );
  return true;
}

function routeID(pathname, pattern) {
  const match = pathname.match(pattern);
  return match ? Number(match[1]) : null;
}

function writeGetRoute(res, method, pathname, routes) {
  if (method !== "GET") {
    return false;
  }
  const handler = routes[pathname];
  if (!handler) {
    return false;
  }
  return writeSuccess(res, handler());
}

function writeTextRoute(res, method, pathname, routes) {
  if (method !== "GET") {
    return false;
  }
  const handler = routes[pathname];
  if (!handler) {
    return false;
  }
  writeText(res, 200, handler());
  return true;
}

function writeStreamRoute(res, method, pathname, routes) {
  if (method !== "GET") {
    return false;
  }
  const handler = routes[pathname];
  if (!handler) {
    return false;
  }
  handler(res, pathname);
  return true;
}

const requestHandlers = [
  handleSystemRoutes,
  handleAuthenticationRoutes,
  handleParticipantRoutes,
  handleAdminTeamRoutes,
  handleAdminPlayerRoutes,
  handleAdminChallengeRoutes,
  handleAdminDeploymentRoutes,
  handleAdminMetricsRoutes,
  handleAdminGameRoutes,
  handleAdminNetworkRoutes,
  handleStreamRoutes,
];

const server = http.createServer((req, res) => {
  void handleRequest(req, res);
});

async function handleRequest(req, res) {
  const url = new URL(req.url || "/", `http://127.0.0.1:${port}`);
  const ctx = { req, res, url, method: req.method ?? "GET" };

  for (const handler of requestHandlers) {
    if (await handler(ctx)) {
      return;
    }
  }

  return writeFailure(res, 404, `mock route not found: ${url.pathname}`);
}

async function handleSystemRoutes({ req, res, url, method }) {
  if (method === "POST" && url.pathname === "/__reset") {
    const body = await readJsonBody(req);
    state = createStateForScenario(body?.scenario);
    // Close all active SSE connections so Next.js re-fetches fresh data
    for (const conn of activeSSEConnections) {
      conn.end();
    }
    activeSSEConnections.clear();
    return writeSuccess(res, { reset: true });
  }

  if (shouldFailRoute(method, url.pathname)) {
    return writeFailure(res, 503, `mock degraded route: ${url.pathname}`);
  }

  return false;
}

async function handleAuthenticationRoutes({ req, res, url, method }) {
  if (method !== "POST" || url.pathname !== "/api/v2/authenticate") {
    return false;
  }

  const body = await readJsonBody(req);
  const email = body?.email?.trim();
  const password = body?.password?.trim();
  const knownPasswords = {
    "alpha.captain@college.local": "alpha-password",
    "beta.member@college.local": "beta-password",
  };
  const player = state.players.find((item) => item.email === email);

  if (!player || !password || knownPasswords[email] !== password) {
    return writeFailure(res, 403, "email or password is wrong.", "forbidden");
  }

  return writeSuccess(res, {
    token: buildParticipantToken(player),
    token_type: "Bearer",
  });
}

async function handleParticipantRoutes(ctx) {
  const { res, url, method } = ctx;

  const handled = writeGetRoute(res, method, url.pathname, {
    "/api/v2/challenges": () => challenges,
    "/api/v2/scoreboard": () => state.scoreboard,
    "/api/v2/game/status": () => state.gameStatus,
    "/api/v2/services": () => state.serviceMap,
    "/api/v2/team/services": () => state.teamServiceStates,
  });
  if (handled) {
    return true;
  }

  if (method === "GET" && url.pathname === "/api/v2/attacks") {
    const filtered = filterAttackItems(state.attackItems, url.searchParams);
    return writeSuccess(res, paginate(filtered, url.searchParams));
  }

  return (
    handleChallengeSourceDownload(ctx) ||
    (await handleServiceUnlock(ctx)) ||
    handleSSHSession(ctx) ||
    handleServiceRestart(ctx) ||
    handleFactoryReset(ctx)
  );
}

function handleChallengeSourceDownload({ res, url, method }) {
  const challengeID = routeID(
    url.pathname,
    /^\/api\/v2\/challenges\/(\d+)\/source$/,
  );
  if (method !== "GET" || challengeID === null) {
    return false;
  }
  res.writeHead(200, {
    "Content-Type": "application/gzip",
    "Content-Disposition": 'attachment; filename="college-http-source.tar.gz"',
  });
  res.end("mock-source-bundle");
  return true;
}

async function handleServiceUnlock({ req, res, url, method }) {
  const challengeID = routeID(url.pathname, /^\/api\/v2\/services\/(\d+)\/unlock$/);
  if (method !== "POST" || challengeID === null) {
    return false;
  }

  const serviceState = findServiceState(challengeID);
  const body = await readJsonBody(req);
  if (!serviceState || !body?.proof) {
    return writeFailure(res, 400, "unlock proof is invalid.");
  }

  serviceState.unlocked = true;
  serviceState.ssh_hint =
    "unlock accepted; use SSH Access to view the team credential";
  serviceState.last_event = "unlock granted via participant API";

  return writeSuccess(res, {
    challenge_id: challengeID,
    team_id: serviceState.team_id,
    unlocked: true,
  });
}

function serviceActionRoute(pattern, handler) {
  return function ({ req, res, url, method }) {
    const challengeID = routeID(url.pathname, pattern);
    if (method !== "POST" || challengeID === null) {
      return false;
    }
    const serviceState = findServiceState(challengeID);
    if (!serviceState) {
      return writeFailure(res, 404, "service not found.");
    }
    return handler({ res, req, challengeID, serviceState });
  };
}

const handleSSHSession = serviceActionRoute(
  /^\/api\/v2\/services\/(\d+)\/ssh-session$/,
  ({ res, challengeID, serviceState }) => {
    const password = "Adp-team-credential-Aa1!";
    serviceState.unlocked = true;
    serviceState.ssh_hint = "ssh root@10.80.50.11 -p 22";
    serviceState.last_event = "team ssh credential retrieved";

    return writeSuccess(res, {
      challenge_id: challengeID,
      host: "10.80.50.11",
      port: 22,
      username: "root",
      password,
      password_mode: "stable",
      connection_hint: "ssh root@10.80.50.11 -p 22",
    });
  },
);

const handleServiceRestart = serviceActionRoute(
  /^\/api\/v2\/services\/(\d+)\/reset\/restart$/,
  ({ res, challengeID, serviceState }) => {
    serviceState.status = "warming";
    serviceState.checker = "warning";
    serviceState.last_event = "service restart triggered via participant API";
    serviceState.reset_cooldown = "restart requested";

    return writeSuccess(res, {
      challenge_id: challengeID,
      team_id: serviceState.team_id,
      action: "restart",
    });
  },
);

const handleFactoryReset = serviceActionRoute(
  /^\/api\/v2\/services\/(\d+)\/reset\/factory$/,
  ({ res, challengeID, serviceState }) => {
    serviceState.status = "warming";
    serviceState.checker = "warning";
    serviceState.unlocked = true;
    serviceState.ssh_hint =
      "unlock preserved; open SSH Access to reapply the team credential";
    serviceState.last_event = "factory reset triggered via participant API";
    serviceState.reset_cooldown = "cooldown: 90s";

    return writeSuccess(res, {
      challenge_id: challengeID,
      team_id: serviceState.team_id,
      action: "factory_reset",
      unlock_preserved: true,
    });
  },
);

async function handleAdminTeamRoutes({ req, res, url, method }) {
  if (url.pathname === "/api/v2/admin/teams") {
    if (method === "GET") {
      return writeSuccess(res, state.teams);
    }
    if (method !== "POST") {
      return false;
    }
    const body = await readJsonBody(req);
    const name = body?.name?.trim();
    const contactEmail = body?.contact_email?.trim();

    if (!name || !contactEmail) {
      return writeFailure(res, 400, "team request is invalid.");
    }

    const publishedChallenges = state.challenges.filter(
      (challenge) => challenge.published,
    ).length;
    const team = {
      id: state.nextTeamID,
      name,
      contact_email: contactEmail,
      player_count: 0,
      deployed_challenges: publishedChallenges,
    };
    state.nextTeamID += 1;
    state.teams.push(team);

    return writeSuccess(res, team);
  }

  const teamID = routeID(url.pathname, /^\/api\/v2\/admin\/teams\/(\d+)$/);
  if (teamID === null) {
    return false;
  }
  if (method === "PUT") {
    const team = state.teams.find((item) => item.id === teamID);
    if (!team) {
      return writeFailure(res, 404, "team not found.");
    }

    const body = await readJsonBody(req);
    const name = body?.name?.trim();
    const contactEmail = body?.contact_email?.trim();
    if (!name || !contactEmail) {
      return writeFailure(res, 400, "team update request is invalid.");
    }

    team.name = name;
    team.contact_email = contactEmail;
    state.players = state.players.map((player) =>
      player.team_id === teamID ? { ...player, team_name: name } : player,
    );

    return writeSuccess(res, team);
  }

  if (method === "DELETE") {
    state.teams = state.teams.filter((team) => team.id !== teamID);
    state.players = state.players.filter((player) => player.team_id !== teamID);
    return writeSuccess(res, {});
  }

  return false;
}

async function handleAdminPlayerRoutes(ctx) {
  return (
    handlePlayerWireGuardRoutes(ctx) ||
    (await handlePlayerCollectionRoutes(ctx)) ||
    (await handlePlayerItemRoutes(ctx))
  );
}

async function handlePlayerCollectionRoutes({ req, res, url, method }) {
  if (url.pathname !== "/api/v2/admin/players") {
    return false;
  }

  if (method === "GET") {
    return writeSuccess(res, state.players);
  }
  if (method !== "POST") {
    return false;
  }

  const body = await readJsonBody(req);
  const teamID = Number(body?.team_id);
  const team = state.teams.find((item) => item.id === teamID);
  const displayName = body?.display_name?.trim();
  const email = body?.email?.trim();
  const password = body?.password?.trim();
  const role = body?.role?.trim() || "member";

  if (!team || !displayName || !email || !password) {
    return writeFailure(res, 400, "player request is invalid.");
  }

  const playerID = state.nextPlayerID;
  const player = {
    id: playerID,
    team_id: team.id,
    team_name: team.name,
    display_name: displayName,
    email,
    role,
    wireguard_peer: `wg-${playerID}`,
    wireguard_address: `10.70.12.${playerID - 980}/32`,
    wireguard_status: "active",
    wireguard_issued_at: nowIso(),
    created_at: nowIso(),
  };
  state.nextPlayerID += 1;
  state.players.push(player);

  return writeSuccess(res, player);
}

async function handlePlayerItemRoutes({ req, res, url, method }) {
  const playerID = routeID(url.pathname, /^\/api\/v2\/admin\/players\/(\d+)$/);
  if (playerID === null) {
    return false;
  }
  if (method === "PUT") {
    const player = state.players.find((item) => item.id === playerID);
    if (!player) {
      return writeFailure(res, 404, "player not found.");
    }

    const body = await readJsonBody(req);
    const displayName = body?.display_name?.trim();
    const email = body?.email?.trim();
    const role = body?.role?.trim();
    if (!displayName || !email || !role) {
      return writeFailure(res, 400, "player update request is invalid.");
    }

    player.display_name = displayName;
    player.email = email;
    player.role = role;

    return writeSuccess(res, player);
  }

  if (method === "DELETE") {
    state.players = state.players.filter((player) => player.id !== playerID);
    return writeSuccess(res, {});
  }

  return false;
}

function handlePlayerWireGuardRoutes({ res, url, method }) {
  const playerID = routeID(
    url.pathname,
    /^\/api\/v2\/admin\/players\/(\d+)\/wireguard$/,
  );
  const rotatePlayerID = routeID(
    url.pathname,
    /^\/api\/v2\/admin\/players\/(\d+)\/wireguard\/rotate$/,
  );
  const revokePlayerID = routeID(
    url.pathname,
    /^\/api\/v2\/admin\/players\/(\d+)\/wireguard\/revoke$/,
  );

  if (method === "GET" && playerID !== null) {
    return writeWireGuardPeer(res, playerID);
  }

  if (method === "POST" && rotatePlayerID !== null) {
    const player = state.players.find((item) => item.id === rotatePlayerID);
    if (!player) {
      return writeFailure(res, 404, "player not found.");
    }
    player.wireguard_peer = `${player.wireguard_peer}-rotated`;
    player.wireguard_address = `10.70.15.${rotatePlayerID - 980}/32`;
    player.wireguard_status = "active";
    player.wireguard_issued_at = nowIso();
    delete player.wireguard_revoked_at;
    return writeSuccess(res, buildWireGuardPeer(player));
  }

  if (method === "POST" && revokePlayerID !== null) {
    const player = state.players.find((item) => item.id === revokePlayerID);
    if (!player) {
      return writeFailure(res, 404, "player not found.");
    }
    player.wireguard_status = "revoked";
    player.wireguard_revoked_at = nowIso();
    return writeSuccess(res, buildWireGuardPeer(player));
  }

  return false;
}

function writeWireGuardPeer(res, playerID) {
  const player = state.players.find((item) => item.id === playerID);
  if (!player) {
    return writeFailure(res, 404, "player not found.");
  }
  return writeSuccess(res, buildWireGuardPeer(player));
}

async function handleAdminChallengeRoutes({ req, res, url, method }) {
  if (url.pathname === "/api/v2/admin/challenges") {
    if (method === "GET") {
      return writeSuccess(res, state.challenges);
    }
    return method === "POST" ? handleCreateChallenge(req, res) : false;
  }

  const challengeID = routeID(url.pathname, /^\/api\/v2\/admin\/challenges\/(\d+)$/);
  if (challengeID !== null && method === "PUT") {
    return handleUpdateChallenge(req, res, challengeID);
  }
  if (challengeID !== null && method === "DELETE") {
    state.challenges = state.challenges.filter((challenge) => challenge.id !== challengeID);
    state.deployments = state.deployments.filter(
      (deployment) => deployment.challenge_id !== challengeID,
    );
    return writeSuccess(res, {});
  }

  const validateID = routeID(
    url.pathname,
    /^\/api\/v2\/admin\/challenges\/(\d+)\/validate$/,
  );
  if (method === "POST" && validateID !== null) {
    return writeSuccess(res, { ...challengeValidationResult, challenge_id: validateID });
  }

  const deployID = routeID(
    url.pathname,
    /^\/api\/v2\/admin\/challenges\/(\d+)\/deploy$/,
  );
  if (method === "POST" && deployID !== null) {
    return handleDeployChallenge(res, deployID);
  }

  return false;
}

async function handleCreateChallenge(req, res) {
  const body = await readJsonBody(req);
  const name = body?.name?.trim();
  if (!name) {
    return writeFailure(res, 400, "challenge request is invalid.");
  }

  const challenge = {
    id: state.nextChallengeID,
    name,
    baseline_image: body?.baseline_image?.trim() || "",
    checker_image: body?.checker_image?.trim() || "",
    source_bundle_path: body?.source_bundle_path?.trim() || "",
    weight: Number(body?.weight) || 1,
    service_port: Number(body?.service_port) || 30051,
    service_subnet_octet: Number(body?.service_subnet_octet) || 51,
    published: false,
    deployed_teams: 0,
    total_teams: state.teams.length,
    runtime_status: "draft",
    queued_teams: 0,
    ready_teams: 0,
    created_at: nowIso(),
  };
  state.nextChallengeID += 1;
  state.challenges.push(challenge);
  return writeSuccess(res, challenge);
}

async function handleUpdateChallenge(req, res, challengeID) {
  const challenge = state.challenges.find((item) => item.id === challengeID);
  if (!challenge) {
    return writeFailure(res, 404, "challenge not found.");
  }

  const body = await readJsonBody(req);
  const name = body?.name?.trim();
  if (!name) {
    return writeFailure(res, 400, "challenge update request is invalid.");
  }

  challenge.name = name;
  challenge.baseline_image = body?.baseline_image?.trim() || "";
  challenge.checker_image = body?.checker_image?.trim() || "";
  challenge.source_bundle_path = body?.source_bundle_path?.trim() || "";
  challenge.weight = Number(body?.weight) || 1;
  state.deployments = state.deployments.map((deployment) =>
    deployment.challenge_id === challengeID
      ? { ...deployment, challenge_name: challenge.name }
      : deployment,
  );

  return writeSuccess(res, challenge);
}

function handleDeployChallenge(res, challengeID) {
  const challenge = state.challenges.find((item) => item.id === challengeID);
  if (!challenge) {
    return writeFailure(res, 404, "challenge not found.");
  }

  const newJobID = state.nextDeploymentJobID;
  state.nextDeploymentJobID += 1;

  state.deployments = [
    {
      id: newJobID,
      challenge_id: challengeID,
      challenge_name: challenge.name,
      status: "queued",
      target_team_count: 3,
      queued_team_count: 3,
      ready_team_count: 0,
      failed_team_count: 0,
      created_at: nowIso(),
      completed_at: "",
    },
    ...state.deployments.map((deployment) =>
      deployment.challenge_id === challengeID &&
      (deployment.status === "queued" || deployment.status === "running")
        ? {
            ...deployment,
            status: "superseded",
            queued_team_count: 0,
            completed_at: nowIso(),
          }
        : deployment,
    ),
  ];

  state.challenges = state.challenges.map((item) =>
    item.id === challengeID
      ? {
          ...item,
          published: true,
          deployed_teams: 3,
          total_teams: 3,
          runtime_status: "deploying",
          queued_teams: 3,
          ready_teams: 0,
        }
      : item,
  );

  return writeSuccess(res, {
    job_id: newJobID,
    challenge_id: challengeID,
    challenge_name: challenge.name,
    status: "queued",
    published: true,
    deployed_team_count: 3,
    total_team_count: 3,
    queued_team_count: 3,
    ready_team_count: 0,
    created_at: nowIso(),
    completed_at: "",
  });
}

function handleAdminDeploymentRoutes({ res, url, method }) {
  const handled = writeGetRoute(res, method, url.pathname, {
    "/api/v2/admin/deployments": () => state.deployments,
    "/api/v2/admin/operations/status": () => state.operationsStatus,
  });
  if (handled) {
    return true;
  }

  if (method === "GET" && url.pathname === "/api/v2/admin/audit-logs") {
    const filtered = filterAuditLogs(auditLogs, url.searchParams);
    return writeSuccess(res, paginate(filtered, url.searchParams));
  }

  if (method === "POST" && url.pathname === "/api/v2/admin/deployments/reconcile") {
    return reconcileDeployments(res);
  }

  const deploymentID = routeID(url.pathname, /^\/api\/v2\/admin\/deployments\/(\d+)$/);
  if (method === "DELETE" && deploymentID !== null) {
    return deleteDeployment(res, deploymentID);
  }

  return false;
}

function handleAdminMetricsRoutes({ res, url, method }) {
  return writeTextRoute(res, method, url.pathname, {
    "/game-core/metrics": () => renderGameCoreMetrics(state.serviceMetrics.game_core),
    "/submission-service/metrics": () =>
      renderSubmissionMetrics(state.serviceMetrics.submission_service),
    "/controller-service/metrics": () =>
      renderControllerMetrics(state.serviceMetrics.controller_service),
    "/metrics": () => renderRealtimeMetrics(state.serviceMetrics.realtime_gateway),
    "/wireguard-gateway/metrics": () =>
      renderWireguardMetrics(state.serviceMetrics.wireguard_gateway),
  });
}

function reconcileDeployments(res) {
  if (state.deploymentReconcileProblem) {
    return writeFailure(
      res,
      state.deploymentReconcileProblem.statusCode,
      state.deploymentReconcileProblem.detail,
    );
  }

  let processedJobs = 0;
  let processedInstances = 0;
  let completedJobs = 0;

  state.deployments = state.deployments.map((deployment) => {
    if (deployment.status !== "queued" && deployment.status !== "provisioning") {
      return deployment;
    }

    processedJobs += 1;
    processedInstances += deployment.queued_team_count;
    completedJobs += 1;

    return {
      ...deployment,
      status: "completed",
      ready_team_count: deployment.target_team_count,
      queued_team_count: 0,
      completed_at: nowIso(),
    };
  });

  return writeSuccess(res, {
    processed_jobs: processedJobs,
    processed_instances: processedInstances,
    completed_jobs: completedJobs,
  });
}

function deleteDeployment(res, deploymentID) {
  const deployment = state.deployments.find((item) => item.id === deploymentID);
  if (!deployment) {
    return writeFailure(res, 404, "deployment job not found.");
  }
  if (deployment.status === "queued" || deployment.status === "provisioning") {
    return writeFailure(res, 400, "deployment job is still active.");
  }

  state.deployments = state.deployments.filter((item) => item.id !== deploymentID);
  return writeSuccess(res, {});
}

async function handleAdminGameRoutes(ctx) {
  const { res, url, method } = ctx;

  const handled = writeGetRoute(res, method, url.pathname, {
    "/api/v2/admin/game/status": () => state.gameStatus,
    "/api/v2/admin/game/scoreboard": () => state.scoreboard,
  });
  if (handled) {
    return true;
  }

  if (method === "GET" && url.pathname === "/api/v2/admin/game/attacks") {
    const filtered = filterAttackItems(state.attackItems, url.searchParams);
    return writeSuccess(res, paginate(filtered, url.searchParams));
  }

  if (method === "GET" && url.pathname === "/api/v2/admin/game/scheduler/events") {
    const filtered = filterSchedulerEvents(state.schedulerEvents, url.searchParams);
    return writeSuccess(res, paginate(filtered, url.searchParams));
  }

  if (method === "GET" && url.pathname === "/api/v2/admin/game/checker-runs") {
    const filtered = filterCheckerRuns(state.checkerRuns, url.searchParams);
    return writeSuccess(res, paginate(filtered, url.searchParams));
  }

  return (
    handleAdvanceTick(ctx) ||
    handleRecomputeScoring(ctx) ||
    (await handleMatchControlRoutes(ctx)) ||
    (await handleSchedulerControlRoutes(ctx))
  );
}

function handleAdvanceTick({ res, url, method }) {
  if (method !== "POST" || url.pathname !== "/api/v2/admin/game/ticks/advance") {
    return false;
  }

  const nextTickID =
    (state.gameStatus.current_tick?.id ?? state.gameStatus.total_ticks) + 1;
  const tick = {
    id: nextTickID,
    status: "completed",
    total_checker_runs: 6,
    successful_checker_runs: 6,
    failed_checker_runs: 0,
    skipped_checker_runs: 0,
    started_at: nowIso(),
    completed_at: nowIso(),
    message: `tick #${nextTickID} completed`,
  };

  state.gameStatus = {
    ...state.gameStatus,
    current_tick: tick,
    total_ticks: nextTickID,
    total_checker_runs: state.gameStatus.total_checker_runs + tick.total_checker_runs,
    successful_checker_runs:
      state.gameStatus.successful_checker_runs + tick.successful_checker_runs,
    failed_checker_runs:
      state.gameStatus.failed_checker_runs + tick.failed_checker_runs,
    skipped_checker_runs:
      state.gameStatus.skipped_checker_runs + tick.skipped_checker_runs,
    scheduler: state.gameStatus.scheduler
      ? {
          ...state.gameStatus.scheduler,
          last_tick_id: nextTickID,
          last_run_at: tick.completed_at,
        }
      : state.gameStatus.scheduler,
  };
  addSchedulerEvent(
    "tick_completed",
    "organizer",
    state.gameStatus.scheduler?.state ?? "stopped",
    `tick #${nextTickID} completed`,
    nextTickID,
  );

  return writeSuccess(res, tick);
}

function handleRecomputeScoring({ res, url, method }) {
  if (method !== "POST" || url.pathname !== "/api/v2/admin/game/scoring/recompute") {
    return false;
  }

  state.scoreboard = state.scoreboard.map((row, index) =>
    index === 0 ? { ...row, total: row.total + 5, delta: "+1" } : row,
  );
  return writeSuccess(res, state.scoreboard);
}

async function handleMatchControlRoutes({ req, res, url, method }) {
  if (method === "POST" && url.pathname === "/api/v2/admin/game/match/start") {
    state.gameStatus.match = {
      ...state.gameStatus.match,
      state: "running",
      started_at: nowIso(),
      ended_at: "",
      accepting_submissions: true,
    };
    return writeSuccess(res, state.gameStatus.match);
  }

  if (method === "PUT" && url.pathname === "/api/v2/admin/game/match/schedule") {
    const body = await readJsonBody(req);
    const scheduledStartAt = body?.scheduled_start_at ?? undefined;
    const scheduledEndAt = body?.scheduled_end_at ?? undefined;

    state.gameStatus.match = {
      ...state.gameStatus.match,
      scheduled_start_at: scheduledStartAt,
      scheduled_end_at: scheduledEndAt,
      schedule_configured: Boolean(scheduledStartAt || scheduledEndAt),
    };
    addSchedulerEvent(
      "schedule_updated",
      "organizer",
      state.gameStatus.scheduler.state,
      "match schedule updated",
    );
    return writeSuccess(res, state.gameStatus.match);
  }

  if (method === "POST" && url.pathname === "/api/v2/admin/game/match/stop") {
    state.gameStatus.match = {
      ...state.gameStatus.match,
      state: "finished",
      ended_at: nowIso(),
      accepting_submissions: false,
    };
    state.gameStatus.scheduler = {
      ...state.gameStatus.scheduler,
      state: "stopped",
      next_run_at: "",
    };
    addSchedulerEvent("match_stopped", "organizer", "stopped", "match stopped");
    return writeSuccess(res, state.gameStatus.match);
  }

  return false;
}

async function handleSchedulerControlRoutes({ req, res, url, method }) {
  if (method === "POST" && url.pathname === "/api/v2/admin/game/scheduler/start") {
    state.gameStatus.scheduler = {
      ...state.gameStatus.scheduler,
      state: "running",
      next_run_at: nextRunAt(state.gameStatus.scheduler.interval_seconds),
      last_error: "",
    };
    addSchedulerEvent(
      "started",
      "organizer",
      "running",
      "scheduler started via organizer control",
    );
    return writeSuccess(res, state.gameStatus.scheduler);
  }

  if (method === "POST" && url.pathname === "/api/v2/admin/game/scheduler/stop") {
    state.gameStatus.scheduler = {
      ...state.gameStatus.scheduler,
      state: "stopped",
      next_run_at: "",
      last_error: "",
    };
    addSchedulerEvent(
      "stopped",
      "organizer",
      "stopped",
      "scheduler stopped via organizer control",
    );
    return writeSuccess(res, state.gameStatus.scheduler);
  }

  if (method === "PUT" && url.pathname === "/api/v2/admin/game/scheduler/interval") {
    const body = await readJsonBody(req);
    const intervalSeconds = Number(body?.interval_seconds);
    if (!Number.isInteger(intervalSeconds) || intervalSeconds <= 0) {
      return writeFailure(res, 400, "interval_seconds must be positive.");
    }

    state.gameStatus.scheduler = {
      ...state.gameStatus.scheduler,
      interval_seconds: intervalSeconds,
      next_run_at:
        state.gameStatus.scheduler.state === "running"
          ? nextRunAt(intervalSeconds)
          : state.gameStatus.scheduler.next_run_at,
    };
    addSchedulerEvent(
      "interval_updated",
      "organizer",
      state.gameStatus.scheduler.state,
      `scheduler interval set to ${intervalSeconds} seconds`,
    );
    return writeSuccess(res, state.gameStatus.scheduler);
  }

  return false;
}

function handleAdminNetworkRoutes({ res, url, method }) {
  const handled = writeGetRoute(res, method, url.pathname, {
    "/api/v2/admin/wireguard/status": () => state.wireguardStatus,
    "/api/v2/admin/access/status": () => state.accessStatus,
  });
  if (handled) return true;

  if (method === "POST" && url.pathname === "/api/v2/admin/wireguard/reconcile") {
    const peersTotal = state.players.length;
    const peersRevoked = state.players.filter(
      (player) => player.wireguard_status === "revoked",
    ).length;
    const peersActive = peersTotal - peersRevoked;

    state.wireguardStatus = {
      ...state.wireguardStatus,
      state: "applied",
      mode: "host",
      interface: "wg0",
      firewall_backend: "nftables",
      peers_total: peersTotal,
      peers_active: peersActive,
      peers_revoked: peersRevoked,
      revision: `mock-wireguard-revision-${peersActive}-${peersRevoked}`,
      applied_at: nowIso(),
    };
    return writeSuccess(res, state.wireguardStatus);
  }

  if (method === "POST" && url.pathname === "/api/v2/admin/wireguard/teardown") {
    state.wireguardStatus = {
      ...state.wireguardStatus,
      state: "stopped",
      peers_total: 0,
      peers_active: 0,
      peers_revoked: 0,
      revision: "mock-wireguard-teardown",
      applied_at: nowIso(),
    };
    return writeSuccess(res, {});
  }

  if (method === "POST" && url.pathname === "/api/v2/admin/access/reconcile") {
    const activePeers = state.players.filter(
      (player) => player.wireguard_status === "active",
    ).length;
    state.accessStatus = {
      ...state.accessStatus,
      state: "applied",
      mode: "host",
      interface: "wg0",
      firewall_backend: "iptables",
      policies_total: 1,
      ssh_open_services: 1,
      ssh_locked_services: 0,
      allowed_peers_total: activePeers,
      revision: `mock-access-revision-${activePeers}`,
      applied_at: nowIso(),
    };
    return writeSuccess(res, state.accessStatus);
  }

  if (method === "POST" && url.pathname === "/api/v2/admin/access/teardown") {
    state.accessStatus = {
      ...state.accessStatus,
      state: "stopped",
      policies_total: 0,
      ssh_open_services: 0,
      ssh_locked_services: 0,
      allowed_peers_total: 0,
      revision: "mock-access-teardown",
      applied_at: nowIso(),
    };
    return writeSuccess(res, {});
  }

  return false;
}

function handleStreamRoutes({ res, url, method }) {
  return writeStreamRoute(res, method, url.pathname, {
    "/public/v1/scoreboard/stream": (response, pathname) =>
      writeScenarioSSE(response, pathname, state.scoreboard),
    "/public/v1/attacks/stream": (response, pathname) =>
      writeScenarioSSE(
        response,
        pathname,
        paginate(state.attackItems, new URLSearchParams("limit=12&offset=0")),
      ),
    "/admin/v1/game/status/stream": (response, pathname) =>
      writeScenarioSSE(response, pathname, state.gameStatus),
    "/admin/v1/game/scoreboard/stream": (response, pathname) =>
      writeScenarioSSE(response, pathname, state.scoreboard),
    "/admin/v1/game/attacks/stream": (response) =>
      writeSSE(
        response,
        paginate(state.attackItems, new URLSearchParams("limit=12&offset=0")),
      ),
    "/admin/v1/game/checker-runs/stream": (response) =>
      writeSSE(
        response,
        paginate(state.checkerRuns, new URLSearchParams("limit=18&offset=0")),
      ),
    "/admin/v1/game/scheduler/events/stream": (response) =>
      writeSSE(
        response,
        paginate(state.schedulerEvents, new URLSearchParams("limit=12&offset=0")),
      ),
  });
}

server.listen(port, "127.0.0.1", () => {
  process.stdout.write(
    `mock platform api listening on http://127.0.0.1:${port}\n`,
  );
});

function shutdown() {
  server.close(() => {
    process.exit(0);
  });
}

process.on("SIGINT", shutdown);
process.on("SIGTERM", shutdown);
