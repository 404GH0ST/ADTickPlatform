"use client";

import type { ReactElement } from "react";

import {
  ChallengesTab,
  DeploymentsTab,
  EntityFormDialog,
  ErrorBanner,
  GameAttacksCard,
  GameTab,
  NoteBanner,
  PlayersTab,
  ScoreboardTab,
  TeamsTab,
} from "@/components/admin/organizer-dashboard-sections";
import {
  type OrganizerDashboardOptions,
  type OrganizerDashboardState,
  type DeleteTarget,
  useOrganizerDashboard,
} from "@/components/admin/use-organizer-dashboard";

export function OrganizerDashboard({
  attackPage,
  teams,
  players,
  challenges,
  deployments,
  gameStatus,
  operationsStatus,
  serviceMetrics,
  schedulerEventPage,
  checkerRunPage,
  scoreboard,
  initialTab = "teams",
}: OrganizerDashboardOptions & {
  initialTab?:
    | "teams"
    | "players"
    | "challenges"
    | "deployments"
    | "game"
    | "scoreboard"
    | "attacks";
}): ReactElement {
  const state = useOrganizerDashboard({
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
  });

  return (
    <div className="grid gap-4">
      {state.actionError ? <ErrorBanner message={state.actionError} /> : null}
      {state.actionNote ? <NoteBanner message={state.actionNote} /> : null}

      {renderOrganizerPanel({
        accessStatus: state.accessStatus,
        attackFilters: state.attackFilters,
        attackPage: state.attackPageState,
        attacksLiveMode: state.attacksLiveMode,
        highlightedAttackIDs: state.highlightedAttackIDs,
        challengeRows: state.challengeRows,
        checkerRunFilters: state.checkerRunFilters,
        checkerRunPage: state.checkerRunPageState,
        checkerRunsLiveMode: state.checkerRunsLiveMode,
        challengeValidationRows: state.challengeValidationRows,
        deploymentRows: state.deploymentRows,
        gameState: state.gameState,
        operationsStatus: state.operationsStatus,
        serviceMetrics: state.serviceMetrics,
        initialTab,
        pendingAction: state.pendingAction,
        playerRows: state.playerRows,
        schedulerEventFilters: state.schedulerEventFilters,
        schedulerEventPage: state.schedulerEventPageState,
        schedulerEventsLiveMode: state.schedulerEventsLiveMode,
        scoreRows: state.scoreRows,
        selectedWireGuardPeer: state.selectedWireGuardPeer,
        teamRows: state.teamRows,
        wireGuardDialogOpen: state.wireGuardDialogOpen,
        wireGuardGatewayStatus: state.wireGuardGatewayStatus,
        onAdvanceGameTick: () => {
          void state.advanceGameTick();
        },
        onApplyAttackFilters: () => {
          void state.applyAttackFilters();
        },
        onApplyCheckerRunFilters: () => {
          void state.applyCheckerRunFilters();
        },
        onApplySchedulerEventFilters: () => {
          void state.applySchedulerEventFilters();
        },
        onCloseDeleteDialog: state.closeDeleteDialog,
        onCloseWireGuardDialog: state.closeWireGuardDialog,
        onConfirmDelete: () => {
          void state.confirmDelete();
        },
        onOpenCreateDialog: state.openCreateDialog,
        onOpenEditDialog: state.openEditDialog,
        onSelectDeleteTarget: state.selectDeleteTarget,
        deleteTarget: state.deleteTarget,
        onDeployChallenge: (challenge) => {
          void state.deployChallenge(challenge);
        },
        onDownloadWireGuardConfig: state.downloadWireGuardConfig,
        onInspectWireGuard: (player) => {
          void state.inspectWireGuard(player);
        },
        onPageAttacks: (direction) => {
          void state.pageAttacks(direction);
        },
        onPageCheckerRuns: (direction) => {
          void state.pageCheckerRuns(direction);
        },
        onPageSchedulerEvents: (direction) => {
          void state.pageSchedulerEvents(direction);
        },
        onRecomputeGameScoring: () => {
          void state.recomputeGameScoring();
        },
        onReconcileAccess: () => {
          void state.reconcileAccess();
        },
        onReconcileDeployments: () => {
          void state.reconcileDeployments();
        },
        onReconcileWireGuardGateway: () => {
          void state.reconcileWireGuardGateway();
        },
        onRefreshAccessStatus: () => {
          void state.refreshAccessStatus();
        },
        onRefreshAttacks: () => {
          void state.refreshAttacks();
        },
        onRefreshCheckerRuns: () => {
          void state.refreshCheckerRuns();
        },
        onRefreshGameScoreboard: () => {
          void state.refreshGameScoreboard();
        },
        onRefreshGameStatus: () => {
          void state.refreshGameStatus();
        },
        onRefreshOperationsStatus: () => {
          void state.refreshOperationsStatus();
        },
        onRefreshServiceMetrics: () => {
          void state.refreshServiceMetrics();
        },
        onRefreshSchedulerEvents: () => {
          void state.refreshSchedulerEvents();
        },
        onRefreshWireGuardGatewayStatus: () => {
          void state.refreshWireGuardGatewayStatus();
        },
        onResetAttackFilters: () => {
          void state.resetAttackFilters();
        },
        onResetCheckerRunFilters: () => {
          void state.resetCheckerRunFilters();
        },
        onResetSchedulerEventFilters: () => {
          void state.resetSchedulerEventFilters();
        },
        onRevokeWireGuard: (player) => {
          void state.revokeWireGuard(player);
        },
        onRotateWireGuard: (player) => {
          void state.rotateWireGuard(player);
        },
        onSchedulerEventFilterChange: state.setSchedulerEventFilters,
        onSetAttackFilters: state.setAttackFilters,
        onSetCheckerRunFilters: state.setCheckerRunFilters,
        onStartGameMatch: () => {
          void state.startGameMatch();
        },
        onStartGameScheduler: () => {
          void state.startGameScheduler();
        },
        onStopGameMatch: () => {
          void state.stopGameMatch();
        },
        onStopGameScheduler: () => {
          void state.stopGameScheduler();
        },
        onUpdateGameMatchSchedule: state.updateGameMatchSchedule,
        onUpdateGameScheduler: (intervalSeconds: number) => {
          void state.updateGameScheduler(intervalSeconds);
        },
        onTeardownAccess: () => {
          void state.teardownAccess();
        },
        onTeardownWireGuardGateway: () => {
          void state.teardownWireGuardGateway();
        },
        onValidateChallenge: (challenge) => {
          void state.validateChallenge(challenge);
        },
      })}

      <EntityFormDialog
        formMode={state.formMode}
        formEntity={state.formEntity}

        pendingAction={state.pendingAction}
        teamDraft={state.teamDraft}
        playerDraft={state.playerDraft}
        challengeDraft={state.challengeDraft}
        teamRows={state.teamRows}
        onTeamDraftChange={state.setTeamDraft}
        onPlayerDraftChange={state.setPlayerDraft}
        onChallengeDraftChange={state.setChallengeDraft}
        onClose={state.closeFormDialog}
        onSubmit={() => {
          void state.submitForm();
        }}
      />
    </div>
  );
}

