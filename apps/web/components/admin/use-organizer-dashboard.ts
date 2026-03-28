"use client";

import { useEffect, useMemo, useRef, useState } from "react";

import type {
  AdminAttackFeedPage,
  AdminAttackFeedQuery,
  AdminChallenge,
  AdminChallengeValidationResult,
  AdminCheckerRunPage,
  AdminCheckerRunQuery,
  AdminControllerAccessStatus,
  AdminDeployment,
  AdminDeploymentJob,
  AdminGameScoreRow,
  AdminGameStatus,
  AdminGameTickStatus,
  AdminOperationsStatus,
  AdminServiceMetricSnapshot,
  AdminPlayer,
  AdminReconcileResult,
  AdminSchedulerEventPage,
  AdminSchedulerEventQuery,
  AdminTeam,
  AdminWireGuardGatewayStatus,
  AdminWireGuardPeer,
} from "@/lib/admin-dashboard-types";

export type OrganizerDashboardOptions = {
  attackPage: AdminAttackFeedPage;
  challenges: AdminChallenge[];
  checkerRunPage: AdminCheckerRunPage;
  deployments: AdminDeploymentJob[];
  gameStatus: AdminGameStatus;
  operationsStatus?: AdminOperationsStatus;
  serviceMetrics?: AdminServiceMetricSnapshot | null;
  players: AdminPlayer[];
  schedulerEventPage: AdminSchedulerEventPage;
  scoreboard: AdminGameScoreRow[];
  teams: AdminTeam[];
};

type ActionEnvelope<T> =
  | {
      status: "success";
      data: T;
    }
  | {
      status: "failed" | "forbidden";
      message: string;
    };

const defaultCheckerRunFilters = {
  limit: "18",
  offset: "0",
  tickId: "",
  teamId: "",
  challengeId: "",
  phase: "",
  status: "",
};

const baseAttackFilters = {
  limit: "12",
  offset: "0",
  attacker: "",
  victim: "",
  service: "",
  tickFrom: "",
  tickTo: "",
};

const defaultSchedulerEventFilters = {
  limit: "12",
  offset: "0",
  eventType: "",
  source: "",
  state: "",
};

type TeamDraft = {
  contactEmail: string;
  name: string;
};

type PlayerDraft = {
  displayName: string;
  email: string;
  password: string;
  role: string;
  teamId: number;
};

type ChallengeDraft = {
  baselineImage: string;
  checkerImage: string;
  name: string;
  servicePort: string;
  serviceSubnetOctet: string;
  weight: string;
};

export type DeleteTarget = {
  kind: "team" | "player" | "challenge" | "deployment";
  id: number;
  label: string;
  warning?: string;
};

type CheckerRunFilters = typeof defaultCheckerRunFilters;
type AttackFilters = typeof baseAttackFilters;
type SchedulerEventFilters = typeof defaultSchedulerEventFilters;

type OrganizerSummary = {
  pendingDeployments: number;
  pendingDrafts: number;
  players: number;
  publishedChallenges: number;
  teams: number;
  totalTicks: number;
};

export type FormMode = "create" | "edit" | null;
export type FormEntity = "team" | "player" | "challenge" | null;

export type OrganizerDashboardState = {
  accessStatus: AdminControllerAccessStatus | null;
  actionError: string | null;
  actionNote: string | null;
  applyAttackFilters: () => Promise<void>;
  attackFilters: AttackFilters;
  highlightedAttackIDs: string[];
  attackPageState: AdminAttackFeedPage;
  attacksLiveMode: boolean;
  applyCheckerRunFilters: () => Promise<void>;
  applySchedulerEventFilters: () => Promise<void>;
  challengeDraft: ChallengeDraft;
  challengeRows: AdminChallenge[];
  checkerRunFilters: CheckerRunFilters;
  checkerRunPageState: AdminCheckerRunPage;
  checkerRunsLiveMode: boolean;
  challengeValidationRows: Record<number, AdminChallengeValidationResult>;
  closeDeleteDialog: () => void;
  closeFormDialog: () => void;
  closeWireGuardDialog: () => void;
  confirmDelete: () => Promise<void>;
  createChallenge: () => Promise<void>;
  createPlayer: () => Promise<void>;
  createTeam: () => Promise<void>;
  deleteTarget: DeleteTarget | null;
  editingId: number | null;
  formEntity: FormEntity;
  formMode: FormMode;
  openCreateDialog: (entity: "team" | "player" | "challenge") => void;
  openEditDialog: (entity: "team" | "player" | "challenge", id: number) => void;
  selectDeleteTarget: (target: DeleteTarget) => void;
  submitForm: () => Promise<void>;
  deploymentRows: AdminDeploymentJob[];
  deployChallenge: (challenge: AdminChallenge) => Promise<void>;
  downloadWireGuardConfig: (peer: AdminWireGuardPeer) => void;
  gameState: AdminGameStatus;
  inspectWireGuard: (player: AdminPlayer) => Promise<void>;
  pageAttacks: (direction: "prev" | "next") => Promise<void>;
  pageCheckerRuns: (direction: "prev" | "next") => Promise<void>;
  pageSchedulerEvents: (direction: "prev" | "next") => Promise<void>;
  pendingAction: string | null;
  playerDraft: PlayerDraft;
  playerRows: AdminPlayer[];
  recomputeGameScoring: (silent?: boolean) => Promise<void>;
  reconcileAccess: () => Promise<void>;
  reconcileDeployments: () => Promise<void>;
  reconcileWireGuardGateway: () => Promise<void>;
  teardownAccess: () => Promise<void>;
  teardownWireGuardGateway: () => Promise<void>;
  refreshAccessStatus: (silent?: boolean) => Promise<void>;
  refreshAttacks: (silent?: boolean, filters?: AttackFilters) => Promise<void>;
  refreshCheckerRuns: (
    silent?: boolean,
    filters?: CheckerRunFilters,
  ) => Promise<void>;
  refreshGameScoreboard: (silent?: boolean) => Promise<void>;
  refreshGameStatus: (silent?: boolean) => Promise<void>;
  refreshSchedulerEvents: (
    silent?: boolean,
    filters?: SchedulerEventFilters,
  ) => Promise<void>;
  refreshWireGuardGatewayStatus: (silent?: boolean) => Promise<void>;
  resetAttackFilters: () => Promise<void>;
  resetCheckerRunFilters: () => Promise<void>;
  resetSchedulerEventFilters: () => Promise<void>;
  revokeWireGuard: (player: AdminPlayer) => Promise<void>;
  rotateWireGuard: (player: AdminPlayer) => Promise<void>;
  schedulerEventFilters: SchedulerEventFilters;
  schedulerEventPageState: AdminSchedulerEventPage;
  schedulerEventsLiveMode: boolean;
  scoreRows: AdminGameScoreRow[];
  selectedWireGuardPeer: AdminWireGuardPeer | null;
  setAttackFilters: (next: AttackFilters) => void;
  setChallengeDraft: (next: ChallengeDraft) => void;
  setCheckerRunFilters: (next: CheckerRunFilters) => void;
  setPlayerDraft: (next: PlayerDraft) => void;
  setSchedulerEventFilters: (next: SchedulerEventFilters) => void;
  setTeamDraft: (next: TeamDraft) => void;
  startGameMatch: () => Promise<void>;
  startGameScheduler: () => Promise<void>;
  stopGameMatch: () => Promise<void>;
  stopGameScheduler: () => Promise<void>;
  updateGameMatchSchedule: (schedule: {
    scheduledStartAt?: string;
    scheduledEndAt?: string;
  }) => Promise<void>;
  updateGameScheduler: (intervalSeconds: number) => Promise<void>;
  summary: OrganizerSummary;
  teamDraft: TeamDraft;
  teamRows: AdminTeam[];
  validateChallenge: (
    challenge: AdminChallenge,
    silent?: boolean,
  ) => Promise<AdminChallengeValidationResult>;
  wireGuardDialogOpen: boolean;
  wireGuardGatewayStatus: AdminWireGuardGatewayStatus | null;
  operationsStatus: AdminOperationsStatus | null;
  serviceMetrics: AdminServiceMetricSnapshot | null;
  advanceGameTick: () => Promise<void>;
  refreshOperationsStatus: (silent?: boolean) => Promise<void>;
  refreshServiceMetrics: (silent?: boolean) => Promise<void>;
};

