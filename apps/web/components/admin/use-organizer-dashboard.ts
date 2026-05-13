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
  AdminRuntimeEvidenceReport,
  AdminServiceMetricSnapshot,
  AdminPlayer,
  AdminReconcileResult,
  AdminSchedulerEventPage,
  AdminSchedulerEventQuery,
  AdminTeam,
  AdminWireGuardGatewayStatus,
  AdminWireGuardPeer,
} from "@/lib/admin-dashboard-types";
import { buildQueryString, parseApiError, processApiResponse } from "@/lib/api-utils";

import { useAttackHighlights } from "@/components/hooks/use-attack-highlights";

import {
  computePageOffset,
  handleRealtimeAttackMessage,
  parsePositiveInteger,
  parseNonNegativeInteger,
  isDefaultAttackFilters,
  type AttackFilters,
} from "@/lib/dashboard-utils";

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

async function writeTextToClipboard(text: string): Promise<void> {
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return;
    } catch {
      // Fall back to document copy for browsers or harnesses without clipboard permission.
    }
  }

  const textarea = document.createElement("textarea");
  textarea.value = text;
  textarea.setAttribute("readonly", "");
  textarea.style.position = "fixed";
  textarea.style.opacity = "0";
  textarea.style.pointerEvents = "none";
  document.body.appendChild(textarea);
  textarea.focus();
  textarea.select();

  const copied = document.execCommand("copy");
  document.body.removeChild(textarea);

  if (!copied) {
    throw new Error("clipboard write failed");
  }
}

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
  sourceBundlePath: string;
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
  copyRuntimeHealthSummary: () => Promise<void>;
  deleteTarget: DeleteTarget | null;
  downloadRuntimeHealthReport: () => Promise<void>;
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
  refreshDeploymentRows: (silent?: boolean) => Promise<void>;
  refreshGameScoreboard: (silent?: boolean) => Promise<void>;
  refreshGameStatus: (silent?: boolean) => Promise<void>;
  refreshRuntimeHealth: (silent?: boolean) => Promise<void>;
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
  const { highlightedAttackIDs, scheduleAttackHighlights, clearAttackHighlights } = useAttackHighlights();
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
    sourceBundlePath: "",
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
  const checkerRunsLiveRef = useRef(true);
  const schedulerEventsLiveRef = useRef(true);
  const pendingActionRef = useRef<string | null>(null);
  const gameStatusRealtimeSuppressedUntilRef = useRef(0);

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
    checkerRunsLiveRef.current = checkerRunsLiveMode;
  }, [checkerRunsLiveMode]);

  useEffect(() => {
    schedulerEventsLiveRef.current = schedulerEventsLiveMode;
  }, [schedulerEventsLiveMode]);

  useEffect(() => {
    pendingActionRef.current = pendingAction;
  }, [pendingAction]);

  function suppressGameStatusRealtime(durationMs = 1_500): void {
    gameStatusRealtimeSuppressedUntilRef.current =
      window.performance.now() + durationMs;
  }

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
      const action = pendingActionRef.current;
      if (
        (action !== null && action.startsWith("game:")) ||
        window.performance.now() < gameStatusRealtimeSuppressedUntilRef.current
      ) {
        return;
      }

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

    attacksSource.onmessage = handleRealtimeAttackMessage({
      isLive: () => attacksLiveRef.current,
      currentPage: () => attackPageRef.current,
      limitStr: attackFilters.limit,
      setPage: setAttackPageState,
      scheduleHighlights: scheduleAttackHighlights,
    });

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
  }, []);

  async function persistEntity<T>(
    url: string,
    method: "POST" | "PUT",
    body: object,
    typeLabel: string,
  ): Promise<T> {
    const response = await fetch(url, {
      method,
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    return processApiResponse<T>(
      response,
      `${typeLabel} ${method === "POST" ? "create" : "update"} request`,
    );
  }

  async function createTeam(): Promise<boolean> {
    setPendingAction("team:create");
    setActionError(null);
    setActionNote(null);

    try {
      const data = await persistEntity<AdminTeam>(
        "/api/admin/teams",
        "POST",
        { name: teamDraft.name, contact_email: teamDraft.contactEmail },
        "team",
      );

      setTeamRows((current) => [...current, data]);
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
        teamId: current.teamId || data.id,
      }));
      setActionNote(
        `Created ${data.name} and seeded deployed services for published challenges.`,
      );
      return true;
    } catch (error) {
      setActionError(
        error instanceof Error ? error.message : "team create failed",
      );
      return false;
    } finally {
      setPendingAction(null);
    }
  }

  async function createPlayer(): Promise<boolean> {
    if (!playerDraft.teamId) {
      setActionError("select a team before creating a player");
      return false;
    }

    setPendingAction("player:create");
    setActionError(null);
    setActionNote(null);

    try {
      const data = await persistEntity<AdminPlayer>(
        "/api/admin/players",
        "POST",
        {
          team_id: playerDraft.teamId,
          display_name: playerDraft.displayName,
          email: playerDraft.email,
          password: playerDraft.password,
          role: playerDraft.role,
        },
        "player",
      );

      setPlayerRows((current) => [...current, data]);
      setTeamRows((current) =>
        current.map((team) =>
          team.id === data.team_id
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
        `Created player ${data.display_name} with peer ${data.wireguard_peer}. Reconcile the WireGuard gateway and service access policy if that team already has unlocked services.`,
      );
      return true;
    } catch (error) {
      setActionError(
        error instanceof Error ? error.message : "player create failed",
      );
      return false;
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
    return refreshPagedFeed<AdminWireGuardGatewayStatus>({
      silent,
      actionKey: "wireguard-gateway:status",
      url: "/api/admin/wireguard/status",
      errorLabel: "wireguard gateway status failed",
      setPage: setWireGuardGatewayStatus,
      formatNote: (data) =>
        `WireGuard gateway is ${data.state} in ${data.mode} mode.`,
    });
  }

  async function refreshAccessStatus(silent = false): Promise<void> {
    return refreshPagedFeed<AdminControllerAccessStatus>({
      silent,
      actionKey: "access:status",
      url: "/api/admin/access/status",
      errorLabel: "controller access status failed",
      setPage: setAccessStatus,
      formatNote: (data) =>
        `Controller access policy is ${data.state} in ${data.mode} mode.`,
    });
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
      const payload = await processApiResponse<AdminOperationsStatus>(
        response,
        "/api/admin/operations/status",
      );
      setOperationsStatusState(payload);
      if (!silent) {
        setActionNote(
          payload.alerts.length === 0
            ? "Runtime operations look healthy."
            : `Loaded ${payload.alerts.length} runtime alert(s).`,
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
      const payload = await processApiResponse<AdminServiceMetricSnapshot>(
        response,
        "/api/admin/operations/metrics",
      );
      setServiceMetricsState(payload);
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

  async function refreshRuntimeHealth(silent = false): Promise<void> {
    if (!silent) {
      setPendingAction("runtime:health");
      setActionError(null);
      setActionNote(null);
    }

    try {
      await Promise.all([
        refreshDeploymentRows(true),
        refreshWireGuardGatewayStatus(true),
        refreshAccessStatus(true),
        refreshOperationsStatus(true),
        refreshServiceMetrics(true),
      ]);
      if (!silent) {
        setActionNote(
          "Refreshed deployment, access policy, WireGuard, runtime alerts, and service metrics.",
        );
      }
    } catch (error) {
      if (!silent) {
        setActionError(
          error instanceof Error ? error.message : "runtime health refresh failed",
        );
      }
    } finally {
      if (!silent) {
        setPendingAction(null);
      }
    }
  }

  async function downloadRuntimeHealthReport(): Promise<void> {
    setPendingAction("runtime:report");
    setActionError(null);
    setActionNote(null);

    try {
      const response = await fetch("/api/admin/runtime-health/report", {
        cache: "no-store",
      });
      if (!response.ok) {
        throw new Error(
          await parseApiError(response, "/api/admin/runtime-health/report"),
        );
      }

      const report = (await response
        .clone()
        .json()) as AdminRuntimeEvidenceReport;
      const blob = await response.blob();
      const contentDisposition =
        response.headers.get("content-disposition") ?? "";
      const downloadName =
        contentDisposition.match(/filename="?([^"]+)"?/)?.[1] ??
        "runtime-health-report.json";

      const objectUrl = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = objectUrl;
      anchor.download = downloadName;
      anchor.click();
      URL.revokeObjectURL(objectUrl);

      setActionNote(
        report.failures.length === 0
          ? `Downloaded complete runtime evidence report as ${downloadName}.`
          : `Downloaded partial runtime evidence report as ${downloadName}; missing ${report.failures
              .map((failure) => failure.section)
              .join(", ")}.`,
      );
    } catch (error) {
      setActionError(
        error instanceof Error
          ? error.message
          : "runtime health report download failed",
      );
    } finally {
      setPendingAction(null);
    }
  }

  async function copyRuntimeHealthSummary(): Promise<void> {
    setPendingAction("runtime:summary");
    setActionError(null);
    setActionNote(null);

    try {
      const response = await fetch("/api/admin/runtime-health/report", {
        cache: "no-store",
      });
      const payload = await processApiResponse<AdminRuntimeEvidenceReport>(
        response,
        "/api/admin/runtime-health/report",
      );
      await writeTextToClipboard(payload.summary);
      setActionNote("Copied runtime summary to clipboard.");
    } catch (error) {
      setActionError(
        error instanceof Error ? error.message : "runtime summary copy failed",
      );
    } finally {
      setPendingAction(null);
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
      const payload = await processApiResponse<AdminWireGuardPeer>(
        response,
        `/api/admin/players/${player.id}/wireguard`,
      );

      applyWireGuardUpdate(payload);
      setSelectedWireGuardPeer(payload);
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

  async function wireGuardAction(
    player: AdminPlayer,
    action: "rotate" | "revoke",
    successMessage: string,
    openDialog: boolean,
  ): Promise<void> {
    setPendingAction(`wireguard:${action}:${player.id}`);
    setActionError(null);
    setActionNote(null);

    try {
      const response = await fetch(
        `/api/admin/players/${player.id}/wireguard/${action}`,
        { method: "POST" },
      );
      const payload = await processApiResponse<AdminWireGuardPeer>(
        response,
        `/api/admin/players/${player.id}/wireguard/${action}`,
      );

      applyWireGuardUpdate(payload);
      setSelectedWireGuardPeer(payload);
      if (openDialog) setWireGuardDialogOpen(true);
      setActionNote(successMessage);
    } catch (error) {
      setActionError(
        error instanceof Error ? error.message : `wireguard ${action} failed`,
      );
    } finally {
      setPendingAction(null);
    }
  }

  async function rotateWireGuard(player: AdminPlayer): Promise<void> {
    return wireGuardAction(
      player,
      "rotate",
      `Rotated WireGuard config for ${player.display_name}. Reconcile the WireGuard gateway so the old peer material stops working.`,
      true,
    );
  }

  async function revokeWireGuard(player: AdminPlayer): Promise<void> {
    return wireGuardAction(
      player,
      "revoke",
      `Revoked WireGuard config for ${player.display_name}. Reconcile the WireGuard gateway and service access policy to remove the peer from runtime access.`,
      false,
    );
  }

  async function reconcileWireGuardGateway(): Promise<void> {
    setPendingAction("wireguard-gateway:reconcile");
    setActionError(null);
    setActionNote(null);

    try {
      const response = await fetch("/api/admin/wireguard/reconcile", {
        method: "POST",
      });
      const payload = await processApiResponse<AdminWireGuardGatewayStatus>(
        response,
        "/api/admin/wireguard/reconcile",
      );

      setWireGuardGatewayStatus(payload);
      setActionNote(
        `Maintenance reconcile applied WireGuard revision ${payload.revision ?? "n/a"} with ${payload.peers_active} active peer(s) and ${payload.peers_revoked} revoked peer(s).`,
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
      const payload = await processApiResponse<AdminControllerAccessStatus>(
        response,
        "/api/admin/access/reconcile",
      );
      setAccessStatus(payload);
      setActionNote(
        `Maintenance reconcile applied controller access revision ${payload.revision ?? "n/a"} across ${payload.policies_total} service policy row(s).`,
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
      await processApiResponse<void>(response, "/api/admin/wireguard/teardown");

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
      await processApiResponse<void>(response, "/api/admin/access/teardown");

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

  async function deleteEntity(opts: {
    actionKey: string;
    url: string;
    errorLabel: string;
    onSuccess: () => void;
    successNote: string;
  }): Promise<void> {
    setPendingAction(opts.actionKey);
    setActionError(null);
    setActionNote(null);

    try {
      const response = await fetch(opts.url, { method: "DELETE" });
      await processApiResponse<void>(response, opts.url);

      opts.onSuccess();
      setActionNote(opts.successNote);
    } catch (error) {
      setActionError(
        error instanceof Error ? error.message : opts.errorLabel,
      );
    } finally {
      setPendingAction(null);
    }
  }

  async function deleteTeam(team: AdminTeam): Promise<void> {
    return deleteEntity({
      actionKey: `team:delete:${team.id}`,
      url: `/api/admin/teams/${team.id}`,
      errorLabel: "team delete failed",
      onSuccess: () => {
        setTeamRows((current) => current.filter((item) => item.id !== team.id));
        setPlayerRows((current) =>
          current.filter((item) => item.team_id !== team.id),
        );
      },
      successNote: `Team "${team.name}" deleted.`,
    });
  }

  async function deletePlayer(player: AdminPlayer): Promise<void> {
    return deleteEntity({
      actionKey: `player:delete:${player.id}`,
      url: `/api/admin/players/${player.id}`,
      errorLabel: "player delete failed",
      onSuccess: () => {
        setPlayerRows((current) =>
          current.filter((item) => item.id !== player.id),
        );
      },
      successNote: `Player "${player.display_name}" deleted.`,
    });
  }

  async function deleteChallenge(challenge: AdminChallenge): Promise<void> {
    return deleteEntity({
      actionKey: `challenge:delete:${challenge.id}`,
      url: `/api/admin/challenges/${challenge.id}`,
      errorLabel: "challenge delete failed",
      onSuccess: () => {
        setChallengeRows((current) =>
          current.filter((item) => item.id !== challenge.id),
        );
      },
      successNote: `Challenge "${challenge.name}" deleted.`,
    });
  }

  async function deleteDeployment(deployment: AdminDeploymentJob): Promise<void> {
    return deleteEntity({
      actionKey: `deployment:delete:${deployment.id}`,
      url: `/api/admin/deployments/${deployment.id}`,
      errorLabel: "deployment delete failed",
      onSuccess: () => {
        setDeploymentRows((current) =>
          current.filter((item) => item.id !== deployment.id),
        );
      },
      successNote: `Deployment job #${deployment.id} deleted.`,
    });
  }

  async function createChallenge(): Promise<boolean> {
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
          source_bundle_path: challengeDraft.sourceBundlePath,
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
      const payload = await processApiResponse<AdminChallenge>(
        response,
        "/api/admin/challenges",
      );

      setChallengeRows((current) => [...current, payload]);
      setChallengeValidationRows((current) => {
        const next = { ...current };
        delete next[payload.id];
        return next;
      });
      setChallengeDraft({
        name: "",
        baselineImage: "",
        checkerImage: "",
        sourceBundlePath: "",
        servicePort: "",
        serviceSubnetOctet: "",
        weight: "1",
      });
      setActionNote(
        `Created draft challenge ${payload.name}. Deploy it to replicate one service per team.`,
      );
      return true;
    } catch (error) {
      setActionError(
        error instanceof Error ? error.message : "challenge create failed",
      );
      return false;
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
      const payload = await processApiResponse<AdminChallengeValidationResult>(
        response,
        `/api/admin/challenges/${challenge.id}/validate`,
      );

      applyChallengeValidation(payload);
      if (!silent) {
        setActionNote(
          payload.status === "valid"
            ? `${challenge.name} service and checker images satisfy the current runtime package policy.`
            : payload.message ||
                `${challenge.name} failed runtime validation.`,
        );
      }
      return payload;
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
        !validation.checker_contract_ok ||
        !validation.service_state_contract_ok
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
      const payload = await processApiResponse<AdminDeployment>(
        response,
        `/api/admin/challenges/${challenge.id}/deploy`,
      );

      setChallengeRows((current) =>
        current.map((item) =>
          item.id === challenge.id
            ? {
                ...item,
                published: true,
                deployed_teams: payload.total_team_count,
                total_teams: payload.total_team_count,
                runtime_status:
                  payload.queued_team_count > 0 ? "deploying" : "ready",
                queued_teams: payload.queued_team_count,
                ready_teams: payload.ready_team_count,
              }
            : item,
        ),
      );

      if (payload.job_id > 0) {
        setDeploymentRows((current) => [
          {
            id: payload.job_id,
            challenge_id: payload.challenge_id,
            challenge_name: payload.challenge_name,
            status: payload.status,
            target_team_count: payload.total_team_count,
            queued_team_count: payload.queued_team_count,
            ready_team_count: payload.ready_team_count,
            failed_team_count: 0,
            created_at: payload.created_at,
            completed_at: payload.completed_at,
          },
          ...current.map((deployment) =>
            deployment.challenge_id === payload.challenge_id &&
            deployment.id !== payload.job_id &&
            (deployment.status === "queued" || deployment.status === "running")
              ? {
                  ...deployment,
                  status: "superseded",
                  queued_team_count: 0,
                  completed_at:
                    payload.created_at ?? new Date().toISOString(),
                }
              : deployment,
          ),
        ]);
      } else {
        setDeploymentRows((current) =>
          current.map((deployment) =>
            deployment.challenge_id === payload.challenge_id &&
            (deployment.status === "queued" || deployment.status === "running")
              ? {
                  ...deployment,
                  status: "superseded",
                  queued_team_count: 0,
                  completed_at:
                    payload.created_at ?? new Date().toISOString(),
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
        payload.status === "queued"
          ? `Queued ${payload.challenge_name} for ${payload.deployed_team_count} team runtimes. Run trusted reconcile to verify rollout, SSH access, and WireGuard state.`
          : `${payload.challenge_name} was already fully deployed across all teams.`,
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
      const payload = await processApiResponse<AdminReconcileResult>(
        reconcileResponse,
        "/api/admin/deployments/reconcile",
      );

      await refreshRuntimeHealth(true);

      setActionNote(
        `Trusted reconcile processed ${payload.processed_jobs} job(s), advanced ${payload.processed_instances} team service instance(s), and refreshed deployment, SSH access, and WireGuard truth.`,
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
      const payload = await processApiResponse<AdminGameStatus>(
        response,
        "/api/admin/game/status",
      );

      setGameState(payload);
      if (!silent) {
        setActionNote(
          `Game-core reports match ${payload.match?.state ?? "unknown"} with ${payload.total_ticks} persisted tick(s).`,
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

  async function refreshDeploymentRows(silent = false): Promise<void> {
    return refreshPagedFeed<AdminDeploymentJob[]>({
      silent,
      actionKey: "deployments:list",
      url: "/api/admin/deployments",
      errorLabel: "deployment list fetch failed",
      setPage: setDeploymentRows,
      formatNote: (rows) => `Loaded ${rows.length} deployment job(s).`,
    });
  }

  async function startGameMatch(): Promise<void> {
    suppressGameStatusRealtime();
    setPendingAction("game:match:start");
    setActionError(null);
    setActionNote(null);

    let matchStarted = false;

    try {
      const matchResponse = await fetch("/api/admin/game/match/start", {
        method: "POST",
      });
      const matchPayload = await processApiResponse<AdminGameStatus["match"]>(
        matchResponse,
        "/api/admin/game/match/start",
      );

      matchStarted = true;
      setGameState((current) => ({ ...current, match: matchPayload }));

      const schedulerResponse = await fetch("/api/admin/game/scheduler/start", {
        method: "POST",
      });
      const schedulerPayload = await processApiResponse<
        AdminGameStatus["scheduler"]
      >(schedulerResponse, "/api/admin/game/scheduler/start");

      setGameState((current) => ({
        ...current,
        match: matchPayload,
        scheduler: schedulerPayload,
      }));
      await refreshSchedulerEvents(true);
      setActionNote(
        `Game started. Submissions are ${matchPayload?.accepting_submissions ? "open" : "closed"} and the scheduler is running at ${schedulerPayload?.interval_seconds ?? 0}s.`,
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
      suppressGameStatusRealtime(1_000);
      setPendingAction(null);
    }
  }

  async function stopGameMatch(): Promise<void> {
    suppressGameStatusRealtime();
    setPendingAction("game:match:stop");
    setActionError(null);
    setActionNote(null);

    try {
      const response = await fetch("/api/admin/game/match/stop", {
        method: "POST",
      });
      const payload = await processApiResponse<AdminGameStatus["match"]>(
        response,
        "/api/admin/game/match/stop",
      );

      setGameState((current) => ({
        ...current,
        match: payload,
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
      suppressGameStatusRealtime(1_000);
      setPendingAction(null);
    }
  }

  async function startGameScheduler(): Promise<void> {
    suppressGameStatusRealtime();
    setPendingAction("game:scheduler:start");
    setActionError(null);
    setActionNote(null);

    try {
      const response = await fetch("/api/admin/game/scheduler/start", {
        method: "POST",
      });
      const payload = await processApiResponse<AdminGameStatus["scheduler"]>(
        response,
        "/api/admin/game/scheduler/start",
      );

      setGameState((current) => ({ ...current, scheduler: payload }));
      await refreshSchedulerEvents(true);
      setActionNote(
        `Scheduler started with ${payload?.interval_seconds ?? 0}s interval.`,
      );
    } catch (error) {
      setActionError(
        error instanceof Error
          ? error.message
          : "game-core scheduler start failed",
      );
    } finally {
      suppressGameStatusRealtime(1_000);
      setPendingAction(null);
    }
  }

  async function stopGameScheduler(): Promise<void> {
    suppressGameStatusRealtime();
    setPendingAction("game:scheduler:stop");
    setActionError(null);
    setActionNote(null);

    try {
      const response = await fetch("/api/admin/game/scheduler/stop", {
        method: "POST",
      });
      const payload = await processApiResponse<AdminGameStatus["scheduler"]>(
        response,
        "/api/admin/game/scheduler/stop",
      );

      setGameState((current) => ({ ...current, scheduler: payload }));
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
      suppressGameStatusRealtime(1_000);
      setPendingAction(null);
    }
  }

  async function updateGameMatchSchedule(schedule: {
    scheduledStartAt?: string;
    scheduledEndAt?: string;
  }): Promise<void> {
    suppressGameStatusRealtime();
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
      const payload = await processApiResponse<AdminGameStatus["match"]>(
        response,
        "/api/admin/game/match/schedule",
      );

      setGameState((current) => ({ ...current, match: payload }));
      setActionNote(
        `Match window updated. Start: ${payload?.scheduled_start_at ?? "manual"}; end: ${payload?.scheduled_end_at ?? "manual"}.`,
      );
    } catch (error) {
      setActionError(
        error instanceof Error
          ? error.message
          : "game match schedule update failed",
      );
    } finally {
      suppressGameStatusRealtime(1_000);
      setPendingAction(null);
    }
  }

  async function updateGameScheduler(intervalSeconds: number): Promise<void> {
    suppressGameStatusRealtime();
    setPendingAction("game:scheduler:update");
    setActionError(null);
    setActionNote(null);

    try {
      const response = await fetch("/api/admin/game/scheduler/interval", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ interval_seconds: intervalSeconds }),
      });
      const payload = await processApiResponse<AdminGameStatus["scheduler"]>(
        response,
        "/api/admin/game/scheduler/interval",
      );

      setGameState((current) => ({ ...current, scheduler: payload }));
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
      suppressGameStatusRealtime(1_000);
      setPendingAction(null);
    }
  }



  async function refreshPagedFeed<T>(opts: {
    silent: boolean;
    actionKey: string;
    url: string;
    errorLabel: string;
    setPage: (page: T) => void;
    formatNote: (page: T) => string;
    beforeSet?: () => void;
  }): Promise<void> {
    if (!opts.silent) {
      setPendingAction(opts.actionKey);
      setActionError(null);
      setActionNote(null);
    }

    try {
      const response = await fetch(opts.url);
      const payload = await processApiResponse<T>(response, opts.url);

      opts.beforeSet?.();
      opts.setPage(payload);
      if (!opts.silent) {
        setActionNote(opts.formatNote(payload));
      }
    } catch (error) {
      if (!opts.silent) {
        setActionError(
          error instanceof Error ? error.message : opts.errorLabel,
        );
      }
    } finally {
      if (!opts.silent) {
        setPendingAction(null);
      }
    }
  }

  async function refreshAttacks(
    silent = false,
    filters: AttackFilters = attackFilters,
  ): Promise<void> {
    return refreshPagedFeed<AdminAttackFeedPage>({
      silent,
      actionKey: "game:attacks",
      url: `/api/admin/game/attacks${buildAttackQueryString(filters)}`,
      errorLabel: "attack feed fetch failed",
      setPage: setAttackPageState,
      beforeSet: clearAttackHighlights,
      formatNote: (page) =>
        `Loaded ${page.items.length} accepted attack row(s) (${page.offset + 1}-${page.offset + page.items.length} of ${page.total_count}).`,
    });
  }

  async function refreshCheckerRuns(
    silent = false,
    filters: CheckerRunFilters = checkerRunFilters,
  ): Promise<void> {
    return refreshPagedFeed<AdminCheckerRunPage>({
      silent,
      actionKey: "game:checker-runs",
      url: `/api/admin/game/checker-runs${buildCheckerRunQueryString(filters)}`,
      errorLabel: "checker runs fetch failed",
      setPage: setCheckerRunPageState,
      formatNote: (page) =>
        `Loaded ${page.items.length} checker run row(s) from game-core (${page.offset + 1}-${page.offset + page.items.length} of ${page.total_count}).`,
    });
  }

  async function refreshSchedulerEvents(
    silent = false,
    filters: SchedulerEventFilters = schedulerEventFilters,
  ): Promise<void> {
    return refreshPagedFeed<AdminSchedulerEventPage>({
      silent,
      actionKey: "game:scheduler-events",
      url: `/api/admin/game/scheduler/events${buildSchedulerEventQueryString(filters)}`,
      errorLabel: "scheduler events fetch failed",
      setPage: setSchedulerEventPageState,
      formatNote: (page) =>
        `Loaded ${page.items.length} scheduler event row(s) from game-core (${page.offset + 1}-${page.offset + page.items.length} of ${page.total_count}).`,
    });
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
    const currentOffset = parseNonNegativeInteger(checkerRunFilters.offset) ?? 0;
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
    const next = computePageOffset(attackFilters, direction);
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
    const currentOffset = parseNonNegativeInteger(schedulerEventFilters.offset) ?? 0;
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
      const payload = await processApiResponse<AdminGameScoreRow[]>(
        response,
        "/api/admin/game/scoreboard",
      );

      setScoreRows(payload);
      if (!silent) {
        setActionNote(
          `Loaded ${payload.length} scoreboard row(s) from game-core.`,
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
      const payload = await processApiResponse<AdminGameScoreRow[]>(
        response,
        "/api/admin/game/scoring/recompute",
      );

      setScoreRows(payload);
      if (!silent) {
        setActionNote(
          `Recomputed ${payload.length} scoreboard row(s) from authoritative tick and submission state.`,
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
    suppressGameStatusRealtime();
    setPendingAction("game:advance");
    setActionError(null);
    setActionNote(null);

    try {
      const response = await fetch("/api/admin/game/ticks/advance", {
        method: "POST",
      });
      const payload = await processApiResponse<AdminGameTickStatus>(
        response,
        "/api/admin/game/ticks/advance",
      );

      setGameState((current) => ({
        current_tick: payload,
        total_ticks: Math.max(current.total_ticks + 1, payload.id),
        total_checker_runs:
          current.total_checker_runs + payload.total_checker_runs,
        successful_checker_runs:
          current.successful_checker_runs +
          payload.successful_checker_runs,
        failed_checker_runs:
          current.failed_checker_runs + payload.failed_checker_runs,
        skipped_checker_runs:
          current.skipped_checker_runs + payload.skipped_checker_runs,
      }));
      await refreshSchedulerEvents(true);
      await refreshCheckerRuns(true);
      await recomputeGameScoring(true);
      setActionNote(
        `Tick ${payload.id} completed with ${payload.successful_checker_runs} success, ${payload.failed_checker_runs} failed, and ${payload.skipped_checker_runs} skipped checker runs.`,
      );
    } catch (error) {
      setActionError(
        error instanceof Error
          ? error.message
          : "game-core tick advance failed",
      );
    } finally {
      suppressGameStatusRealtime(1_000);
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
        sourceBundlePath: "",
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
          sourceBundlePath: challenge.source_bundle_path,
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

  async function updateTeam(): Promise<boolean> {
    if (editingId === null) return false;
    setPendingAction("team:update");
    setActionError(null);
    setActionNote(null);
    try {
      const data = await persistEntity<AdminTeam>(
        `/api/admin/teams/${editingId}`,
        "PUT",
        { name: teamDraft.name, contact_email: teamDraft.contactEmail },
        "team",
      );
      setTeamRows((current) =>
        current.map((t) => (t.id === editingId ? data : t)),
      );
      setActionNote(`Updated team ${data.name}.`);
      return true;
    } catch (error) {
      setActionError(
        error instanceof Error ? error.message : "team update failed",
      );
      return false;
    } finally {
      setPendingAction(null);
    }
  }

  async function updatePlayer(): Promise<boolean> {
    if (editingId === null) return false;
    setPendingAction("player:update");
    setActionError(null);
    setActionNote(null);
    try {
      const data = await persistEntity<AdminPlayer>(
        `/api/admin/players/${editingId}`,
        "PUT",
        {
          display_name: playerDraft.displayName,
          email: playerDraft.email,
          role: playerDraft.role,
        },
        "player",
      );
      setPlayerRows((current) =>
        current.map((p) => (p.id === editingId ? data : p)),
      );
      setActionNote(`Updated player ${data.display_name}.`);
      return true;
    } catch (error) {
      setActionError(
        error instanceof Error ? error.message : "player update failed",
      );
      return false;
    } finally {
      setPendingAction(null);
    }
  }

  async function updateChallenge(): Promise<boolean> {
    if (editingId === null) return false;
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
          source_bundle_path: challengeDraft.sourceBundlePath,
          weight: Number(challengeDraft.weight) || 1,
        }),
      });
      const payload = await processApiResponse<AdminChallenge>(
        response,
        `/api/admin/challenges/${editingId}`,
      );
      setChallengeRows((current) =>
        current.map((c) => (c.id === editingId ? payload : c)),
      );
      setActionNote(`Updated challenge ${payload.name}.`);
      return true;
    } catch (error) {
      setActionError(
        error instanceof Error ? error.message : "challenge update failed",
      );
      return false;
    } finally {
      setPendingAction(null);
    }
  }

  async function submitForm(): Promise<void> {
    if (formMode === "create") {
      let created = false;
      if (formEntity === "team") created = await createTeam();
      else if (formEntity === "player") created = await createPlayer();
      else if (formEntity === "challenge") created = await createChallenge();
      if (created) {
        closeFormDialog();
      }
    } else if (formMode === "edit") {
      let updated = false;
      if (formEntity === "team") updated = await updateTeam();
      else if (formEntity === "player") updated = await updatePlayer();
      else if (formEntity === "challenge") updated = await updateChallenge();
      if (updated) {
        closeFormDialog();
      }
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
    createChallenge: async () => {
      await createChallenge();
    },
    createPlayer: async () => {
      await createPlayer();
    },
    createTeam: async () => {
      await createTeam();
    },
    copyRuntimeHealthSummary,
    deleteTarget,
    downloadRuntimeHealthReport,
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
    refreshDeploymentRows,
    refreshGameScoreboard,
    refreshGameStatus,
    refreshOperationsStatus,
    refreshRuntimeHealth,
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