function renderOrganizerPanel({
  accessStatus,
  attackFilters,
  attackPage,
  attacksLiveMode,
  highlightedAttackIDs,
  challengeRows,
  checkerRunFilters,
  checkerRunPage,
  checkerRunsLiveMode,
  challengeValidationRows,
  deploymentRows,
  gameState,
  operationsStatus,
  serviceMetrics,
  initialTab,
  pendingAction,
  playerRows,
  schedulerEventFilters,
  schedulerEventPage,
  schedulerEventsLiveMode,
  scoreRows,
  selectedWireGuardPeer,
  teamRows,
  wireGuardDialogOpen,
  wireGuardGatewayStatus,
  onAdvanceGameTick,
  onApplyAttackFilters,
  onApplyCheckerRunFilters,
  onApplySchedulerEventFilters,
  onCloseDeleteDialog,
  onCloseWireGuardDialog,
  onConfirmDelete,
  onOpenCreateDialog,
  onOpenEditDialog,
  onSelectDeleteTarget,
  deleteTarget,
  onDeployChallenge,
  onDownloadWireGuardConfig,
  onInspectWireGuard,
  onPageAttacks,
  onPageCheckerRuns,
  onPageSchedulerEvents,
  onRecomputeGameScoring,
  onReconcileAccess,
  onReconcileDeployments,
  onReconcileWireGuardGateway,
  onRefreshAccessStatus,
  onRefreshAttacks,
  onRefreshCheckerRuns,
  onRefreshGameScoreboard,
  onRefreshGameStatus,
  onRefreshOperationsStatus,
  onRefreshServiceMetrics,
  onRefreshSchedulerEvents,
  onRefreshWireGuardGatewayStatus,
  onResetAttackFilters,
  onResetCheckerRunFilters,
  onResetSchedulerEventFilters,
  onRevokeWireGuard,
  onRotateWireGuard,
  onSchedulerEventFilterChange,
  onSetAttackFilters,
  onSetCheckerRunFilters,
  onStartGameMatch,
  onStartGameScheduler,
  onStopGameMatch,
  onStopGameScheduler,
  onUpdateGameMatchSchedule,
  onUpdateGameScheduler,
  onTeardownAccess,
  onTeardownWireGuardGateway,
  onValidateChallenge,
}: {
  accessStatus: OrganizerDashboardState["accessStatus"];
  attackFilters: OrganizerDashboardState["attackFilters"];
  attackPage: OrganizerDashboardState["attackPageState"];
  attacksLiveMode: boolean;
  highlightedAttackIDs: OrganizerDashboardState["highlightedAttackIDs"];
  challengeRows: OrganizerDashboardOptions["challenges"];
  checkerRunFilters: OrganizerDashboardState["checkerRunFilters"];
  checkerRunPage: OrganizerDashboardState["checkerRunPageState"];
  checkerRunsLiveMode: boolean;
  challengeValidationRows: OrganizerDashboardState["challengeValidationRows"];
  deploymentRows: OrganizerDashboardOptions["deployments"];
  gameState: OrganizerDashboardOptions["gameStatus"];
  operationsStatus: OrganizerDashboardState["operationsStatus"];
  serviceMetrics: OrganizerDashboardState["serviceMetrics"];
  initialTab:
    | "teams"
    | "players"
    | "challenges"
    | "deployments"
    | "game"
    | "scoreboard"
    | "attacks";
  pendingAction: string | null;
  playerRows: OrganizerDashboardOptions["players"];
  schedulerEventFilters: OrganizerDashboardState["schedulerEventFilters"];
  schedulerEventPage: OrganizerDashboardState["schedulerEventPageState"];
  schedulerEventsLiveMode: boolean;
  scoreRows: OrganizerDashboardOptions["scoreboard"];
  selectedWireGuardPeer: OrganizerDashboardState["selectedWireGuardPeer"];
  teamRows: OrganizerDashboardOptions["teams"];
  wireGuardDialogOpen: boolean;
  wireGuardGatewayStatus: OrganizerDashboardState["wireGuardGatewayStatus"];
  onAdvanceGameTick: () => void;
  onApplyAttackFilters: () => void;
  onApplyCheckerRunFilters: () => void;
  onApplySchedulerEventFilters: () => void;
  onCloseDeleteDialog: () => void;
  onCloseWireGuardDialog: () => void;
  onConfirmDelete: () => void;
  onOpenCreateDialog: (entity: "team" | "player" | "challenge") => void;
  onOpenEditDialog: (
    entity: "team" | "player" | "challenge",
    id: number,
  ) => void;
  onSelectDeleteTarget: (target: DeleteTarget) => void;
  deleteTarget: DeleteTarget | null;
  onDeployChallenge: (
    challenge: OrganizerDashboardOptions["challenges"][number],
  ) => void;
  onDownloadWireGuardConfig: OrganizerDashboardState["downloadWireGuardConfig"];
  onInspectWireGuard: (
    player: OrganizerDashboardOptions["players"][number],
  ) => void;
  onPageAttacks: (direction: "prev" | "next") => void;
  onPageCheckerRuns: (direction: "prev" | "next") => void;
  onPageSchedulerEvents: (direction: "prev" | "next") => void;
  onRecomputeGameScoring: () => void;
  onReconcileAccess: () => void;
  onReconcileDeployments: () => void;
  onReconcileWireGuardGateway: () => void;
  onRefreshAccessStatus: () => void;
  onRefreshAttacks: () => void;
  onRefreshCheckerRuns: () => void;
  onRefreshGameScoreboard: () => void;
  onRefreshGameStatus: () => void;
  onRefreshOperationsStatus: () => void;
  onRefreshServiceMetrics: () => void;
  onRefreshSchedulerEvents: () => void;
  onRefreshWireGuardGatewayStatus: () => void;
  onResetAttackFilters: () => void;
  onResetCheckerRunFilters: () => void;
  onResetSchedulerEventFilters: () => void;
  onRevokeWireGuard: (
    player: OrganizerDashboardOptions["players"][number],
  ) => void;
  onRotateWireGuard: (
    player: OrganizerDashboardOptions["players"][number],
  ) => void;
  onSchedulerEventFilterChange: OrganizerDashboardState["setSchedulerEventFilters"];
  onSetAttackFilters: OrganizerDashboardState["setAttackFilters"];
  onSetCheckerRunFilters: OrganizerDashboardState["setCheckerRunFilters"];
  onStartGameMatch: () => void;
  onStartGameScheduler: () => void;
  onStopGameMatch: () => void;
  onStopGameScheduler: () => void;
  onUpdateGameMatchSchedule: OrganizerDashboardState["updateGameMatchSchedule"];
  onUpdateGameScheduler: (intervalSeconds: number) => void;
  onTeardownAccess: () => void;
  onTeardownWireGuardGateway: () => void;
  onValidateChallenge: (
    challenge: OrganizerDashboardOptions["challenges"][number],
  ) => void;
}): ReactElement {
  if (initialTab === "teams") {
    return (
      <TeamsTab
        deleteTarget={deleteTarget}
        pendingAction={pendingAction}
        teamRows={teamRows}
        onCloseDeleteDialog={onCloseDeleteDialog}
        onConfirmDelete={onConfirmDelete}
        onOpenCreateDialog={() => onOpenCreateDialog("team")}
        onOpenEditDialog={(id) => onOpenEditDialog("team", id)}
        onSelectDeleteTarget={onSelectDeleteTarget}
      />
    );
  }

  if (initialTab === "players") {
    return (
      <PlayersTab
        accessStatus={accessStatus}
        deleteTarget={deleteTarget}
        pendingAction={pendingAction}
        playerRows={playerRows}
        selectedWireGuardPeer={selectedWireGuardPeer}
        wireGuardDialogOpen={wireGuardDialogOpen}
        wireGuardGatewayStatus={wireGuardGatewayStatus}
        onCloseDeleteDialog={onCloseDeleteDialog}
        onCloseWireGuardDialog={onCloseWireGuardDialog}
        onConfirmDelete={onConfirmDelete}
        onOpenCreateDialog={() => onOpenCreateDialog("player")}
        onOpenEditDialog={(id) => onOpenEditDialog("player", id)}
        onSelectDeleteTarget={onSelectDeleteTarget}
        onDownloadWireGuardConfig={onDownloadWireGuardConfig}
        onInspectWireGuard={onInspectWireGuard}
        onReconcileAccess={onReconcileAccess}
        onReconcileWireGuardGateway={onReconcileWireGuardGateway}
        onRefreshAccessStatus={onRefreshAccessStatus}
        onRefreshWireGuardGatewayStatus={onRefreshWireGuardGatewayStatus}
        onRevokeWireGuard={onRevokeWireGuard}
        onRotateWireGuard={onRotateWireGuard}
        onTeardownAccess={onTeardownAccess}
        onTeardownWireGuardGateway={onTeardownWireGuardGateway}
      />
    );
  }

  if (initialTab === "challenges") {
    return (
      <ChallengesTab
        challengeRows={challengeRows}
        deleteTarget={deleteTarget}
        pendingAction={pendingAction}
        validationRows={challengeValidationRows}
        onCloseDeleteDialog={onCloseDeleteDialog}
        onConfirmDelete={onConfirmDelete}
        onOpenCreateDialog={() => onOpenCreateDialog("challenge")}
        onOpenEditDialog={(id) => onOpenEditDialog("challenge", id)}
        onSelectDeleteTarget={onSelectDeleteTarget}
        onDeployChallenge={onDeployChallenge}
        onValidateChallenge={onValidateChallenge}
      />
    );
  }

  if (initialTab === "deployments") {
    return (
      <DeploymentsTab
        deleteTarget={deleteTarget}
        deploymentRows={deploymentRows}
        pendingAction={pendingAction}
        onCloseDeleteDialog={onCloseDeleteDialog}
        onConfirmDelete={onConfirmDelete}
        onReconcileDeployments={onReconcileDeployments}
        onSelectDeleteTarget={onSelectDeleteTarget}
      />
    );
  }

  if (initialTab === "scoreboard") {
    return (
      <ScoreboardTab
        pendingAction={pendingAction}
        scoreRows={scoreRows}
        onRefresh={onRefreshGameScoreboard}
      />
    );
  }

  if (initialTab === "attacks") {
    return (
      <GameAttacksCard
        attackPage={attackPage}
        attacksLiveMode={attacksLiveMode}
        highlightedAttackIDs={highlightedAttackIDs}
        pendingAction={pendingAction}
        onRefresh={onRefreshAttacks}
      />
    );
  }

  return (
      <GameTab
        attackPage={attackPage}
        attacksLiveMode={attacksLiveMode}
        highlightedAttackIDs={highlightedAttackIDs}
        checkerRunPage={checkerRunPage}
        checkerRunsLiveMode={checkerRunsLiveMode}
        filters={{
          attack: attackFilters,
          checkerRun: checkerRunFilters,
          schedulerEvent: schedulerEventFilters,
        }}
        gameState={gameState}
        operationsStatus={operationsStatus}
        serviceMetrics={serviceMetrics}
        pendingAction={pendingAction}
        schedulerEventPage={schedulerEventPage}
        schedulerEventsLiveMode={schedulerEventsLiveMode}
        scoreRows={scoreRows}
      onAdvanceTick={onAdvanceGameTick}
      onApplyAttackFilters={onApplyAttackFilters}
      onApplyCheckerRunFilters={onApplyCheckerRunFilters}
      onApplySchedulerEventFilters={onApplySchedulerEventFilters}
      onAttackFilterChange={onSetAttackFilters}
      onPageAttacks={onPageAttacks}
      onCheckerRunFilterChange={onSetCheckerRunFilters}
      onPageCheckerRuns={onPageCheckerRuns}
      onPageSchedulerEvents={onPageSchedulerEvents}
      onRecomputeScores={onRecomputeGameScoring}
      onRefreshAttacks={onRefreshAttacks}
      onRefreshCheckerRuns={onRefreshCheckerRuns}
        onRefreshGameScoreboard={onRefreshGameScoreboard}
        onRefreshGameStatus={onRefreshGameStatus}
        onRefreshOperationsStatus={onRefreshOperationsStatus}
        onRefreshServiceMetrics={onRefreshServiceMetrics}
        onRefreshSchedulerEvents={onRefreshSchedulerEvents}
      onResetAttackFilters={onResetAttackFilters}
      onResetCheckerRunFilters={onResetCheckerRunFilters}
      onResetSchedulerEventFilters={onResetSchedulerEventFilters}
      onSchedulerEventFilterChange={onSchedulerEventFilterChange}
      onStartMatch={onStartGameMatch}
      onStartScheduler={onStartGameScheduler}
      onStopMatch={onStopGameMatch}
      onStopScheduler={onStopGameScheduler}
      onUpdateMatchSchedule={onUpdateGameMatchSchedule}
      onUpdateScheduler={onUpdateGameScheduler}
      focusPanel="all"
    />
  );
}