export function useOrganizerDashboard({
  attackPage,
  challenges,
  checkerRunPage,
  deployments,
  gameStatus,
  operationsStatus,
  serviceMetrics,
  players,
  schedulerEventPage,
  scoreboard,
  teams,
}: OrganizerDashboardOptions): OrganizerDashboardState {
  const initialAttackFilters = useMemo<AttackFilters>(
    () => ({
      ...baseAttackFilters,
      limit: String(attackPage.limit ?? 12),
      offset: String(attackPage.offset ?? 0),
    }),
    [attackPage.limit, attackPage.offset],
  );
  const [teamRows, setTeamRows] = useState(teams);
  const [playerRows, setPlayerRows] = useState(players);
  const [challengeRows, setChallengeRows] = useState(challenges);
  const [deploymentRows, setDeploymentRows] = useState(deployments);
  const [gameState, setGameState] = useState(gameStatus);
  const [schedulerEventPageState, setSchedulerEventPageState] =
    useState<AdminSchedulerEventPage>(schedulerEventPage);
  const [checkerRunPageState, setCheckerRunPageState] =
    useState<AdminCheckerRunPage>(checkerRunPage);
  const [attackPageState, setAttackPageState] =
    useState<AdminAttackFeedPage>(attackPage);
  const [highlightedAttackIDs, setHighlightedAttackIDs] = useState<string[]>(
    [],
  );
  const [scoreRows, setScoreRows] = useState(scoreboard);
  const [challengeValidationRows, setChallengeValidationRows] = useState<
    Record<number, AdminChallengeValidationResult>
  >({});
  const [selectedWireGuardPeer, setSelectedWireGuardPeer] =
    useState<AdminWireGuardPeer | null>(null);
  const [accessStatus, setAccessStatus] =
    useState<AdminControllerAccessStatus | null>(null);
  const [wireGuardGatewayStatus, setWireGuardGatewayStatus] =
    useState<AdminWireGuardGatewayStatus | null>(null);
  const [operationsStatusState, setOperationsStatusState] =
    useState<AdminOperationsStatus | null>(operationsStatus ?? null);
  const [serviceMetricsState, setServiceMetricsState] =
    useState<AdminServiceMetricSnapshot | null>(serviceMetrics ?? null);
  const [wireGuardDialogOpen, setWireGuardDialogOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget | null>(null);
  const [formMode, setFormMode] = useState<FormMode>(null);
  const [formEntity, setFormEntity] = useState<FormEntity>(null);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [pendingAction, setPendingAction] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [actionNote, setActionNote] = useState<string | null>(null);
  const [teamDraft, setTeamDraft] = useState<TeamDraft>({
    name: "",
    contactEmail: "",
  });
  const [playerDraft, setPlayerDraft] = useState<PlayerDraft>({
    teamId: 0,
    displayName: "",
    email: "",
    password: "",
    role: "member",
  });
  const [challengeDraft, setChallengeDraft] = useState<ChallengeDraft>({
    name: "",
    baselineImage: "",
    checkerImage: "",
    servicePort: "",
    serviceSubnetOctet: "",
    weight: "1",
  });
  const [checkerRunFilters, setCheckerRunFilters] = useState<CheckerRunFilters>(
    defaultCheckerRunFilters,
  );
  const [attackFilters, setAttackFilters] =
    useState<AttackFilters>(initialAttackFilters);
  const [schedulerEventFilters, setSchedulerEventFilters] =
    useState<SchedulerEventFilters>(defaultSchedulerEventFilters);
  const attacksLiveRef = useRef(true);
  const attackPageRef = useRef<AdminAttackFeedPage>(attackPage);
  const attackHighlightTimeoutRef = useRef<number | null>(null);
  const checkerRunsLiveRef = useRef(true);
  const schedulerEventsLiveRef = useRef(true);

  const summary = useMemo(
    () => ({
      teams: teamRows.length,
      players: playerRows.length,
      publishedChallenges: challengeRows.filter(
        (challenge) => challenge.published,
      ).length,
      pendingDrafts: challengeRows.filter((challenge) => !challenge.published)
        .length,
      pendingDeployments: deploymentRows.filter(
        (deployment) => deployment.status !== "completed",
      ).length,
      totalTicks: gameState.total_ticks,
    }),
    [
      challengeRows,
      deploymentRows,
      gameState.total_ticks,
      playerRows,
      teamRows,
    ],
  );
  const checkerRunsLiveMode = useMemo(
    () => isDefaultCheckerRunFilters(checkerRunFilters),
    [checkerRunFilters],
  );
  const attackRealtimeEnabled = initialAttackFilters.offset === "0";
  const attacksLiveMode = useMemo(
    () =>
      attackRealtimeEnabled &&
      isDefaultAttackFilters(attackFilters, initialAttackFilters),
    [attackFilters, attackRealtimeEnabled, initialAttackFilters],
  );
  const schedulerEventsLiveMode = useMemo(
    () => isDefaultSchedulerEventFilters(schedulerEventFilters),
    [schedulerEventFilters],
  );

  useEffect(() => {
    void refreshWireGuardGatewayStatus(true);
    void refreshAccessStatus(true);
    void refreshOperationsStatus(true);
    void refreshServiceMetrics(true);
  }, []);

  useEffect(() => {
    const interval = window.setInterval(() => {
      void refreshOperationsStatus(true);
      void refreshServiceMetrics(true);
    }, 30000);

    const handleFocus = () => {
      void refreshOperationsStatus(true);
      void refreshServiceMetrics(true);
    };

    window.addEventListener("focus", handleFocus);
    return () => {
      window.clearInterval(interval);
      window.removeEventListener("focus", handleFocus);
    };
  }, []);

  useEffect(() => {
    attacksLiveRef.current = attacksLiveMode;
  }, [attacksLiveMode]);

  useEffect(() => {
    attackPageRef.current = attackPageState;
  }, [attackPageState]);

  useEffect(() => {
    return () => {
      if (attackHighlightTimeoutRef.current !== null) {
        window.clearTimeout(attackHighlightTimeoutRef.current);
      }
    };
  }, []);

  useEffect(() => {
    checkerRunsLiveRef.current = checkerRunsLiveMode;
  }, [checkerRunsLiveMode]);

  useEffect(() => {
    schedulerEventsLiveRef.current = schedulerEventsLiveMode;
  }, [schedulerEventsLiveMode]);

  useEffect(() => {
    const gameStatusSource = new EventSource(
      "/api/admin/realtime/game/status/stream",
    );
    const scoreboardSource = new EventSource(
      "/api/admin/realtime/game/scoreboard/stream",
    );
    const attacksSource = new EventSource(
      "/api/admin/realtime/game/attacks/stream",
    );
    const checkerRunsSource = new EventSource(
      "/api/admin/realtime/game/checker-runs/stream",
    );
    const schedulerEventsSource = new EventSource(
      "/api/admin/realtime/game/scheduler/events/stream",
    );

    gameStatusSource.onmessage = (event) => {
      try {
        setGameState(JSON.parse(event.data) as AdminGameStatus);
      } catch {
        // Keep the last good organizer snapshot if one frame is malformed.
      }
    };

    scoreboardSource.onmessage = (event) => {
      try {
        setScoreRows(JSON.parse(event.data) as AdminGameScoreRow[]);
      } catch {
        // Keep the last good organizer snapshot if one frame is malformed.
      }
    };

    attacksSource.onmessage = (event) => {
      if (!attacksLiveRef.current) {
        return;
      }

      try {
        const nextPage = JSON.parse(event.data) as AdminAttackFeedPage;
        const mergedPage = mergeRealtimeAdminAttackPage(
          attackPageRef.current,
          nextPage,
          attackFilters.limit,
        );
        const nextHighlights = collectNewAdminAttackIDs(
          attackPageRef.current,
          mergedPage,
        );
        setAttackPageState(mergedPage);
        if (nextHighlights.length > 0) {
          scheduleAttackHighlights(nextHighlights);
        }
      } catch {
        // Keep the last good organizer snapshot if one frame is malformed.
      }
    };

    checkerRunsSource.onmessage = (event) => {
      if (!checkerRunsLiveRef.current) {
        return;
      }

      try {
        setCheckerRunPageState(JSON.parse(event.data) as AdminCheckerRunPage);
      } catch {
        // Keep the last good organizer snapshot if one frame is malformed.
      }
    };

    schedulerEventsSource.onmessage = (event) => {
      if (!schedulerEventsLiveRef.current) {
        return;
      }

      try {
        setSchedulerEventPageState(
          JSON.parse(event.data) as AdminSchedulerEventPage,
        );
      } catch {
        // Keep the last good organizer snapshot if one frame is malformed.
      }
    };

    return () => {
      gameStatusSource.close();
      scoreboardSource.close();
      attacksSource.close();
      checkerRunsSource.close();
      schedulerEventsSource.close();
    };
  }, [attackFilters.limit]);

  async function createTeam(): Promise<void> {
    setPendingAction("team:create");
    setActionError(null);
    setActionNote(null);

    try {
      const response = await fetch("/api/admin/teams", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: teamDraft.name,
          contact_email: teamDraft.contactEmail,
        }),
      });
      const payload = (await response.json()) as ActionEnvelope<AdminTeam>;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload ? payload.message : "team create failed",
        );
      }

      setTeamRows((current) => [...current, payload.data]);
      setChallengeRows((current) =>
        current.map((challenge) =>
          challenge.published
            ? {
                ...challenge,
                total_teams: challenge.total_teams + 1,
                deployed_teams: challenge.deployed_teams + 1,
                ready_teams: challenge.ready_teams + 1,
              }
            : { ...challenge, total_teams: challenge.total_teams + 1 },
        ),
      );
      await refreshGameScoreboard(true);
      setTeamDraft({ name: "", contactEmail: "" });
      setPlayerDraft((current) => ({
        ...current,
        teamId: current.teamId || payload.data.id,
      }));
      setActionNote(
        `Created ${payload.data.name} and seeded deployed services for published challenges.`,
      );
    } catch (error) {
      setActionError(
        error instanceof Error ? error.message : "team create failed",
      );
    } finally {
      setPendingAction(null);
    }
  }

  async function createPlayer(): Promise<void> {
    if (!playerDraft.teamId) {
      setActionError("select a team before creating a player");
      return;
    }

    setPendingAction("player:create");
    setActionError(null);
    setActionNote(null);

    try {
      const response = await fetch("/api/admin/players", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          team_id: playerDraft.teamId,
          display_name: playerDraft.displayName,
          email: playerDraft.email,
          password: playerDraft.password,
          role: playerDraft.role,
        }),
      });
      const payload = (await response.json()) as ActionEnvelope<AdminPlayer>;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload ? payload.message : "player create failed",
        );
      }

      setPlayerRows((current) => [...current, payload.data]);
      setTeamRows((current) =>
        current.map((team) =>
          team.id === payload.data.team_id
            ? { ...team, player_count: team.player_count + 1 }
            : team,
        ),
      );
      setPlayerDraft((current) => ({
        ...current,
        displayName: "",
        email: "",
        password: "",
      }));
      setActionNote(
        `Created player ${payload.data.display_name} with peer ${payload.data.wireguard_peer}. Reconcile the WireGuard gateway and service access policy if that team already has unlocked services.`,
      );
    } catch (error) {
      setActionError(
        error instanceof Error ? error.message : "player create failed",
      );
    } finally {
      setPendingAction(null);
    }
  }

  function applyWireGuardUpdate(peer: AdminWireGuardPeer): void {
    setPlayerRows((current) =>
      current.map((player) =>
        player.id === peer.player_id
          ? {
              ...player,
              wireguard_peer: peer.wireguard_peer,
              wireguard_address: peer.address,
              wireguard_status: peer.status,
              wireguard_issued_at: peer.issued_at,
              wireguard_revoked_at: peer.revoked_at,
            }
          : player,
      ),
    );
    setSelectedWireGuardPeer((current) =>
      current?.player_id === peer.player_id ? peer : current,
    );
  }

  async function refreshWireGuardGatewayStatus(silent = false): Promise<void> {
    if (!silent) {
      setPendingAction("wireguard-gateway:status");
      setActionError(null);
      setActionNote(null);
    }

    try {
      const response = await fetch("/api/admin/wireguard/status");
      const payload =
        (await response.json()) as ActionEnvelope<AdminWireGuardGatewayStatus>;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload
            ? payload.message
            : "wireguard gateway status failed",
        );
      }

      setWireGuardGatewayStatus(payload.data);
      if (!silent) {
        setActionNote(
          `WireGuard gateway is ${payload.data.state} in ${payload.data.mode} mode.`,
        );
      }
    } catch (error) {
      if (!silent) {
        setActionError(
          error instanceof Error
            ? error.message
            : "wireguard gateway status failed",
        );
      }
    } finally {
      if (!silent) {
        setPendingAction(null);
      }
    }
  }

  async function refreshAccessStatus(silent = false): Promise<void> {
    if (!silent) {
      setPendingAction("access:status");
      setActionError(null);
      setActionNote(null);
    }

    try {
      const response = await fetch("/api/admin/access/status");
      const payload =
        (await response.json()) as ActionEnvelope<AdminControllerAccessStatus>;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload
            ? payload.message
            : "controller access status failed",
        );
      }
      setAccessStatus(payload.data);
      if (!silent) {
        setActionNote(
          `Controller access policy is ${payload.data.state} in ${payload.data.mode} mode.`,
        );
      }
    } catch (error) {
      if (!silent) {
        setActionError(
          error instanceof Error
            ? error.message
            : "controller access status failed",
        );
      }
    } finally {
      if (!silent) {
        setPendingAction(null);
      }
    }
  }

  async function refreshOperationsStatus(silent = false): Promise<void> {
    if (!silent) {
      setPendingAction("operations:status");
      setActionError(null);
      setActionNote(null);
    }

    try {
      const response = await fetch("/api/admin/operations/status", {
        cache: "no-store",
      });
      const payload =
        (await response.json()) as ActionEnvelope<AdminOperationsStatus>;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload
            ? payload.message
            : "operations status failed",
        );
      }
      setOperationsStatusState(payload.data);
      if (!silent) {
        setActionNote(
          payload.data.alerts.length === 0
            ? "Runtime operations look healthy."
            : `Loaded ${payload.data.alerts.length} runtime alert(s).`,
        );
      }
    } catch (error) {
      if (!silent) {
        setActionError(
          error instanceof Error ? error.message : "operations status failed",
        );
      }
    } finally {
      if (!silent) {
        setPendingAction(null);
      }
    }
  }

  async function refreshServiceMetrics(silent = false): Promise<void> {
    if (!silent) {
      setPendingAction("operations:metrics");
      setActionError(null);
      setActionNote(null);
    }

    try {
      const response = await fetch("/api/admin/operations/metrics", {
        cache: "no-store",
      });
      const payload =
        (await response.json()) as ActionEnvelope<AdminServiceMetricSnapshot>;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload
            ? payload.message
            : "operations metrics failed",
        );
      }
      setServiceMetricsState(payload.data);
      if (!silent) {
        setActionNote("Loaded live service metrics.");
      }
    } catch (error) {
      if (!silent) {
        setActionError(
          error instanceof Error
            ? error.message
            : "operations metrics failed",
        );
      }
    } finally {
      if (!silent) {
        setPendingAction(null);
      }
    }
  }

  function downloadWireGuardConfig(peer: AdminWireGuardPeer): void {
    const blob = new Blob([peer.config], { type: "text/plain;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = peer.download_name;
    anchor.click();
    URL.revokeObjectURL(url);
  }

  async function inspectWireGuard(player: AdminPlayer): Promise<void> {
    setPendingAction(`wireguard:get:${player.id}`);
    setActionError(null);
    setActionNote(null);

    try {
      const response = await fetch(`/api/admin/players/${player.id}/wireguard`);
      const payload =
        (await response.json()) as ActionEnvelope<AdminWireGuardPeer>;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload
            ? payload.message
            : "wireguard config fetch failed",
        );
      }

      applyWireGuardUpdate(payload.data);
      setSelectedWireGuardPeer(payload.data);
      setWireGuardDialogOpen(true);
      setActionNote(`Loaded WireGuard config for ${player.display_name}.`);
    } catch (error) {
      setActionError(
        error instanceof Error
          ? error.message
          : "wireguard config fetch failed",
      );
    } finally {
      setPendingAction(null);
    }
  }

  async function rotateWireGuard(player: AdminPlayer): Promise<void> {
    setPendingAction(`wireguard:rotate:${player.id}`);
    setActionError(null);
    setActionNote(null);

    try {
      const response = await fetch(
        `/api/admin/players/${player.id}/wireguard/rotate`,
        {
          method: "POST",
        },
      );
      const payload =
        (await response.json()) as ActionEnvelope<AdminWireGuardPeer>;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload ? payload.message : "wireguard rotate failed",
        );
      }

      applyWireGuardUpdate(payload.data);
      setSelectedWireGuardPeer(payload.data);
      setWireGuardDialogOpen(true);
      setActionNote(
        `Rotated WireGuard config for ${player.display_name}. Reconcile the WireGuard gateway so the old peer material stops working.`,
      );
    } catch (error) {
      setActionError(
        error instanceof Error ? error.message : "wireguard rotate failed",
      );
    } finally {
      setPendingAction(null);
    }
  }

  async function revokeWireGuard(player: AdminPlayer): Promise<void> {
    setPendingAction(`wireguard:revoke:${player.id}`);
    setActionError(null);
    setActionNote(null);

    try {
      const response = await fetch(
        `/api/admin/players/${player.id}/wireguard/revoke`,
        {
          method: "POST",
        },
      );
      const payload =
        (await response.json()) as ActionEnvelope<AdminWireGuardPeer>;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload ? payload.message : "wireguard revoke failed",
        );
      }

      applyWireGuardUpdate(payload.data);
      setSelectedWireGuardPeer(payload.data);
      setActionNote(
        `Revoked WireGuard config for ${player.display_name}. Reconcile the WireGuard gateway and service access policy to remove the peer from runtime access.`,
      );
    } catch (error) {
      setActionError(
        error instanceof Error ? error.message : "wireguard revoke failed",
      );
    } finally {
      setPendingAction(null);
    }
  }

  async function reconcileWireGuardGateway(): Promise<void> {
    setPendingAction("wireguard-gateway:reconcile");
    setActionError(null);
    setActionNote(null);

    try {
      const response = await fetch("/api/admin/wireguard/reconcile", {
        method: "POST",
      });
      const payload =
        (await response.json()) as ActionEnvelope<AdminWireGuardGatewayStatus>;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload
            ? payload.message
            : "wireguard gateway reconcile failed",
        );
      }

      setWireGuardGatewayStatus(payload.data);
      setActionNote(
        `WireGuard gateway applied revision ${payload.data.revision ?? "n/a"} with ${payload.data.peers_active} active peer(s) and ${payload.data.peers_revoked} revoked peer(s).`,
      );
    } catch (error) {
      setActionError(
        error instanceof Error
          ? error.message
          : "wireguard gateway reconcile failed",
      );
    } finally {
      setPendingAction(null);
    }
  }

  async function reconcileAccess(): Promise<void> {
    setPendingAction("access:reconcile");
    setActionError(null);
    setActionNote(null);

    try {
      const response = await fetch("/api/admin/access/reconcile", {
        method: "POST",
      });
      const payload =
        (await response.json()) as ActionEnvelope<AdminControllerAccessStatus>;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload
            ? payload.message
            : "controller access reconcile failed",
        );
      }
      setAccessStatus(payload.data);
      setActionNote(
        `Controller access applied revision ${payload.data.revision ?? "n/a"} across ${payload.data.policies_total} service policy row(s).`,
      );
    } catch (error) {
      setActionError(
        error instanceof Error
          ? error.message
          : "controller access reconcile failed",
      );
    } finally {
      setPendingAction(null);
    }
  }

  async function teardownWireGuardGateway(): Promise<void> {
    if (
      !confirm(
        "Are you sure you want to TEARDOWN the WireGuard gateway? This will remove ALL peer access.",
      )
    ) {
      return;
    }

    setPendingAction("wireguard-gateway:teardown");
    setActionError(null);
    setActionNote(null);

    try {
      const response = await fetch("/api/admin/wireguard/teardown", {
        method: "POST",
      });
      const payload = (await response.json()) as ActionEnvelope<void>;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload
            ? payload.message
            : "wireguard gateway teardown failed",
        );
      }

      await refreshWireGuardGatewayStatus(true);
      setActionNote("WireGuard gateway rules and interface torn down.");
    } catch (error) {
      setActionError(
        error instanceof Error
          ? error.message
          : "wireguard gateway teardown failed",
      );
    } finally {
      setPendingAction(null);
    }
  }

  async function teardownAccess(): Promise<void> {
    if (
      !confirm(
        "Are you sure you want to TEARDOWN the service access policies? This will remove ALL SSH and service access rules.",
      )
    ) {
      return;
    }

    setPendingAction("access:teardown");
    setActionError(null);
    setActionNote(null);

    try {
      const response = await fetch("/api/admin/access/teardown", {
        method: "POST",
      });
      const payload = (await response.json()) as ActionEnvelope<void>;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload
            ? payload.message
            : "controller access teardown failed",
        );
      }

      await refreshAccessStatus(true);
      setActionNote("Controller service access rules torn down.");
    } catch (error) {
      setActionError(
        error instanceof Error
          ? error.message
          : "controller access teardown failed",
      );
    } finally {
      setPendingAction(null);
    }
  }

  async function deleteTeam(team: AdminTeam): Promise<void> {
    setPendingAction(`team:delete:${team.id}`);
    setActionError(null);
    setActionNote(null);

    try {
      const response = await fetch(`/api/admin/teams/${team.id}`, {
        method: "DELETE",
      });
      const payload = (await response.json()) as ActionEnvelope<void>;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload ? payload.message : "team delete failed",
        );
      }

      setTeamRows((current) => current.filter((item) => item.id !== team.id));
      setPlayerRows((current) =>
        current.filter((item) => item.team_id !== team.id),
      );
      setActionNote(`Team "${team.name}" deleted.`);
    } catch (error) {
      setActionError(
        error instanceof Error ? error.message : "team delete failed",
      );
    } finally {
      setPendingAction(null);
    }
  }

  async function deletePlayer(player: AdminPlayer): Promise<void> {
    setPendingAction(`player:delete:${player.id}`);
    setActionError(null);
    setActionNote(null);

    try {
      const response = await fetch(`/api/admin/players/${player.id}`, {
        method: "DELETE",
      });
      const payload = (await response.json()) as ActionEnvelope<void>;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload ? payload.message : "player delete failed",
        );
      }

      setPlayerRows((current) =>
        current.filter((item) => item.id !== player.id),
      );
      setActionNote(`Player "${player.display_name}" deleted.`);
    } catch (error) {
      setActionError(
        error instanceof Error ? error.message : "player delete failed",
      );
    } finally {
      setPendingAction(null);
    }
  }

  async function deleteChallenge(challenge: AdminChallenge): Promise<void> {
    setPendingAction(`challenge:delete:${challenge.id}`);
    setActionError(null);
    setActionNote(null);

    try {
      const response = await fetch(`/api/admin/challenges/${challenge.id}`, {
        method: "DELETE",
      });
      const payload = (await response.json()) as ActionEnvelope<void>;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload ? payload.message : "challenge delete failed",
        );
      }

      setChallengeRows((current) =>
        current.filter((item) => item.id !== challenge.id),
      );
      setActionNote(`Challenge "${challenge.name}" deleted.`);
    } catch (error) {
      setActionError(
        error instanceof Error ? error.message : "challenge delete failed",
      );
    } finally {
      setPendingAction(null);
    }
  }

  async function deleteDeployment(deployment: AdminDeploymentJob): Promise<void> {
    setPendingAction(`deployment:delete:${deployment.id}`);
    setActionError(null);
    setActionNote(null);

    try {
      const response = await fetch(`/api/admin/deployments/${deployment.id}`, {
        method: "DELETE",
      });
      const payload = (await response.json()) as ActionEnvelope<void>;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload ? payload.message : "deployment delete failed",
        );
      }

      setDeploymentRows((current) =>
        current.filter((item) => item.id !== deployment.id),
      );
      setActionNote(`Deployment job #${deployment.id} deleted.`);
    } catch (error) {
      setActionError(
        error instanceof Error ? error.message : "deployment delete failed",
      );
    } finally {
      setPendingAction(null);
    }
  }

  async function createChallenge(): Promise<void> {
    setPendingAction("challenge:create");
    setActionError(null);
    setActionNote(null);

    try {
      const response = await fetch("/api/admin/challenges", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: challengeDraft.name,
          baseline_image: challengeDraft.baselineImage,
          checker_image: challengeDraft.checkerImage,
          weight: Number(challengeDraft.weight) || 1,
          service_port:
            challengeDraft.servicePort.trim() === ""
              ? undefined
              : Number(challengeDraft.servicePort),
          service_subnet_octet:
            challengeDraft.serviceSubnetOctet.trim() === ""
              ? undefined
              : Number(challengeDraft.serviceSubnetOctet),
        }),
      });
      const payload = (await response.json()) as ActionEnvelope<AdminChallenge>;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload ? payload.message : "challenge create failed",
        );
      }

      setChallengeRows((current) => [...current, payload.data]);
      setChallengeValidationRows((current) => {
        const next = { ...current };
        delete next[payload.data.id];
        return next;
      });
      setChallengeDraft({
        name: "",
        baselineImage: "",
        checkerImage: "",
        servicePort: "",
        serviceSubnetOctet: "",
        weight: "1",
      });
      setActionNote(
        `Created draft challenge ${payload.data.name}. Deploy it to replicate one service per team.`,
      );
    } catch (error) {
      setActionError(
        error instanceof Error ? error.message : "challenge create failed",
      );
    } finally {
      setPendingAction(null);
    }
  }

  function applyChallengeValidation(
    result: AdminChallengeValidationResult,
  ): void {
    setChallengeValidationRows((current) => ({
      ...current,
      [result.challenge_id]: result,
    }));
  }

  async function validateChallenge(
    challenge: AdminChallenge,
    silent = false,
  ): Promise<AdminChallengeValidationResult> {
    if (!silent) {
      setPendingAction(`validate:${challenge.id}`);
      setActionError(null);
      setActionNote(null);
    }

    try {
      const response = await fetch(
        `/api/admin/challenges/${challenge.id}/validate`,
        {
          method: "POST",
        },
      );
      const payload =
        (await response.json()) as ActionEnvelope<AdminChallengeValidationResult>;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload
            ? payload.message
            : "challenge validation failed",
        );
      }

      applyChallengeValidation(payload.data);
      if (!silent) {
        setActionNote(
          payload.data.status === "valid"
            ? `${challenge.name} service and checker images satisfy the current runtime package policy.`
            : payload.data.message ||
                `${challenge.name} failed runtime validation.`,
        );
      }
      return payload.data;
    } catch (error) {
      if (!silent) {
        setActionError(
          error instanceof Error
            ? error.message
            : "challenge validation failed",
        );
      }
      throw error;
    } finally {
      if (!silent) {
        setPendingAction(null);
      }
    }
  }

  async function deployChallenge(challenge: AdminChallenge): Promise<void> {
    setPendingAction(`deploy:${challenge.id}`);
    setActionError(null);
    setActionNote(null);

    try {
      const validation = await validateChallenge(challenge, true);
      applyChallengeValidation(validation);
      if (
        validation.status !== "valid" ||
        !validation.baseline_ssh_contract_ok ||
        !validation.checker_contract_ok
      ) {
        throw new Error(
          validation.message || "challenge package failed runtime validation",
        );
      }

      const response = await fetch(
        `/api/admin/challenges/${challenge.id}/deploy`,
        {
          method: "POST",
        },
      );
      const payload =
        (await response.json()) as ActionEnvelope<AdminDeployment>;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload ? payload.message : "challenge deploy failed",
        );
      }

      setChallengeRows((current) =>
        current.map((item) =>
          item.id === challenge.id
            ? {
                ...item,
                published: true,
                deployed_teams: payload.data.total_team_count,
                total_teams: payload.data.total_team_count,
                runtime_status:
                  payload.data.queued_team_count > 0 ? "deploying" : "ready",
                queued_teams: payload.data.queued_team_count,
                ready_teams: payload.data.ready_team_count,
              }
            : item,
        ),
      );

      if (payload.data.job_id > 0) {
        setDeploymentRows((current) => [
          {
            id: payload.data.job_id,
            challenge_id: payload.data.challenge_id,
            challenge_name: payload.data.challenge_name,
            status: payload.data.status,
            target_team_count: payload.data.total_team_count,
            queued_team_count: payload.data.queued_team_count,
            ready_team_count: payload.data.ready_team_count,
            failed_team_count: 0,
            created_at: payload.data.created_at,
            completed_at: payload.data.completed_at,
          },
          ...current.map((deployment) =>
            deployment.challenge_id === payload.data.challenge_id &&
            deployment.id !== payload.data.job_id &&
            (deployment.status === "queued" || deployment.status === "running")
              ? {
                  ...deployment,
                  status: "superseded",
                  queued_team_count: 0,
                  completed_at:
                    payload.data.created_at ?? new Date().toISOString(),
                }
              : deployment,
          ),
        ]);
      } else {
        setDeploymentRows((current) =>
          current.map((deployment) =>
            deployment.challenge_id === payload.data.challenge_id &&
            (deployment.status === "queued" || deployment.status === "running")
              ? {
                  ...deployment,
                  status: "superseded",
                  queued_team_count: 0,
                  completed_at:
                    payload.data.created_at ?? new Date().toISOString(),
                }
              : deployment,
          ),
        );
      }

      if (!challenge.published) {
        setTeamRows((current) =>
          current.map((team) => ({
            ...team,
            deployed_challenges: team.deployed_challenges + 1,
          })),
        );
      }

      setActionNote(
        payload.data.status === "queued"
          ? `Queued ${payload.data.challenge_name} for ${payload.data.deployed_team_count} team runtimes. Reconcile to mark the rollout ready.`
          : `${payload.data.challenge_name} was already fully deployed across all teams.`,
      );
    } catch (error) {
      setActionError(
        error instanceof Error ? error.message : "challenge deploy failed",
      );
    } finally {
      setPendingAction(null);
    }
  }

  async function reconcileDeployments(): Promise<void> {
    setPendingAction("deployments:reconcile");
    setActionError(null);
    setActionNote(null);

    try {
      const reconcileResponse = await fetch("/api/admin/deployments/reconcile", {
        method: "POST",
      });
      const payload =
        (await reconcileResponse.json()) as ActionEnvelope<AdminReconcileResult>;
      if (!reconcileResponse.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload
            ? payload.message
            : "deployment reconcile failed",
        );
      }

      const deploymentsResponse = await fetch("/api/admin/deployments");
      const deploymentsPayload =
        (await deploymentsResponse.json()) as ActionEnvelope<AdminDeploymentJob[]>;
      if (
        !deploymentsResponse.ok ||
        deploymentsPayload.status !== "success"
      ) {
        throw new Error(
          "message" in deploymentsPayload
            ? deploymentsPayload.message
            : "deployment list refresh failed",
        );
      }

      setDeploymentRows(deploymentsPayload.data);

      setActionNote(
        `Controller reconcile processed ${payload.data.processed_jobs} job(s), advanced ${payload.data.processed_instances} team service instance(s), and refreshed the deployment queue.`,
      );
    } catch (error) {
      setActionError(
        error instanceof Error ? error.message : "deployment reconcile failed",
      );
    } finally {
      setPendingAction(null);
    }
  }

  async function refreshGameStatus(silent = false): Promise<void> {
    if (!silent) {
      setPendingAction("game:status");
      setActionError(null);
      setActionNote(null);
    }

    try {
      const response = await fetch("/api/admin/game/status");
      const payload =
        (await response.json()) as ActionEnvelope<AdminGameStatus>;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload ? payload.message : "game-core status failed",
        );
      }

      setGameState(payload.data);
      if (!silent) {
        setActionNote(
          `Game-core reports match ${payload.data.match?.state ?? "unknown"} with ${payload.data.total_ticks} persisted tick(s).`,
        );
      }
    } catch (error) {
      if (!silent) {
        setActionError(
          error instanceof Error ? error.message : "game-core status failed",
        );
      }
    } finally {
      if (!silent) {
        setPendingAction(null);
      }
    }
  }

  async function startGameMatch(): Promise<void> {
    setPendingAction("game:match:start");
    setActionError(null);
    setActionNote(null);

    let matchStarted = false;

    try {
      const matchResponse = await fetch("/api/admin/game/match/start", {
        method: "POST",
      });
      const matchPayload = (await matchResponse.json()) as ActionEnvelope<
        AdminGameStatus["match"]
      >;
      if (!matchResponse.ok || matchPayload.status !== "success") {
        throw new Error(
          "message" in matchPayload
            ? matchPayload.message
            : "game match start failed",
        );
      }

      matchStarted = true;
      setGameState((current) => ({ ...current, match: matchPayload.data }));

      const schedulerResponse = await fetch("/api/admin/game/scheduler/start", {
        method: "POST",
      });
      const schedulerPayload = (await schedulerResponse.json()) as ActionEnvelope<
        AdminGameStatus["scheduler"]
      >;
      if (!schedulerResponse.ok || schedulerPayload.status !== "success") {
        throw new Error(
          "message" in schedulerPayload
            ? schedulerPayload.message
            : "game-core scheduler start failed",
        );
      }

      setGameState((current) => ({
        ...current,
        match: matchPayload.data,
        scheduler: schedulerPayload.data,
      }));
      await refreshSchedulerEvents(true);
      setActionNote(
        `Game started. Submissions are ${matchPayload.data?.accepting_submissions ? "open" : "closed"} and the scheduler is running at ${schedulerPayload.data?.interval_seconds ?? 0}s.`,
      );
    } catch (error) {
      setActionError(
        error instanceof Error
          ? matchStarted
            ? `Match started, but scheduler start failed: ${error.message}`
            : error.message
          : "game start failed",
      );
    } finally {
      setPendingAction(null);
    }
  }

  async function stopGameMatch(): Promise<void> {
    setPendingAction("game:match:stop");
    setActionError(null);
    setActionNote(null);

    try {
      const response = await fetch("/api/admin/game/match/stop", {
        method: "POST",
      });
      const payload = (await response.json()) as ActionEnvelope<
        AdminGameStatus["match"]
      >;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload ? payload.message : "game match stop failed",
        );
      }

      setGameState((current) => ({
        ...current,
        match: payload.data,
        scheduler: {
          ...(current.scheduler ?? { interval_seconds: 60 }),
          state: "stopped",
          next_run_at: "",
        },
      }));
      await refreshSchedulerEvents(true);
      setActionNote("Match stopped. Participant submissions are now closed.");
    } catch (error) {
      setActionError(
        error instanceof Error ? error.message : "game match stop failed",
      );
    } finally {
      setPendingAction(null);
    }
  }

  async function startGameScheduler(): Promise<void> {
    setPendingAction("game:scheduler:start");
    setActionError(null);
    setActionNote(null);

    try {
      const response = await fetch("/api/admin/game/scheduler/start", {
        method: "POST",
      });
      const payload = (await response.json()) as ActionEnvelope<
        AdminGameStatus["scheduler"]
      >;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload
            ? payload.message
            : "game-core scheduler start failed",
        );
      }

      setGameState((current) => ({ ...current, scheduler: payload.data }));
      await refreshSchedulerEvents(true);
      setActionNote(
        `Scheduler started with ${payload.data?.interval_seconds ?? 0}s interval.`,
      );
    } catch (error) {
      setActionError(
        error instanceof Error
          ? error.message
          : "game-core scheduler start failed",
      );
    } finally {
      setPendingAction(null);
    }
  }

  async function stopGameScheduler(): Promise<void> {
    setPendingAction("game:scheduler:stop");
    setActionError(null);
    setActionNote(null);

    try {
      const response = await fetch("/api/admin/game/scheduler/stop", {
        method: "POST",
      });
      const payload = (await response.json()) as ActionEnvelope<
        AdminGameStatus["scheduler"]
      >;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload
            ? payload.message
            : "game-core scheduler stop failed",
        );
      }

      setGameState((current) => ({ ...current, scheduler: payload.data }));
      await refreshSchedulerEvents(true);
      setActionNote(
        "Scheduler stopped. Manual tick advance remains available.",
      );
    } catch (error) {
      setActionError(
        error instanceof Error
          ? error.message
          : "game-core scheduler stop failed",
      );
    } finally {
      setPendingAction(null);
    }
  }

  async function updateGameMatchSchedule(schedule: {
    scheduledStartAt?: string;
    scheduledEndAt?: string;
  }): Promise<void> {
    setPendingAction("game:match:schedule");
    setActionError(null);
    setActionNote(null);

    try {
      const response = await fetch("/api/admin/game/match/schedule", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          scheduled_start_at: schedule.scheduledStartAt,
          scheduled_end_at: schedule.scheduledEndAt,
        }),
      });
      const payload = (await response.json()) as ActionEnvelope<
        AdminGameStatus["match"]
      >;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload
            ? payload.message
            : "game match schedule update failed",
        );
      }

      setGameState((current) => ({ ...current, match: payload.data }));
      setActionNote(
        `Match window updated. Start: ${payload.data?.scheduled_start_at ?? "manual"}; end: ${payload.data?.scheduled_end_at ?? "manual"}.`,
      );
    } catch (error) {
      setActionError(
        error instanceof Error
          ? error.message
          : "game match schedule update failed",
      );
    } finally {
      setPendingAction(null);
    }
  }

  async function updateGameScheduler(intervalSeconds: number): Promise<void> {
    setPendingAction("game:scheduler:update");
    setActionError(null);
    setActionNote(null);

    try {
      const response = await fetch("/api/admin/game/scheduler/interval", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ interval_seconds: intervalSeconds }),
      });
      const payload = (await response.json()) as ActionEnvelope<
        AdminGameStatus["scheduler"]
      >;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload
            ? payload.message
            : "game-core scheduler update failed",
        );
      }

      setGameState((current) => ({ ...current, scheduler: payload.data }));
      setActionNote(
        `Scheduler interval updated to ${intervalSeconds} seconds.`,
      );
    } catch (error) {
      setActionError(
        error instanceof Error
          ? error.message
          : "game-core scheduler update failed",
      );
    } finally {
      setPendingAction(null);
    }
  }

  function scheduleAttackHighlights(ids: string[]): void {
    if (attackHighlightTimeoutRef.current !== null) {
      window.clearTimeout(attackHighlightTimeoutRef.current);
    }

    setHighlightedAttackIDs(ids);
    attackHighlightTimeoutRef.current = window.setTimeout(() => {
      setHighlightedAttackIDs([]);
      attackHighlightTimeoutRef.current = null;
    }, 4000);
  }

  function clearAttackHighlights(): void {
    if (attackHighlightTimeoutRef.current !== null) {
      window.clearTimeout(attackHighlightTimeoutRef.current);
      attackHighlightTimeoutRef.current = null;
    }
    setHighlightedAttackIDs([]);
  }

  async function refreshAttacks(
    silent = false,
    filters: AttackFilters = attackFilters,
  ): Promise<void> {
    if (!silent) {
      setPendingAction("game:attacks");
      setActionError(null);
      setActionNote(null);
    }

    try {
      const response = await fetch(
        `/api/admin/game/attacks${buildAttackQueryString(filters)}`,
      );
      const payload =
        (await response.json()) as ActionEnvelope<AdminAttackFeedPage>;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload ? payload.message : "attack feed fetch failed",
        );
      }

      clearAttackHighlights();
      setAttackPageState(payload.data);
      if (!silent) {
        setActionNote(
          `Loaded ${payload.data.items.length} accepted attack row(s) (${payload.data.offset + 1}-${payload.data.offset + payload.data.items.length} of ${payload.data.total_count}).`,
        );
      }
    } catch (error) {
      if (!silent) {
        setActionError(
          error instanceof Error ? error.message : "attack feed fetch failed",
        );
      }
    } finally {
      if (!silent) {
        setPendingAction(null);
      }
    }
  }

  async function refreshCheckerRuns(
    silent = false,
    filters: CheckerRunFilters = checkerRunFilters,
  ): Promise<void> {
    if (!silent) {
      setPendingAction("game:checker-runs");
      setActionError(null);
      setActionNote(null);
    }

    try {
      const response = await fetch(
        `/api/admin/game/checker-runs${buildCheckerRunQueryString(filters)}`,
      );
      const payload =
        (await response.json()) as ActionEnvelope<AdminCheckerRunPage>;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload ? payload.message : "checker runs fetch failed",
        );
      }

      setCheckerRunPageState(payload.data);
      if (!silent) {
        setActionNote(
          `Loaded ${payload.data.items.length} checker run row(s) from game-core (${payload.data.offset + 1}-${payload.data.offset + payload.data.items.length} of ${payload.data.total_count}).`,
        );
      }
    } catch (error) {
      if (!silent) {
        setActionError(
          error instanceof Error ? error.message : "checker runs fetch failed",
        );
      }
    } finally {
      if (!silent) {
        setPendingAction(null);
      }
    }
  }

  async function refreshSchedulerEvents(
    silent = false,
    filters: SchedulerEventFilters = schedulerEventFilters,
  ): Promise<void> {
    if (!silent) {
      setPendingAction("game:scheduler-events");
      setActionError(null);
      setActionNote(null);
    }

    try {
      const response = await fetch(
        `/api/admin/game/scheduler/events${buildSchedulerEventQueryString(filters)}`,
      );
      const payload =
        (await response.json()) as ActionEnvelope<AdminSchedulerEventPage>;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload
            ? payload.message
            : "scheduler events fetch failed",
        );
      }

      setSchedulerEventPageState(payload.data);
      if (!silent) {
        setActionNote(
          `Loaded ${payload.data.items.length} scheduler event row(s) from game-core (${payload.data.offset + 1}-${payload.data.offset + payload.data.items.length} of ${payload.data.total_count}).`,
        );
      }
    } catch (error) {
      if (!silent) {
        setActionError(
          error instanceof Error
            ? error.message
            : "scheduler events fetch failed",
        );
      }
    } finally {
      if (!silent) {
        setPendingAction(null);
      }
    }
  }

  async function applyCheckerRunFilters(): Promise<void> {
    const next = { ...checkerRunFilters, offset: "0" };
    setCheckerRunFilters(next);
    await refreshCheckerRuns(false, next);
  }

  async function resetCheckerRunFilters(): Promise<void> {
    setCheckerRunFilters(defaultCheckerRunFilters);
    await refreshCheckerRuns(false, defaultCheckerRunFilters);
  }

  async function pageCheckerRuns(direction: "prev" | "next"): Promise<void> {
    const limit = parsePositiveInteger(checkerRunFilters.limit, 18, 200) ?? 18;
    const currentOffset = parseNonNegativeInteger(checkerRunFilters.offset);
    const nextOffset =
      direction === "prev"
        ? Math.max(0, currentOffset - limit)
        : currentOffset + limit;
    const next = { ...checkerRunFilters, offset: String(nextOffset) };
    setCheckerRunFilters(next);
    await refreshCheckerRuns(false, next);
  }

  async function applyAttackFilters(): Promise<void> {
    const next = { ...attackFilters, offset: "0" };
    setAttackFilters(next);
    await refreshAttacks(false, next);
  }

  async function resetAttackFilters(): Promise<void> {
    setAttackFilters(initialAttackFilters);
    await refreshAttacks(false, initialAttackFilters);
  }

  async function pageAttacks(direction: "prev" | "next"): Promise<void> {
    const limit = parsePositiveInteger(attackFilters.limit, 12, 200) ?? 12;
    const currentOffset = parseNonNegativeInteger(attackFilters.offset);
    const nextOffset =
      direction === "prev"
        ? Math.max(0, currentOffset - limit)
        : currentOffset + limit;
    const next = { ...attackFilters, offset: String(nextOffset) };
    setAttackFilters(next);
    await refreshAttacks(false, next);
  }

  async function applySchedulerEventFilters(): Promise<void> {
    const next = { ...schedulerEventFilters, offset: "0" };
    setSchedulerEventFilters(next);
    await refreshSchedulerEvents(false, next);
  }

  async function resetSchedulerEventFilters(): Promise<void> {
    setSchedulerEventFilters(defaultSchedulerEventFilters);
    await refreshSchedulerEvents(false, defaultSchedulerEventFilters);
  }

  async function pageSchedulerEvents(
    direction: "prev" | "next",
  ): Promise<void> {
    const limit =
      parsePositiveInteger(schedulerEventFilters.limit, 12, 200) ?? 12;
    const currentOffset = parseNonNegativeInteger(schedulerEventFilters.offset);
    const nextOffset =
      direction === "prev"
        ? Math.max(0, currentOffset - limit)
        : currentOffset + limit;
    const next = { ...schedulerEventFilters, offset: String(nextOffset) };
    setSchedulerEventFilters(next);
    await refreshSchedulerEvents(false, next);
  }

  async function refreshGameScoreboard(silent = false): Promise<void> {
    if (!silent) {
      setPendingAction("game:scoreboard");
      setActionError(null);
      setActionNote(null);
    }

    try {
      const response = await fetch("/api/admin/game/scoreboard");
      const payload = (await response.json()) as ActionEnvelope<
        AdminGameScoreRow[]
      >;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload ? payload.message : "scoreboard fetch failed",
        );
      }

      setScoreRows(payload.data);
      if (!silent) {
        setActionNote(
          `Loaded ${payload.data.length} scoreboard row(s) from game-core.`,
        );
      }
    } catch (error) {
      if (!silent) {
        setActionError(
          error instanceof Error ? error.message : "scoreboard fetch failed",
        );
      }
    } finally {
      if (!silent) {
        setPendingAction(null);
      }
    }
  }

  async function recomputeGameScoring(silent = false): Promise<void> {
    if (!silent) {
      setPendingAction("game:scoring");
      setActionError(null);
      setActionNote(null);
    }

    try {
      const response = await fetch("/api/admin/game/scoring/recompute", {
        method: "POST",
      });
      const payload = (await response.json()) as ActionEnvelope<
        AdminGameScoreRow[]
      >;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload ? payload.message : "score recompute failed",
        );
      }

      setScoreRows(payload.data);
      if (!silent) {
        setActionNote(
          `Recomputed ${payload.data.length} scoreboard row(s) from authoritative tick and submission state.`,
        );
      }
    } catch (error) {
      if (!silent) {
        setActionError(
          error instanceof Error ? error.message : "score recompute failed",
        );
      }
    } finally {
      if (!silent) {
        setPendingAction(null);
      }
    }
  }

  async function advanceGameTick(): Promise<void> {
    setPendingAction("game:advance");
    setActionError(null);
    setActionNote(null);

    try {
      const response = await fetch("/api/admin/game/ticks/advance", {
        method: "POST",
      });
      const payload =
        (await response.json()) as ActionEnvelope<AdminGameTickStatus>;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload
            ? payload.message
            : "game-core tick advance failed",
        );
      }

      setGameState((current) => ({
        current_tick: payload.data,
        total_ticks: Math.max(current.total_ticks + 1, payload.data.id),
        total_checker_runs:
          current.total_checker_runs + payload.data.total_checker_runs,
        successful_checker_runs:
          current.successful_checker_runs +
          payload.data.successful_checker_runs,
        failed_checker_runs:
          current.failed_checker_runs + payload.data.failed_checker_runs,
        skipped_checker_runs:
          current.skipped_checker_runs + payload.data.skipped_checker_runs,
      }));
      await refreshSchedulerEvents(true);
      await refreshCheckerRuns(true);
      await recomputeGameScoring(true);
      setActionNote(
        `Tick ${payload.data.id} completed with ${payload.data.successful_checker_runs} success, ${payload.data.failed_checker_runs} failed, and ${payload.data.skipped_checker_runs} skipped checker runs.`,
      );
    } catch (error) {
      setActionError(
        error instanceof Error
          ? error.message
          : "game-core tick advance failed",
      );
    } finally {
      setPendingAction(null);
    }
  }

  function closeWireGuardDialog(): void {
    setWireGuardDialogOpen(false);
  }

  function selectDeleteTarget(target: DeleteTarget): void {
    setDeleteTarget(target);
  }

  function closeDeleteDialog(): void {
    setDeleteTarget(null);
  }

  async function confirmDelete(): Promise<void> {
    if (!deleteTarget) {
      return;
    }

    const target = deleteTarget;
    setDeleteTarget(null);

    switch (target.kind) {
      case "team": {
        const team = teamRows.find((t) => t.id === target.id);
        if (team) {
          await deleteTeam(team);
        }
        break;
      }
      case "player": {
        const player = playerRows.find((p) => p.id === target.id);
        if (player) {
          await deletePlayer(player);
        }
        break;
      }
      case "challenge": {
        const challenge = challengeRows.find((c) => c.id === target.id);
        if (challenge) {
          await deleteChallenge(challenge);
        }
        break;
      }
      case "deployment": {
        const deployment = deploymentRows.find((d) => d.id === target.id);
        if (deployment) {
          await deleteDeployment(deployment);
        }
        break;
      }
    }
  }

  function openCreateDialog(entity: "team" | "player" | "challenge"): void {
    setFormMode("create");
    setFormEntity(entity);
    setEditingId(null);
    setActionError(null);
    setActionNote(null);
    if (entity === "team") {
      setTeamDraft({ name: "", contactEmail: "" });
    } else if (entity === "player") {
      const defaultTeamID =
        teamRows.find((team) => team.id === playerDraft.teamId)?.id ??
        teamRows[0]?.id ??
        0;
      setPlayerDraft((current) => ({
        ...current,
        teamId: defaultTeamID,
        displayName: "",
        email: "",
        password: "",
        role: "member",
      }));
    } else {
      setChallengeDraft({
        name: "",
        baselineImage: "",
        checkerImage: "",
        servicePort: "",
        serviceSubnetOctet: "",
        weight: "1",
      });
    }
  }

  function openEditDialog(
    entity: "team" | "player" | "challenge",
    id: number,
  ): void {
    setFormMode("edit");
    setFormEntity(entity);
    setEditingId(id);
    setActionError(null);
    setActionNote(null);
    if (entity === "team") {
      const team = teamRows.find((t) => t.id === id);
      if (team) {
        setTeamDraft({ name: team.name, contactEmail: team.contact_email });
      }
    } else if (entity === "player") {
      const player = playerRows.find((p) => p.id === id);
      if (player) {
        setPlayerDraft({
          teamId: player.team_id,
          displayName: player.display_name,
          email: player.email,
          password: "",
          role: player.role,
        });
      }
    } else {
      const challenge = challengeRows.find((c) => c.id === id);
      if (challenge) {
        setChallengeDraft({
          name: challenge.name,
          baselineImage: challenge.baseline_image,
          checkerImage: challenge.checker_image,
          servicePort: String(challenge.service_port),
          serviceSubnetOctet: String(challenge.service_subnet_octet),
          weight: String(challenge.weight),
        });
      }
    }
  }

  function closeFormDialog(): void {
    setFormMode(null);
    setFormEntity(null);
    setEditingId(null);
  }

  async function updateTeam(): Promise<void> {
    if (editingId === null) return;
    setPendingAction("team:update");
    setActionError(null);
    setActionNote(null);
    try {
      const response = await fetch(`/api/admin/teams/${editingId}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: teamDraft.name,
          contact_email: teamDraft.contactEmail,
        }),
      });
      const payload = (await response.json()) as ActionEnvelope<AdminTeam>;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload ? payload.message : "team update failed",
        );
      }
      setTeamRows((current) =>
        current.map((t) => (t.id === editingId ? payload.data : t)),
      );
      closeFormDialog();
      setActionNote(`Updated team ${payload.data.name}.`);
    } catch (error) {
      setActionError(
        error instanceof Error ? error.message : "team update failed",
      );
    } finally {
      setPendingAction(null);
    }
  }

  async function updatePlayer(): Promise<void> {
    if (editingId === null) return;
    setPendingAction("player:update");
    setActionError(null);
    setActionNote(null);
    try {
      const response = await fetch(`/api/admin/players/${editingId}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          display_name: playerDraft.displayName,
          email: playerDraft.email,
          role: playerDraft.role,
        }),
      });
      const payload = (await response.json()) as ActionEnvelope<AdminPlayer>;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload ? payload.message : "player update failed",
        );
      }
      setPlayerRows((current) =>
        current.map((p) => (p.id === editingId ? payload.data : p)),
      );
      closeFormDialog();
      setActionNote(`Updated player ${payload.data.display_name}.`);
    } catch (error) {
      setActionError(
        error instanceof Error ? error.message : "player update failed",
      );
    } finally {
      setPendingAction(null);
    }
  }

  async function updateChallenge(): Promise<void> {
    if (editingId === null) return;
    setPendingAction("challenge:update");
    setActionError(null);
    setActionNote(null);
    try {
      const response = await fetch(`/api/admin/challenges/${editingId}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: challengeDraft.name,
          baseline_image: challengeDraft.baselineImage,
          checker_image: challengeDraft.checkerImage,
          weight: Number(challengeDraft.weight) || 1,
        }),
      });
      const payload = (await response.json()) as ActionEnvelope<AdminChallenge>;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload ? payload.message : "challenge update failed",
        );
      }
      setChallengeRows((current) =>
        current.map((c) => (c.id === editingId ? payload.data : c)),
      );
      closeFormDialog();
      setActionNote(`Updated challenge ${payload.data.name}.`);
    } catch (error) {
      setActionError(
        error instanceof Error ? error.message : "challenge update failed",
      );
    } finally {
      setPendingAction(null);
    }
  }

  async function submitForm(): Promise<void> {
    if (formMode === "create") {
      if (formEntity === "team") await createTeam();
      else if (formEntity === "player") await createPlayer();
      else if (formEntity === "challenge") await createChallenge();
      closeFormDialog();
    } else if (formMode === "edit") {
      if (formEntity === "team") await updateTeam();
      else if (formEntity === "player") await updatePlayer();
      else if (formEntity === "challenge") await updateChallenge();
    }
  }

  return {
    accessStatus,
    actionError,
    actionNote,
    applyAttackFilters,
    attackFilters,
    highlightedAttackIDs,
    attackPageState,
    attacksLiveMode,
    applyCheckerRunFilters,
    applySchedulerEventFilters,
    challengeDraft,
    challengeRows,
    challengeValidationRows,
    checkerRunFilters,
    checkerRunPageState,
    checkerRunsLiveMode,
    closeDeleteDialog,
    closeFormDialog,
    closeWireGuardDialog,
    confirmDelete,
    createChallenge,
    createPlayer,
    createTeam,
    deleteTarget,
    editingId,
    formEntity,
    formMode,
    openCreateDialog,
    openEditDialog,
    selectDeleteTarget,
    submitForm,
    deploymentRows,
    deployChallenge,
    downloadWireGuardConfig,
    gameState,
    inspectWireGuard,
    pageAttacks,
    pageCheckerRuns,
    pageSchedulerEvents,
    pendingAction,
    playerDraft,
    playerRows,
    recomputeGameScoring,
    reconcileAccess,
    reconcileDeployments,
    reconcileWireGuardGateway,
    refreshAccessStatus,
    refreshAttacks,
    refreshCheckerRuns,
    refreshGameScoreboard,
    refreshGameStatus,
    refreshOperationsStatus,
    refreshServiceMetrics,
    refreshSchedulerEvents,
    refreshWireGuardGatewayStatus,
    teardownAccess,
    teardownWireGuardGateway,
    resetAttackFilters,
    resetCheckerRunFilters,
    resetSchedulerEventFilters,
    revokeWireGuard,
    rotateWireGuard,
    schedulerEventFilters,
    schedulerEventPageState,
    schedulerEventsLiveMode,
    scoreRows,
    selectedWireGuardPeer,
    setAttackFilters,
    setChallengeDraft,
    setCheckerRunFilters,
    setPlayerDraft,
    setSchedulerEventFilters,
    setTeamDraft,
    startGameMatch,
    startGameScheduler,
    stopGameMatch,
    stopGameScheduler,
    updateGameMatchSchedule,
    updateGameScheduler,
    summary,
    teamDraft,
    teamRows,
    validateChallenge,
    wireGuardDialogOpen,
    wireGuardGatewayStatus,
    operationsStatus: operationsStatusState,
    serviceMetrics: serviceMetricsState,
    advanceGameTick,
  };
}

function buildCheckerRunQuery(
  filters: CheckerRunFilters,
): AdminCheckerRunQuery {
  return {
    limit: parsePositiveInteger(filters.limit, 18, 200),
    offset: parseNonNegativeInteger(filters.offset),
    tick_id: parsePositiveInteger(filters.tickId),
    team_id: parsePositiveInteger(filters.teamId),
    challenge_id: parsePositiveInteger(filters.challengeId),
    phase: filters.phase.trim().toLowerCase() || undefined,
    status: filters.status.trim().toLowerCase() || undefined,
  };
}

function buildCheckerRunQueryString(filters: CheckerRunFilters): string {
  return buildQueryString(buildCheckerRunQuery(filters));
}

function buildAttackQuery(filters: AttackFilters): AdminAttackFeedQuery {
  return {
    limit: parsePositiveInteger(filters.limit, 12, 200),
    offset: parseNonNegativeInteger(filters.offset),
    attacker: filters.attacker.trim() || undefined,
    victim: filters.victim.trim() || undefined,
    service: filters.service.trim() || undefined,
    tick_from: parseNonNegativeInteger(filters.tickFrom) || undefined,
    tick_to: parseNonNegativeInteger(filters.tickTo) || undefined,
  };
}

function collectNewAdminAttackIDs(
  currentPage: AdminAttackFeedPage,
  nextPage: AdminAttackFeedPage,
): string[] {
  const currentIDs = new Set(currentPage.items.map((item) => item.id));
  return nextPage.items
    .filter((item) => !currentIDs.has(item.id))
    .map((item) => item.id)
    .slice(0, 4);
}

function mergeRealtimeAdminAttackPage(
  currentPage: AdminAttackFeedPage,
  streamedPage: AdminAttackFeedPage,
  requestedLimit: string,
): AdminAttackFeedPage {
  const limit =
    parsePositiveInteger(requestedLimit, streamedPage.limit) ??
    streamedPage.limit;
  if (limit <= streamedPage.limit) {
    return streamedPage;
  }

  const mergedItems = [...streamedPage.items];
  const seenIDs = new Set(mergedItems.map((item) => item.id));
  for (const item of currentPage.items) {
    if (seenIDs.has(item.id)) {
      continue;
    }
    mergedItems.push(item);
    seenIDs.add(item.id);
    if (mergedItems.length >= limit) {
      break;
    }
  }

  return {
    ...streamedPage,
    items: mergedItems,
    limit,
    has_next: streamedPage.total_count > mergedItems.length,
  };
}

function buildAttackQueryString(filters: AttackFilters): string {
  return buildQueryString(buildAttackQuery(filters));
}

function buildSchedulerEventQuery(
  filters: SchedulerEventFilters,
): AdminSchedulerEventQuery {
  return {
    limit: parsePositiveInteger(filters.limit, 12, 200),
    offset: parseNonNegativeInteger(filters.offset),
    event_type: filters.eventType.trim().toLowerCase() || undefined,
    source: filters.source.trim().toLowerCase() || undefined,
    state: filters.state.trim().toLowerCase() || undefined,
  };
}

function buildSchedulerEventQueryString(
  filters: SchedulerEventFilters,
): string {
  return buildQueryString(buildSchedulerEventQuery(filters));
}

function buildQueryString(
  query: Record<string, string | number | undefined>,
): string {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value === undefined || value === "") {
      continue;
    }
    params.set(key, String(value));
  }
  const encoded = params.toString();
  return encoded ? `?${encoded}` : "";
}

function isDefaultAttackFilters(
  filters: AttackFilters,
  defaults: AttackFilters,
): boolean {
  return (
    filters.limit === defaults.limit &&
    filters.offset === defaults.offset &&
    filters.attacker === defaults.attacker &&
    filters.victim === defaults.victim &&
    filters.service === defaults.service &&
    filters.tickFrom === defaults.tickFrom &&
    filters.tickTo === defaults.tickTo
  );
}

function parsePositiveInteger(
  raw: string,
  fallback?: number,
  max = 0,
): number | undefined {
  const parsed = Number.parseInt(raw, 10);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return fallback;
  }
  if (max > 0 && parsed > max) {
    return max;
  }
  return parsed;
}

function parseNonNegativeInteger(raw: string): number {
  const parsed = Number.parseInt(raw, 10);
  if (!Number.isFinite(parsed) || parsed < 0) {
    return 0;
  }
  return parsed;
}

function isDefaultCheckerRunFilters(filters: CheckerRunFilters): boolean {
  return (
    filters.limit === defaultCheckerRunFilters.limit &&
    filters.offset === defaultCheckerRunFilters.offset &&
    filters.tickId.trim() === "" &&
    filters.teamId.trim() === "" &&
    filters.challengeId.trim() === "" &&
    filters.phase.trim() === "" &&
    filters.status.trim() === ""
  );
}

function isDefaultSchedulerEventFilters(
  filters: SchedulerEventFilters,
): boolean {
  return (
    filters.limit === defaultSchedulerEventFilters.limit &&
    filters.offset === defaultSchedulerEventFilters.offset &&
    filters.eventType.trim() === "" &&
    filters.source.trim() === "" &&
    filters.state.trim() === ""
  );
}
