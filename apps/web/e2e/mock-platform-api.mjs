import http from "node:http";

const port = Number(process.env.MOCK_PLATFORM_API_PORT || "4010");

const challenges = [{ id: 1, name: "college-http" }];

const scoreboard = [
  {
    rank: 1,
    team: "College Alpha",
    attack: 180,
    defense: 140,
    sla: 120,
    total: 440,
    delta: "+2",
  },
  {
    rank: 2,
    team: "College Beta",
    attack: 130,
    defense: 120,
    sla: 110,
    total: 360,
    delta: "-1",
  },
  {
    rank: 3,
    team: "College Gamma",
    attack: 90,
    defense: 105,
    sla: 100,
    total: 295,
    delta: "+0",
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
    sshPasswordCounter: 1,
    nextTeamID: 104,
    nextPlayerID: 1003,
    nextChallengeID: 2,
    nextDeploymentJobID: 89,
    nextSchedulerEventID: 4,
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

  return nextState;
}

let state = createStateForScenario();

function success(data) {
  return JSON.stringify({ status: "success", data });
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

  res.on("close", () => {
    clearInterval(heartbeat);
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

const server = http.createServer(async (req, res) => {
  const url = new URL(req.url || "/", `http://127.0.0.1:${port}`);
  const schedulerStartMatch = url.pathname.match(/^\/api\/v2\/admin\/game\/match\/start$/);
  const schedulerStopMatch = url.pathname.match(/^\/api\/v2\/admin\/game\/match\/stop$/);
  const matchScheduleRoute = url.pathname.match(
    /^\/api\/v2\/admin\/game\/match\/schedule$/,
  );
  const deploymentReconcileRoute = url.pathname.match(
    /^\/api\/v2\/admin\/deployments\/reconcile$/,
  );
  const playerWireGuardRoute = url.pathname.match(
    /^\/api\/v2\/admin\/players\/(\d+)\/wireguard$/,
  );
  const playerWireGuardRotateRoute = url.pathname.match(
    /^\/api\/v2\/admin\/players\/(\d+)\/wireguard\/rotate$/,
  );
  const playerWireGuardRevokeRoute = url.pathname.match(
    /^\/api\/v2\/admin\/players\/(\d+)\/wireguard\/revoke$/,
  );
  const validateChallengeRoute = url.pathname.match(
    /^\/api\/v2\/admin\/challenges\/(\d+)\/validate$/,
  );
  const deployChallengeRoute = url.pathname.match(
    /^\/api\/v2\/admin\/challenges\/(\d+)\/deploy$/,
  );
  const schedulerStart = url.pathname.match(/^\/api\/v2\/admin\/game\/scheduler\/start$/);
  const schedulerStop = url.pathname.match(/^\/api\/v2\/admin\/game\/scheduler\/stop$/);
  const schedulerInterval = url.pathname.match(/^\/api\/v2\/admin\/game\/scheduler\/interval$/);
  const advanceTickRoute = url.pathname.match(
    /^\/api\/v2\/admin\/game\/ticks\/advance$/,
  );
  const recomputeScoringRoute = url.pathname.match(
    /^\/api\/v2\/admin\/game\/scoring\/recompute$/,
  );
  const deleteDeploymentRoute = url.pathname.match(
    /^\/api\/v2\/admin\/deployments\/(\d+)$/,
  );
  const unlockRoute = url.pathname.match(/^\/api\/v2\/services\/(\d+)\/unlock$/);
  const sshRoute = url.pathname.match(/^\/api\/v2\/services\/(\d+)\/ssh-session$/);
  const restartRoute = url.pathname.match(/^\/api\/v2\/services\/(\d+)\/reset\/restart$/);
  const factoryResetRoute = url.pathname.match(/^\/api\/v2\/services\/(\d+)\/reset\/factory$/);

  if (req.method === "POST" && url.pathname === "/__reset") {
    const body = await readJsonBody(req);
    state = createStateForScenario(body?.scenario);
    return writeJson(res, 200, success({ reset: true }));
  }

  if (shouldFailRoute(req.method ?? "GET", url.pathname)) {
    return writeJson(
      res,
      503,
      JSON.stringify({
        status: "failed",
        message: `mock degraded route: ${url.pathname}`,
      }),
    );
  }

  if (req.method === "POST" && url.pathname === "/api/v2/authenticate") {
    const body = await readJsonBody(req);
    const email = body?.email?.trim();
    const password = body?.password?.trim();
    const knownPasswords = {
      "alpha.captain@college.local": "alpha-password",
      "beta.member@college.local": "beta-password",
    };
    const player = state.players.find((item) => item.email === email);

    if (
      !player ||
      !password ||
      knownPasswords[email] !== password
    ) {
      return writeJson(
        res,
        403,
        JSON.stringify({
          status: "forbidden",
          message: "email or password is wrong.",
        }),
      );
    }

    return writeJson(res, 200, success(buildParticipantToken(player)));
  }

  if (req.method === "GET" && url.pathname === "/api/v2/challenges") {
    return writeJson(res, 200, success(challenges));
  }

  if (req.method === "GET" && url.pathname === "/api/v2/scoreboard") {
    return writeJson(res, 200, success(state.scoreboard));
  }

  if (req.method === "GET" && url.pathname === "/api/v2/game/status") {
    return writeJson(res, 200, success(state.gameStatus));
  }

  if (req.method === "GET" && url.pathname === "/api/v2/attacks") {
    const filtered = filterAttackItems(state.attackItems, url.searchParams);
    return writeJson(res, 200, success(paginate(filtered, url.searchParams)));
  }

  if (req.method === "GET" && url.pathname === "/api/v2/services") {
    return writeJson(res, 200, success(state.serviceMap));
  }

  if (req.method === "GET" && url.pathname === "/api/v2/team/services") {
    return writeJson(res, 200, success(state.teamServiceStates));
  }

  if (req.method === "POST" && unlockRoute) {
    const challengeID = Number(unlockRoute[1]);
    const serviceState = findServiceState(challengeID);
    const body = await readJsonBody(req);
    if (!serviceState || !body?.proof) {
      return writeJson(
        res,
        400,
        JSON.stringify({ status: "failed", message: "unlock proof is invalid." }),
      );
    }

    serviceState.unlocked = true;
    serviceState.ssh_hint =
      "unlock accepted; request a one-time root password to get the current credential";
    serviceState.last_event = "unlock granted via participant API";

    return writeJson(
      res,
      200,
      success({
        challenge_id: challengeID,
        team_id: serviceState.team_id,
        unlocked: true,
        ssh_credential_ttl_seconds: 900,
      }),
    );
  }

  if (req.method === "POST" && sshRoute) {
    const challengeID = Number(sshRoute[1]);
    const serviceState = findServiceState(challengeID);
    if (!serviceState) {
      return writeJson(
        res,
        404,
        JSON.stringify({ status: "failed", message: "service not found." }),
      );
    }

    const password = `root-pass-${state.sshPasswordCounter}`;
    state.sshPasswordCounter += 1;
    serviceState.unlocked = true;
    serviceState.ssh_hint = "ssh root@10.80.50.11 -p 22";
    serviceState.last_event = "ssh access active";

    return writeJson(
      res,
      200,
      success({
        host: "10.80.50.11",
        port: 22,
        username: "root",
        password,
        expires_at: "2026-03-20T10:30:00Z",
        connection_hint: "ssh root@10.80.50.11 -p 22",
      }),
    );
  }

  if (req.method === "POST" && restartRoute) {
    const challengeID = Number(restartRoute[1]);
    const serviceState = findServiceState(challengeID);
    if (!serviceState) {
      return writeJson(
        res,
        404,
        JSON.stringify({ status: "failed", message: "service not found." }),
      );
    }

    serviceState.status = "warming";
    serviceState.checker = "warning";
    serviceState.last_event = "service restart triggered via participant API";
    serviceState.reset_cooldown = "restart requested";

    return writeJson(
      res,
      200,
      success({
        challenge_id: challengeID,
        team_id: serviceState.team_id,
        action: "restart",
      }),
    );
  }

  if (req.method === "POST" && factoryResetRoute) {
    const challengeID = Number(factoryResetRoute[1]);
    const serviceState = findServiceState(challengeID);
    if (!serviceState) {
      return writeJson(
        res,
        404,
        JSON.stringify({ status: "failed", message: "service not found." }),
      );
    }

    serviceState.status = "warming";
    serviceState.checker = "warning";
    serviceState.unlocked = true;
    serviceState.ssh_hint =
      "unlock preserved; request a fresh one-time root password to rotate the credential";
    serviceState.last_event = "factory reset triggered via participant API";
    serviceState.reset_cooldown = "cooldown: 90s";

    return writeJson(
      res,
      200,
      success({
        challenge_id: challengeID,
        team_id: serviceState.team_id,
        action: "factory_reset",
        unlock_preserved: true,
      }),
    );
  }

  if (req.method === "GET" && url.pathname === "/api/v2/admin/teams") {
    return writeJson(res, 200, success(state.teams));
  }

  if (req.method === "POST" && url.pathname === "/api/v2/admin/teams") {
    const body = await readJsonBody(req);
    const name = body?.name?.trim();
    const contactEmail = body?.contact_email?.trim();

    if (!name || !contactEmail) {
      return writeJson(
        res,
        400,
        JSON.stringify({ status: "failed", message: "team request is invalid." }),
      );
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

    return writeJson(res, 200, success(team));
  }

  const teamRoute = url.pathname.match(/^\/api\/v2\/admin\/teams\/(\d+)$/);
  if (req.method === "PUT" && teamRoute) {
    const teamID = Number(teamRoute[1]);
    const team = state.teams.find((item) => item.id === teamID);
    if (!team) {
      return writeJson(
        res,
        404,
        JSON.stringify({ status: "failed", message: "team not found." }),
      );
    }

    const body = await readJsonBody(req);
    const name = body?.name?.trim();
    const contactEmail = body?.contact_email?.trim();
    if (!name || !contactEmail) {
      return writeJson(
        res,
        400,
        JSON.stringify({
          status: "failed",
          message: "team update request is invalid.",
        }),
      );
    }

    team.name = name;
    team.contact_email = contactEmail;
    state.players = state.players.map((player) =>
      player.team_id === teamID ? { ...player, team_name: name } : player,
    );

    return writeJson(res, 200, success(team));
  }

  if (req.method === "GET" && url.pathname === "/api/v2/admin/players") {
    return writeJson(res, 200, success(state.players));
  }

  if (req.method === "GET" && playerWireGuardRoute) {
    const playerID = Number(playerWireGuardRoute[1]);
    const player = state.players.find((item) => item.id === playerID);
    if (!player) {
      return writeJson(
        res,
        404,
        JSON.stringify({ status: "failed", message: "player not found." }),
      );
    }

    return writeJson(res, 200, success(buildWireGuardPeer(player)));
  }

  if (req.method === "POST" && playerWireGuardRotateRoute) {
    const playerID = Number(playerWireGuardRotateRoute[1]);
    const player = state.players.find((item) => item.id === playerID);
    if (!player) {
      return writeJson(
        res,
        404,
        JSON.stringify({ status: "failed", message: "player not found." }),
      );
    }

    player.wireguard_peer = `${player.wireguard_peer}-rotated`;
    player.wireguard_address = `10.70.15.${playerID - 980}/32`;
    player.wireguard_status = "active";
    player.wireguard_issued_at = nowIso();
    delete player.wireguard_revoked_at;

    return writeJson(res, 200, success(buildWireGuardPeer(player)));
  }

  if (req.method === "POST" && playerWireGuardRevokeRoute) {
    const playerID = Number(playerWireGuardRevokeRoute[1]);
    const player = state.players.find((item) => item.id === playerID);
    if (!player) {
      return writeJson(
        res,
        404,
        JSON.stringify({ status: "failed", message: "player not found." }),
      );
    }

    player.wireguard_status = "revoked";
    player.wireguard_revoked_at = nowIso();

    return writeJson(res, 200, success(buildWireGuardPeer(player)));
  }

  if (req.method === "POST" && url.pathname === "/api/v2/admin/players") {
    const body = await readJsonBody(req);
    const teamID = Number(body?.team_id);
    const team = state.teams.find((item) => item.id === teamID);
    const displayName = body?.display_name?.trim();
    const email = body?.email?.trim();
    const password = body?.password?.trim();
    const role = body?.role?.trim() || "member";

    if (!team || !displayName || !email || !password) {
      return writeJson(
        res,
        400,
        JSON.stringify({ status: "failed", message: "player request is invalid." }),
      );
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

    return writeJson(res, 200, success(player));
  }

  const playerRoute = url.pathname.match(/^\/api\/v2\/admin\/players\/(\d+)$/);
  if (req.method === "PUT" && playerRoute) {
    const playerID = Number(playerRoute[1]);
    const player = state.players.find((item) => item.id === playerID);
    if (!player) {
      return writeJson(
        res,
        404,
        JSON.stringify({ status: "failed", message: "player not found." }),
      );
    }

    const body = await readJsonBody(req);
    const displayName = body?.display_name?.trim();
    const email = body?.email?.trim();
    const role = body?.role?.trim();
    if (!displayName || !email || !role) {
      return writeJson(
        res,
        400,
        JSON.stringify({
          status: "failed",
          message: "player update request is invalid.",
        }),
      );
    }

    player.display_name = displayName;
    player.email = email;
    player.role = role;

    return writeJson(res, 200, success(player));
  }

  if (req.method === "GET" && url.pathname === "/api/v2/admin/challenges") {
    return writeJson(res, 200, success(state.challenges));
  }

  if (req.method === "POST" && url.pathname === "/api/v2/admin/challenges") {
    const body = await readJsonBody(req);
    const name = body?.name?.trim();
    if (!name) {
      return writeJson(
        res,
        400,
        JSON.stringify({ status: "failed", message: "challenge request is invalid." }),
      );
    }

    const challenge = {
      id: state.nextChallengeID,
      name,
      baseline_image: body?.baseline_image?.trim() || "",
      checker_image: body?.checker_image?.trim() || "",
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

    return writeJson(res, 200, success(challenge));
  }

  const challengeRoute = url.pathname.match(/^\/api\/v2\/admin\/challenges\/(\d+)$/);
  if (req.method === "PUT" && challengeRoute) {
    const challengeID = Number(challengeRoute[1]);
    const challenge = state.challenges.find((item) => item.id === challengeID);
    if (!challenge) {
      return writeJson(
        res,
        404,
        JSON.stringify({ status: "failed", message: "challenge not found." }),
      );
    }

    const body = await readJsonBody(req);
    const name = body?.name?.trim();
    if (!name) {
      return writeJson(
        res,
        400,
        JSON.stringify({
          status: "failed",
          message: "challenge update request is invalid.",
        }),
      );
    }

    challenge.name = name;
    challenge.baseline_image = body?.baseline_image?.trim() || "";
    challenge.checker_image = body?.checker_image?.trim() || "";
    challenge.weight = Number(body?.weight) || 1;
    state.deployments = state.deployments.map((deployment) =>
      deployment.challenge_id === challengeID
        ? { ...deployment, challenge_name: challenge.name }
        : deployment,
    );

    return writeJson(res, 200, success(challenge));
  }

  if (req.method === "POST" && validateChallengeRoute) {
    const challengeID = Number(validateChallengeRoute[1]);
    return writeJson(
      res,
      200,
      success({ ...challengeValidationResult, challenge_id: challengeID }),
    );
  }

  if (req.method === "POST" && deployChallengeRoute) {
    const challengeID = Number(deployChallengeRoute[1]);
    const challenge = state.challenges.find((item) => item.id === challengeID);
    if (!challenge) {
      return writeJson(
        res,
        404,
        JSON.stringify({ status: "failed", message: "challenge not found." }),
      );
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

    return writeJson(
      res,
      200,
      success({
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
      }),
    );
  }

  if (req.method === "GET" && url.pathname === "/api/v2/admin/audit-logs") {
    const filtered = filterAuditLogs(auditLogs, url.searchParams);
    return writeJson(res, 200, success(paginate(filtered, url.searchParams)));
  }

  if (req.method === "GET" && url.pathname === "/api/v2/admin/deployments") {
    return writeJson(res, 200, success(state.deployments));
  }

  if (req.method === "GET" && url.pathname === "/api/v2/admin/operations/status") {
    return writeJson(res, 200, success(state.operationsStatus));
  }

  if (req.method === "POST" && deploymentReconcileRoute) {
    let processedJobs = 0;
    let processedInstances = 0;
    let completedJobs = 0;

    state.deployments = state.deployments.map((deployment) => {
      if (
        deployment.status !== "queued" &&
        deployment.status !== "provisioning"
      ) {
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

    return writeJson(
      res,
      200,
      success({
        processed_jobs: processedJobs,
        processed_instances: processedInstances,
        completed_jobs: completedJobs,
      }),
    );
  }

  if (req.method === "DELETE" && deleteDeploymentRoute) {
    const deploymentID = Number(deleteDeploymentRoute[1]);
    const deployment = state.deployments.find((item) => item.id === deploymentID);

    if (!deployment) {
      return writeJson(
        res,
        404,
        JSON.stringify({
          status: "failed",
          message: "deployment job not found.",
        }),
      );
    }

    if (deployment.status === "queued" || deployment.status === "provisioning") {
      return writeJson(
        res,
        400,
        JSON.stringify({
          status: "failed",
          message: "deployment job is still active.",
        }),
      );
    }

    state.deployments = state.deployments.filter((item) => item.id !== deploymentID);
    return writeJson(res, 200, success({}));
  }

  const deleteTeamRoute = url.pathname.match(/^\/api\/v2\/admin\/teams\/(\d+)$/);
  if (req.method === "DELETE" && deleteTeamRoute) {
    const teamID = Number(deleteTeamRoute[1]);
    state.teams = state.teams.filter((team) => team.id !== teamID);
    state.players = state.players.filter((player) => player.team_id !== teamID);
    return writeJson(res, 200, success({}));
  }

  const deletePlayerRoute = url.pathname.match(
    /^\/api\/v2\/admin\/players\/(\d+)$/,
  );
  if (req.method === "DELETE" && deletePlayerRoute) {
    const playerID = Number(deletePlayerRoute[1]);
    state.players = state.players.filter((player) => player.id !== playerID);
    return writeJson(res, 200, success({}));
  }

  const deleteChallengeRoute = url.pathname.match(
    /^\/api\/v2\/admin\/challenges\/(\d+)$/,
  );
  if (req.method === "DELETE" && deleteChallengeRoute) {
    const challengeID = Number(deleteChallengeRoute[1]);
    state.challenges = state.challenges.filter(
      (challenge) => challenge.id !== challengeID,
    );
    state.deployments = state.deployments.filter(
      (deployment) => deployment.challenge_id !== challengeID,
    );
    return writeJson(res, 200, success({}));
  }

  if (req.method === "GET" && url.pathname === "/api/v2/admin/game/status") {
    return writeJson(res, 200, success(state.gameStatus));
  }

  if (req.method === "GET" && url.pathname === "/api/v2/admin/game/scoreboard") {
    return writeJson(res, 200, success(state.scoreboard));
  }

  if (req.method === "GET" && url.pathname === "/api/v2/admin/game/attacks") {
    const filtered = filterAttackItems(state.attackItems, url.searchParams);
    return writeJson(res, 200, success(paginate(filtered, url.searchParams)));
  }

  if (
    req.method === "GET" &&
    url.pathname === "/api/v2/admin/game/scheduler/events"
  ) {
    const filtered = filterSchedulerEvents(state.schedulerEvents, url.searchParams);
    return writeJson(res, 200, success(paginate(filtered, url.searchParams)));
  }

  if (
    req.method === "GET" &&
    url.pathname === "/api/v2/admin/game/checker-runs"
  ) {
    const filtered = filterCheckerRuns(state.checkerRuns, url.searchParams);
    return writeJson(res, 200, success(paginate(filtered, url.searchParams)));
  }

  if (req.method === "POST" && advanceTickRoute) {
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

    return writeJson(res, 200, success(tick));
  }

  if (req.method === "POST" && recomputeScoringRoute) {
    state.scoreboard = state.scoreboard.map((row, index) =>
      index === 0 ? { ...row, total: row.total + 5, delta: "+1" } : row,
    );
    return writeJson(res, 200, success(state.scoreboard));
  }

  if (req.method === "GET" && url.pathname === "/api/v2/admin/wireguard/status") {
    return writeJson(res, 200, success(state.wireguardStatus));
  }

  if (req.method === "POST" && url.pathname === "/api/v2/admin/wireguard/reconcile") {
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

    return writeJson(res, 200, success(state.wireguardStatus));
  }

  if (req.method === "POST" && url.pathname === "/api/v2/admin/wireguard/teardown") {
    state.wireguardStatus = {
      ...state.wireguardStatus,
      state: "stopped",
      peers_total: 0,
      peers_active: 0,
      peers_revoked: 0,
      revision: "mock-wireguard-teardown",
      applied_at: nowIso(),
    };
    return writeJson(res, 200, success({ status: "success" }));
  }

  if (req.method === "GET" && url.pathname === "/api/v2/admin/access/status") {
    return writeJson(res, 200, success(state.accessStatus));
  }

  if (req.method === "POST" && url.pathname === "/api/v2/admin/access/reconcile") {
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
    return writeJson(res, 200, success(state.accessStatus));
  }

  if (req.method === "POST" && url.pathname === "/api/v2/admin/access/teardown") {
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
    return writeJson(res, 200, success({ status: "success" }));
  }

  if (req.method === "POST" && schedulerStartMatch) {
    state.gameStatus.match = {
      ...state.gameStatus.match,
      state: "running",
      started_at: nowIso(),
      ended_at: "",
      accepting_submissions: true,
    };
    return writeJson(res, 200, success(state.gameStatus.match));
  }

  if (req.method === "PUT" && matchScheduleRoute) {
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
    return writeJson(res, 200, success(state.gameStatus.match));
  }

  if (req.method === "POST" && schedulerStopMatch) {
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
    return writeJson(res, 200, success(state.gameStatus.match));
  }

  if (req.method === "POST" && schedulerStart) {
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
    return writeJson(res, 200, success(state.gameStatus.scheduler));
  }

  if (req.method === "POST" && schedulerStop) {
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
    return writeJson(res, 200, success(state.gameStatus.scheduler));
  }

  if (req.method === "PUT" && schedulerInterval) {
    const body = await readJsonBody(req);
    const intervalSeconds = Number(body?.interval_seconds);
    if (!Number.isInteger(intervalSeconds) || intervalSeconds <= 0) {
      return writeJson(
        res,
        400,
        JSON.stringify({
          status: "failed",
          message: "interval_seconds must be positive.",
        }),
      );
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
    return writeJson(res, 200, success(state.gameStatus.scheduler));
  }

  if (req.method === "GET" && url.pathname === "/public/v1/scoreboard/stream") {
    return writeScenarioSSE(res, url.pathname, state.scoreboard);
  }

  if (req.method === "GET" && url.pathname === "/public/v1/attacks/stream") {
    return writeScenarioSSE(
      res,
      url.pathname,
      paginate(state.attackItems, new URLSearchParams("limit=12&offset=0")),
    );
  }

  if (req.method === "GET" && url.pathname === "/admin/v1/game/status/stream") {
    return writeScenarioSSE(res, url.pathname, state.gameStatus);
  }

  if (
    req.method === "GET" &&
    url.pathname === "/admin/v1/game/scoreboard/stream"
  ) {
    return writeScenarioSSE(res, url.pathname, state.scoreboard);
  }

  if (req.method === "GET" && url.pathname === "/admin/v1/game/attacks/stream") {
    return writeSSE(
      res,
      paginate(state.attackItems, new URLSearchParams("limit=12&offset=0")),
    );
  }

  if (
    req.method === "GET" &&
    url.pathname === "/admin/v1/game/checker-runs/stream"
  ) {
    return writeSSE(
      res,
      paginate(state.checkerRuns, new URLSearchParams("limit=18&offset=0")),
    );
  }

  if (
    req.method === "GET" &&
    url.pathname === "/admin/v1/game/scheduler/events/stream"
  ) {
    return writeSSE(
      res,
      paginate(state.schedulerEvents, new URLSearchParams("limit=12&offset=0")),
    );
  }

  return writeJson(
    res,
    404,
    JSON.stringify({
      status: "failed",
      message: `mock route not found: ${url.pathname}`,
    }),
  );
});

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
