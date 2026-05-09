"use client";

import Link from "next/link";
import type { ReactElement, ReactNode } from "react";
import { useState, useEffect } from "react";
import {
  Copy,
  Download,
  Flag,
  LoaderCircle,
  Network,
  Pencil,
  RefreshCw,
  Trash2,
} from "lucide-react";

import type {
  AdminAttackFeedPage,
  AdminChallenge,
  AdminChallengeValidationResult,
  AdminCheckerRunPage,
  AdminControllerAccessStatus,
  AdminDeploymentJob,
  AdminGameScoreRow,
  AdminGameStatus,
  AdminGameTickStatus,
  AdminOperationsStatus,
  AdminServiceMetricSnapshot,
  AdminPlayer,
  AdminSchedulerEventPage,
  AdminTeam,
  AdminWireGuardGatewayStatus,
  AdminWireGuardPeer,
} from "@/lib/admin-dashboard-types";
import type {
  DeleteTarget,
  FormMode,
  FormEntity,
} from "@/components/admin/use-organizer-dashboard";
import { AttackMapPanel } from "@/components/ui/attack-map-panel";
import { AttackSliceSummaryGrid } from "@/components/ui/attack-slice-summary";
import { AdminRegistryCard } from "@/components/admin/admin-registry-card";
import { AdminRuntimeCard } from "@/components/admin/admin-runtime-card";
import { Badge } from "@/components/ui/badge";
import { AdminTable, AdminTableHeader } from "@/components/ui/admin-table";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { AppDialog } from "@/components/ui/app-dialog";
import { EmptyStateText, EmptyTableRow } from "@/components/ui/empty-state";
import { InfoLine, InfoPanel } from "@/components/ui/info-panel";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  LiveModeBadge,
  PagedFilterActions,
  SliceCountBadge,
} from "@/components/ui/paged-filter-controls";
import { StatusBanner } from "@/components/ui/status-banner";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { cn } from "@/lib/utils";
import { formatIndonesianDate } from "@/lib/date-format";
import { ScoreboardTable } from "@/components/ui/scoreboard-table";
import { AttackFeedTable } from "@/components/ui/attack-feed-table";

type TeamDraft = {
  name: string;
  contactEmail: string;
};

type PlayerDraft = {
  teamId: number;
  displayName: string;
  email: string;
  password: string;
  role: string;
};

type ChallengeDraft = {
  name: string;
  baselineImage: string;
  checkerImage: string;
  sourceBundlePath: string;
  servicePort: string;
  serviceSubnetOctet: string;
  weight: string;
};

type TeamsTabProps = {
  deleteTarget: DeleteTarget | null;
  pendingAction: string | null;
  teamRows: AdminTeam[];
  onCloseDeleteDialog: () => void;
  onConfirmDelete: () => void;
  onOpenCreateDialog: () => void;
  onOpenEditDialog: (id: number) => void;
  onSelectDeleteTarget: (target: DeleteTarget) => void;
};

type PlayersTabProps = {
  accessStatus: AdminControllerAccessStatus | null;
  deleteTarget: DeleteTarget | null;
  pendingAction: string | null;
  playerRows: AdminPlayer[];
  selectedWireGuardPeer: AdminWireGuardPeer | null;
  wireGuardDialogOpen: boolean;
  wireGuardGatewayStatus: AdminWireGuardGatewayStatus | null;
  onCloseDeleteDialog: () => void;
  onCloseWireGuardDialog: () => void;
  onConfirmDelete: () => void;
  onOpenCreateDialog: () => void;
  onOpenEditDialog: (id: number) => void;
  onDownloadWireGuardConfig: (peer: AdminWireGuardPeer) => void;
  onInspectWireGuard: (player: AdminPlayer) => void;
  onReconcileAccess: () => void;
  onReconcileWireGuardGateway: () => void;
  onRefreshAccessStatus: () => void;
  onRefreshWireGuardGatewayStatus: () => void;
  onRevokeWireGuard: (player: AdminPlayer) => void;
  onRotateWireGuard: (player: AdminPlayer) => void;
  onTeardownAccess: () => void;
  onTeardownWireGuardGateway: () => void;
  onSelectDeleteTarget: (target: DeleteTarget) => void;
};

type ChallengesTabProps = {
  challengeRows: AdminChallenge[];
  deleteTarget: DeleteTarget | null;
  pendingAction: string | null;
  validationRows: Record<number, AdminChallengeValidationResult>;
  onCloseDeleteDialog: () => void;
  onConfirmDelete: () => void;
  onOpenCreateDialog: () => void;
  onOpenEditDialog: (id: number) => void;
  onSelectDeleteTarget: (target: DeleteTarget) => void;
  onDeployChallenge: (challenge: AdminChallenge) => void;
  onValidateChallenge: (challenge: AdminChallenge) => void;
};

type DeploymentsTabProps = {
  deleteTarget: DeleteTarget | null;
  deploymentRows: AdminDeploymentJob[];
  pendingAction: string | null;
  onCloseDeleteDialog: () => void;
  onConfirmDelete: () => void;
  onReconcileDeployments: () => void;
  onSelectDeleteTarget: (target: DeleteTarget) => void;
};

type ScoreboardTabProps = {
  pendingAction: string | null;
  scoreRows: AdminGameScoreRow[];
  onRefresh: () => void;
};

type GameFilters = {
  attack: {
    limit: string;
    offset: string;
    attacker: string;
    victim: string;
    service: string;
    tickFrom: string;
    tickTo: string;
  };
  checkerRun: {
    limit: string;
    offset: string;
    tickId: string;
    teamId: string;
    challengeId: string;
    phase: string;
    status: string;
  };
  schedulerEvent: {
    limit: string;
    offset: string;
    eventType: string;
    source: string;
    state: string;
  };
};

type GameTabProps = {
  accessStatus: AdminControllerAccessStatus | null;
  attackPage: AdminAttackFeedPage;
  attacksLiveMode: boolean;
  highlightedAttackIDs: string[];
  checkerRunPage: AdminCheckerRunPage;
  checkerRunsLiveMode: boolean;
  deploymentRows: AdminDeploymentJob[];
  filters: GameFilters;
  focusPanel?: "all" | "attacks";
  gameState: AdminGameStatus;
  operationsStatus: AdminOperationsStatus | null;
  serviceMetrics: AdminServiceMetricSnapshot | null;
  pendingAction: string | null;
  schedulerEventPage: AdminSchedulerEventPage;
  schedulerEventsLiveMode: boolean;
  scoreRows: AdminGameScoreRow[];
  wireGuardGatewayStatus: AdminWireGuardGatewayStatus | null;
  onAdvanceTick: () => void;
  onApplyAttackFilters: () => void;
  onApplyCheckerRunFilters: () => void;
  onApplySchedulerEventFilters: () => void;
  onAttackFilterChange: (next: GameFilters["attack"]) => void;
  onCheckerRunFilterChange: (next: GameFilters["checkerRun"]) => void;
  onCopyRuntimeHealthSummary: () => void;
  onDownloadRuntimeHealthReport: () => void;
  onPageAttacks: (direction: "prev" | "next") => void;
  onPageCheckerRuns: (direction: "prev" | "next") => void;
  onPageSchedulerEvents: (direction: "prev" | "next") => void;
  onRecomputeScores: () => void;
  onRefreshAttacks: () => void;
  onRefreshCheckerRuns: () => void;
  onRefreshDeploymentRows: () => void;
  onRefreshGameScoreboard: () => void;
  onRefreshGameStatus: () => void;
  onRefreshOperationsStatus: () => void;
  onRefreshRuntimeHealth: () => void;
  onRefreshServiceMetrics: () => void;
  onRefreshSchedulerEvents: () => void;
  onResetAttackFilters: () => void;
  onResetCheckerRunFilters: () => void;
  onResetSchedulerEventFilters: () => void;
  onSchedulerEventFilterChange: (next: GameFilters["schedulerEvent"]) => void;
  onStartMatch: () => void;
  onStartScheduler: () => void;
  onStopMatch: () => void;
  onStopScheduler: () => void;
  onUpdateMatchSchedule: (schedule: {
    scheduledStartAt?: string;
    scheduledEndAt?: string;
  }) => void;
  onUpdateScheduler: (intervalSeconds: number) => void;
};

const challengeTone = {
  ready: "tone-success",
  deploying: "tone-neutral",
  draft: "tone-warning",
} as const;

const validationTone = {
  valid: "tone-success",
  invalid: "tone-danger",
  unavailable: "tone-warning",
  unchecked: "tone-neutral",
} as const;

const peerTone = {
  active: "tone-success",
  revoked: "tone-danger",
} as const;

const gatewayTone = {
  applied: "tone-success",
  idle: "tone-warning",
  disabled: "tone-neutral",
  error: "tone-danger",
} as const;

const checkerRunTone = {
  success: "tone-success",
  failed: "tone-danger",
  skipped: "tone-warning",
} as const;

const selectClassName =
  "flex h-10 w-full rounded-md border border-input bg-background px-3 text-sm outline-none ring-offset-background transition focus-visible:ring-2 focus-visible:ring-ring";

function toDateTimeLocalValue(isoString?: string): string {
  if (!isoString) {
    return "";
  }

  const parsed = new Date(isoString);
  if (Number.isNaN(parsed.getTime())) {
    return "";
  }

  const local = new Date(parsed.getTime() - parsed.getTimezoneOffset() * 60000);
  return local.toISOString().slice(0, 16);
}

function fromDateTimeLocalValue(value: string): string | undefined {
  if (!value) {
    return undefined;
  }

  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return undefined;
  }
  return parsed.toISOString();
}

function datePartFromDateTimeLocalValue(value: string): string {
  if (!value) {
    return "";
  }
  const [datePart] = value.split("T");
  return datePart ?? "";
}

function timePartFromDateTimeLocalValue(value: string): string {
  if (!value) {
    return "";
  }
  const [, timePart] = value.split("T");
  return timePart ?? "";
}

function withUpdatedDatePart(currentValue: string, nextDate: string): string {
  if (!nextDate) {
    return "";
  }
  const timePart = timePartFromDateTimeLocalValue(currentValue) || "00:00";
  return `${nextDate}T${timePart}`;
}

function withUpdatedTimePart(currentValue: string, nextTime: string): string {
  if (!nextTime) {
    return "";
  }
  const datePart = datePartFromDateTimeLocalValue(currentValue);
  if (!datePart) {
    return "";
  }
  return `${datePart}T${nextTime}`;
}

function CardActionRow({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}): ReactElement {
  return (
    <div className={cn("flex flex-wrap items-center gap-2 pt-1", className)}>
      {children}
    </div>
  );
}

export function TeamsTab({
  deleteTarget,
  pendingAction,
  teamRows,
  onCloseDeleteDialog,
  onConfirmDelete,
  onOpenCreateDialog,
  onOpenEditDialog,
  onSelectDeleteTarget,
}: TeamsTabProps): ReactElement {
  return (
    <>
    <AdminRegistryCard
      title="Team Registry"
      description="Teams, player counts, and how many published services each team currently owns."
      createLabel="Create Team"
      onCreate={onOpenCreateDialog}
    >
      <Table>
            <TableHeader>
              <TableRow>
                <TableHead>ID</TableHead>
                <TableHead>Name</TableHead>
                <TableHead>Contact</TableHead>
                <TableHead>Players</TableHead>
                <TableHead>Deployed</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {teamRows.length === 0 && (
                <EmptyTableRow
                  colSpan={6}
                  message="No teams yet. Click Create Team to add one."
                />
              )}
              {teamRows.map((team) => (
                <TableRow key={team.id} data-testid={`team-row-${team.id}`}>
                  <TableCell className="font-semibold">#{team.id}</TableCell>
                  <TableCell>{team.name}</TableCell>
                  <TableCell>{team.contact_email}</TableCell>
                  <TableCell>{team.player_count}</TableCell>
                  <TableCell>{team.deployed_challenges}</TableCell>
                  <TableCell className="text-right">
                    <Button
                      size="sm"
                      variant="ghost"
                      data-testid={`edit-team-${team.id}`}
                      disabled={pendingAction !== null}
                      onClick={() => onOpenEditDialog(team.id)}
                    >
                      <Pencil className="h-4 w-4" />
                    </Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      className="button-danger-subtle"
                      data-testid={`delete-team-${team.id}`}
                      disabled={pendingAction !== null}
                      onClick={() =>
                        onSelectDeleteTarget({
                          kind: "team",
                          id: team.id,
                          label: team.name,
                          warning:
                            "This will also delete all associated players and service states.",
                        })
                      }
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
    </AdminRegistryCard>
      <DeleteConfirmDialog
        deleteTarget={deleteTarget}
        pendingAction={pendingAction}
        onClose={onCloseDeleteDialog}
        onConfirm={onConfirmDelete}
      />
    </>
  );
}

export function PlayersTab({
  accessStatus,
  deleteTarget,
  pendingAction,
  playerRows,
  selectedWireGuardPeer,
  wireGuardDialogOpen,
  wireGuardGatewayStatus,
  onCloseDeleteDialog,
  onCloseWireGuardDialog,
  onConfirmDelete,
  onOpenCreateDialog,
  onOpenEditDialog,
  onSelectDeleteTarget,
  onDownloadWireGuardConfig,
  onInspectWireGuard,
  onReconcileAccess,
  onReconcileWireGuardGateway,
  onRefreshAccessStatus,
  onRefreshWireGuardGatewayStatus,
  onRevokeWireGuard,
  onRotateWireGuard,
  onTeardownAccess,
  onTeardownWireGuardGateway,
}: PlayersTabProps): ReactElement {
  return (
    <>
      <div className="grid gap-4">
        <div className="grid items-start gap-4 xl:grid-cols-2">
          <WireGuardGatewayCard
            pendingAction={pendingAction}
            wireGuardGatewayStatus={wireGuardGatewayStatus}
            onReconcileWireGuardGateway={onReconcileWireGuardGateway}
            onRefreshWireGuardGatewayStatus={onRefreshWireGuardGatewayStatus}
            onTeardownWireGuardGateway={onTeardownWireGuardGateway}
          />

          <AccessPolicyCard
            accessStatus={accessStatus}
            pendingAction={pendingAction}
            onReconcileAccess={onReconcileAccess}
            onRefreshAccessStatus={onRefreshAccessStatus}
            onTeardownAccess={onTeardownAccess}
          />
        </div>

        <PlayerRegistryCard
          pendingAction={pendingAction}
          playerRows={playerRows}
          onOpenCreateDialog={onOpenCreateDialog}
          onOpenEditDialog={onOpenEditDialog}
          onInspectWireGuard={onInspectWireGuard}
          onRevokeWireGuard={onRevokeWireGuard}
          onRotateWireGuard={onRotateWireGuard}
          onSelectDeleteTarget={onSelectDeleteTarget}
        />
      </div>

      <WireGuardDialog
        open={wireGuardDialogOpen}
        peer={selectedWireGuardPeer}
        onClose={onCloseWireGuardDialog}
        onDownload={onDownloadWireGuardConfig}
      />
      <DeleteConfirmDialog
        deleteTarget={deleteTarget}
        pendingAction={pendingAction}
        onClose={onCloseDeleteDialog}
        onConfirm={onConfirmDelete}
      />
    </>
  );
}

export function ChallengesTab({
  challengeRows,
  deleteTarget,
  pendingAction,
  validationRows,
  onCloseDeleteDialog,
  onConfirmDelete,
  onOpenCreateDialog,
  onOpenEditDialog,
  onSelectDeleteTarget,
  onDeployChallenge,
  onValidateChallenge,
}: ChallengesTabProps): ReactElement {
  return (
    <>
      <div className="grid gap-4">
        <ChallengeCatalogCard
          challengeRows={challengeRows}
          pendingAction={pendingAction}
          validationRows={validationRows}
          onOpenCreateDialog={onOpenCreateDialog}
          onOpenEditDialog={onOpenEditDialog}
          onSelectDeleteTarget={onSelectDeleteTarget}
          onDeployChallenge={onDeployChallenge}
          onValidateChallenge={onValidateChallenge}
        />
      </div>
      <DeleteConfirmDialog
        deleteTarget={deleteTarget}
        pendingAction={pendingAction}
        onClose={onCloseDeleteDialog}
        onConfirm={onConfirmDelete}
      />
    </>
  );
}

function WireGuardGatewayCard({
  pendingAction,
  wireGuardGatewayStatus,
  onReconcileWireGuardGateway,
  onRefreshWireGuardGatewayStatus,
  onTeardownWireGuardGateway,
}: {
  pendingAction: string | null;
  wireGuardGatewayStatus: AdminWireGuardGatewayStatus | null;
  onReconcileWireGuardGateway: () => void;
  onRefreshWireGuardGatewayStatus: () => void;
  onTeardownWireGuardGateway: () => void;
}): ReactElement {
  return (
    <AdminRuntimeCard
      title="WireGuard Gateway"
      description="Maintenance surface for applying player peer changes to the gateway runtime after create, rotate, and revoke operations."
    >
        <RuntimeStatusBlock
          tone={getGatewayTone(wireGuardGatewayStatus?.state)}
          state={wireGuardGatewayStatus?.state ?? "loading"}
          rows={wireGuardGatewayRows(wireGuardGatewayStatus)}
          error={wireGuardGatewayStatus?.last_error}
        />
        <WireGuardGatewayActions
          pendingAction={pendingAction}
          onReconcile={onReconcileWireGuardGateway}
          onRefresh={onRefreshWireGuardGatewayStatus}
          onTeardown={onTeardownWireGuardGateway}
        />
    </AdminRuntimeCard>
  );
}

function AccessPolicyCard({
  accessStatus,
  pendingAction,
  onReconcileAccess,
  onRefreshAccessStatus,
  onTeardownAccess,
}: {
  accessStatus: AdminControllerAccessStatus | null;
  pendingAction: string | null;
  onReconcileAccess: () => void;
  onRefreshAccessStatus: () => void;
  onTeardownAccess: () => void;
}): ReactElement {
  return (
    <AdminRuntimeCard
      title="Service Access Policy"
      description="Maintenance surface for controller-side SSH allowlists rendered from unlock state plus active team peer addresses."
    >
        <RuntimeStatusBlock
          tone={getGatewayTone(accessStatus?.state)}
          state={accessStatus?.state ?? "loading"}
          rows={accessPolicyRows(accessStatus)}
          error={accessStatus?.last_error}
        />
        <AccessPolicyActions
          pendingAction={pendingAction}
          onReconcile={onReconcileAccess}
          onRefresh={onRefreshAccessStatus}
          onTeardown={onTeardownAccess}
        />
    </AdminRuntimeCard>
  );
}

type RuntimeStatusRow = { label: string; value?: string };

function wireGuardGatewayRows(
  status: AdminWireGuardGatewayStatus | null,
): RuntimeStatusRow[] {
  if (!status) {
    return runtimeNotLoadedRows(["Interface", "Firewall", "Config", "Rules"], [
      { label: "Mode", value: "unknown" },
      { label: "Peers", value: "not loaded" },
      { label: "Revision", value: "not applied yet" },
      { label: "Applied at", value: "n/a" },
    ]);
  }

  return [
    { label: "Mode", value: status.mode },
    { label: "Interface", value: status.interface },
    { label: "Firewall", value: status.firewall_backend },
    { label: "Peers", value: formatWireGuardPeerCounts(status) },
    { label: "Revision", value: status.revision },
    { label: "Applied at", value: formatIndonesianDate(status.applied_at) },
    { label: "Config", value: status.config_path },
    { label: "Rules", value: status.rules_path },
  ];
}

function formatWireGuardPeerCounts(
  status: AdminWireGuardGatewayStatus,
): string {
  return `${status.peers_active} active / ${status.peers_revoked} revoked / ${status.peers_total} total`;
}

function accessPolicyRows(
  status: AdminControllerAccessStatus | null,
): RuntimeStatusRow[] {
  if (!status) {
    return runtimeNotLoadedRows(["Interface", "Firewall", "Rules"], [
      { label: "Mode", value: "unknown" },
      { label: "Policies", value: "not loaded" },
      { label: "Allowed peers", value: "not loaded" },
      { label: "Revision", value: "not applied yet" },
      { label: "Applied at", value: "n/a" },
    ]);
  }

  return [
    { label: "Mode", value: status.mode },
    { label: "Interface", value: status.interface },
    { label: "Firewall", value: status.firewall_backend },
    { label: "Policies", value: formatAccessPolicyCounts(status) },
    { label: "Allowed peers", value: formatAllowedPeerCount(status) },
    { label: "Revision", value: status.revision },
    { label: "Applied at", value: formatIndonesianDate(status.applied_at) },
    { label: "Rules", value: status.rules_path },
  ];
}

function formatAccessPolicyCounts(
  status: AdminControllerAccessStatus,
): string {
  return `${status.policies_total} total / ${status.ssh_open_services} SSH-open / ${status.ssh_locked_services} SSH-locked`;
}

function formatAllowedPeerCount(
  status: AdminControllerAccessStatus,
): string {
  return String(status.allowed_peers_total);
}

function runtimeNotLoadedRows(
  emptyLabels: string[],
  rows: RuntimeStatusRow[],
): RuntimeStatusRow[] {
  return [
    ...rows,
    ...emptyLabels.map((label) => ({ label, value: undefined })),
  ];
}

function WireGuardGatewayActions({
  pendingAction,
  onReconcile,
  onRefresh,
  onTeardown,
}: {
  pendingAction: string | null;
  onReconcile: () => void;
  onRefresh: () => void;
  onTeardown: () => void;
}): ReactElement {
  return (
    <CardActionRow>
      <RuntimeActionButton
        actionID="wireguard-gateway:status"
        label="Refresh Status"
        pendingAction={pendingAction}
        testID="refresh-wireguard-gateway"
        variant="outline"
        onClick={onRefresh}
      />
      <RuntimeActionButton
        actionID="wireguard-gateway:reconcile"
        icon="network"
        label="Reconcile Gateway"
        pendingAction={pendingAction}
        testID="reconcile-wireguard-gateway"
        onClick={onReconcile}
      />
      <RuntimeActionButton
        actionID="wireguard-gateway:teardown"
        className="button-danger-subtle ml-auto"
        icon="trash"
        label="Teardown"
        pendingAction={pendingAction}
        testID="teardown-wireguard-gateway"
        variant="ghost"
        onClick={onTeardown}
      />
    </CardActionRow>
  );
}

function AccessPolicyActions({
  pendingAction,
  onReconcile,
  onRefresh,
  onTeardown,
}: {
  pendingAction: string | null;
  onReconcile: () => void;
  onRefresh: () => void;
  onTeardown: () => void;
}): ReactElement {
  return (
    <CardActionRow>
      <RuntimeActionButton
        actionID="access:status"
        label="Refresh Access"
        pendingAction={pendingAction}
        testID="refresh-access"
        variant="outline"
        onClick={onRefresh}
      />
      <RuntimeActionButton
        actionID="access:reconcile"
        icon="network"
        label="Reconcile Access"
        pendingAction={pendingAction}
        testID="reconcile-access"
        onClick={onReconcile}
      />
      <RuntimeActionButton
        actionID="access:teardown"
        className="button-danger-subtle ml-auto"
        icon="trash"
        label="Teardown"
        pendingAction={pendingAction}
        testID="teardown-access"
        variant="ghost"
        onClick={onTeardown}
      />
    </CardActionRow>
  );
}

function RuntimeActionButton({
  actionID,
  className,
  icon = "refresh",
  label,
  pendingAction,
  testID,
  variant,
  onClick,
}: {
  actionID: string;
  className?: string;
  icon?: "network" | "refresh" | "trash";
  label: string;
  pendingAction: string | null;
  testID: string;
  variant?: "default" | "ghost" | "outline";
  onClick: () => void;
}): ReactElement {
  return (
    <Button
      disabled={pendingAction !== null}
      variant={variant}
      className={className}
      data-testid={testID}
      onClick={onClick}
    >
      <RuntimeActionIcon active={pendingAction === actionID} icon={icon} />
      {label}
    </Button>
  );
}

function RuntimeActionIcon({
  active,
  icon,
}: {
  active: boolean;
  icon: "network" | "refresh" | "trash";
}): ReactElement {
  if (active) {
    return <LoaderCircle className="h-4 w-4 animate-spin" />;
  }

  if (icon === "network") {
    return <Network className="h-4 w-4" />;
  }

  if (icon === "trash") {
    return <Trash2 className="h-4 w-4" />;
  }

  return <RefreshCw className="h-4 w-4" />;
}


function PlayerRegistryCard({
  pendingAction,
  playerRows,
  onOpenCreateDialog,
  onOpenEditDialog,
  onInspectWireGuard,
  onRevokeWireGuard,
  onRotateWireGuard,
  onSelectDeleteTarget,
}: {
  pendingAction: string | null;
  playerRows: AdminPlayer[];
  onOpenCreateDialog: () => void;
  onOpenEditDialog: (id: number) => void;
  onInspectWireGuard: (player: AdminPlayer) => void;
  onRevokeWireGuard: (player: AdminPlayer) => void;
  onRotateWireGuard: (player: AdminPlayer) => void;
  onSelectDeleteTarget: (target: DeleteTarget) => void;
}): ReactElement {
  return (
    <AdminRegistryCard
      title="Player Registry"
      description="Each player gets a generated peer config, status tracking, organizer download/rotate/revoke actions, and a gateway reconcile path."
      createLabel="Create Player"
      onCreate={onOpenCreateDialog}
    >
      <AdminTable>
          <AdminTableHeader columns={[
            { label: "ID", className: "w-[60px]" },
            { label: "Name" },
            { label: "Team" },
            { label: "Role" },
            { label: "Peer" },
            { label: "Address" },
            { label: "Status" },
            { label: "Action" },
          ]} />
          <TableBody>
            {playerRows.map((player) => (
              <TableRow key={player.id} data-testid={`player-row-${player.id}`}>
                <TableCell className="font-semibold text-muted-foreground">#{player.id}</TableCell>
                <TableCell>
                  <div>
                    <p>{player.display_name}</p>
                    <p className="text-xs text-muted-foreground">
                      {player.email}
                    </p>
                  </div>
                </TableCell>
                <TableCell>{player.team_name}</TableCell>
                <TableCell>{player.role}</TableCell>
                <TableCell className="font-mono text-xs">
                  {player.wireguard_peer}
                </TableCell>
                <TableCell className="font-mono text-xs">
                  {player.wireguard_address}
                </TableCell>
                <TableCell>
                  <Badge
                    className={
                      player.wireguard_status === "active"
                        ? peerTone.active
                        : peerTone.revoked
                    }
                    variant="outline"
                  >
                    {player.wireguard_status}
                  </Badge>
                </TableCell>
                <TableCell>
                  <div className="flex flex-wrap items-center gap-2">
                    <Button
                      disabled={pendingAction !== null}
                      size="sm"
                      variant="ghost"
                      data-testid={`edit-player-${player.id}`}
                      onClick={() => onOpenEditDialog(player.id)}
                    >
                      <Pencil className="h-4 w-4" />
                    </Button>
                    <Button
                      disabled={pendingAction !== null}
                      size="sm"
                      variant="outline"
                      data-testid={`inspect-wireguard-${player.id}`}
                      onClick={() => onInspectWireGuard(player)}
                    >
                      {pendingAction === `wireguard:get:${player.id}` ? (
                        <LoaderCircle className="h-4 w-4 animate-spin" />
                      ) : (
                        <Download className="h-4 w-4" />
                      )}
                      Config
                    </Button>
                    <Button
                      disabled={pendingAction !== null}
                      size="sm"
                      variant="outline"
                      data-testid={`rotate-wireguard-${player.id}`}
                      onClick={() => onRotateWireGuard(player)}
                    >
                      {pendingAction === `wireguard:rotate:${player.id}` ? (
                        <LoaderCircle className="h-4 w-4 animate-spin" />
                      ) : (
                        <RefreshCw className="h-4 w-4" />
                      )}
                      Rotate
                    </Button>
                    <Button
                      disabled={
                        pendingAction !== null ||
                        player.wireguard_status === "revoked"
                      }
                      size="sm"
                      variant="outline"
                      data-testid={`revoke-wireguard-${player.id}`}
                      onClick={() => onRevokeWireGuard(player)}
                    >
                      {pendingAction === `wireguard:revoke:${player.id}` ? (
                        <LoaderCircle className="h-4 w-4 animate-spin" />
                      ) : null}
                      Revoke
                    </Button>
                    <Button
                      disabled={pendingAction !== null}
                      size="sm"
                      variant="ghost"
                      className="button-danger-subtle"
                      data-testid={`delete-player-${player.id}`}
                      onClick={() =>
                        onSelectDeleteTarget({
                          kind: "player",
                          id: player.id,
                          label: player.display_name,
                        })
                      }
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
          </AdminTable>
    </AdminRegistryCard>
  );
}


function ChallengeCatalogCard({
  challengeRows,
  pendingAction,
  validationRows,
  onOpenCreateDialog,
  onOpenEditDialog,
  onSelectDeleteTarget,
  onDeployChallenge,
  onValidateChallenge,
}: {
  challengeRows: AdminChallenge[];
  pendingAction: string | null;
  validationRows: Record<number, AdminChallengeValidationResult>;
  onOpenCreateDialog: () => void;
  onOpenEditDialog: (id: number) => void;
  onSelectDeleteTarget: (target: DeleteTarget) => void;
  onDeployChallenge: (challenge: AdminChallenge) => void;
  onValidateChallenge: (challenge: AdminChallenge) => void;
}): ReactElement {
  return (
    <AdminRegistryCard
      title="Challenge Catalog"
      description="Validate the service and checker images first, then deploy to replicate one service instance per team."
      createLabel="Create Challenge"
      onCreate={onOpenCreateDialog}
    >
      <AdminTable>
          <AdminTableHeader columns={[
            { label: "ID", className: "w-[80px]" },
            { label: "Name" },
            { label: "Images" },
            { label: "Status" },
            { label: "Validation" },
            { label: "Weight" },
            { label: "Network" },
            { label: "Replication" },
            { label: "Runtime" },
            { label: "Action" },
          ]} />
          <TableBody>
            {challengeRows.map((challenge) => (
              <ChallengeCatalogRow
                key={`challenge-row-${challenge.id}`}
                challenge={challenge}
                pendingAction={pendingAction}
                validation={validationRows[challenge.id]}
                onOpenEditDialog={onOpenEditDialog}
                onSelectDeleteTarget={onSelectDeleteTarget}
                onDeployChallenge={onDeployChallenge}
                onValidateChallenge={onValidateChallenge}
              />
            ))}
          </TableBody>
        </AdminTable>
      </AdminRegistryCard>
  );
}

function ChallengeCatalogRow({
  challenge,
  pendingAction,
  validation,
  onOpenEditDialog,
  onSelectDeleteTarget,
  onDeployChallenge,
  onValidateChallenge,
}: {
  challenge: AdminChallenge;
  pendingAction: string | null;
  validation?: AdminChallengeValidationResult;
  onOpenEditDialog: (id: number) => void;
  onSelectDeleteTarget: (target: DeleteTarget) => void;
  onDeployChallenge: (challenge: AdminChallenge) => void;
  onValidateChallenge: (challenge: AdminChallenge) => void;
}): ReactElement {
  const validationStatus = validation?.status ?? "unchecked";

  return (
    <TableRow data-testid={`challenge-row-${challenge.id}`}>
      <TableCell className="font-semibold">#{challenge.id}</TableCell>
      <TableCell>{challenge.name}</TableCell>
      <TableCell>
        <div className="max-w-[20rem] text-xs text-muted-foreground">
          <p>{challenge.baseline_image}</p>
          <p>{challenge.checker_image}</p>
          {challenge.source_bundle_path ? (
            <p>{challenge.source_bundle_path}</p>
          ) : null}
        </div>
      </TableCell>
      <TableCell>
        <Badge
          className={getChallengeRuntimeTone(challenge.runtime_status)}
          variant="outline"
        >
          {challenge.runtime_status}
        </Badge>
      </TableCell>
      <TableCell>
        <div className="space-y-2">
          <Badge
            className={getValidationStatusTone(validationStatus)}
            variant="outline"
          >
            {validationStatus}
          </Badge>
          <p className="max-w-[18rem] text-xs leading-5 text-muted-foreground">
            {validation?.message ?? "No runtime preflight recorded yet."}
          </p>
          {validation ? (
            <p className="text-xs leading-5 text-muted-foreground">
              baseline {validation.baseline_ssh_contract_ok ? "ok" : "fail"} /
              checker {validation.checker_contract_ok ? "ok" : "fail"} /
              state {validation.service_state_contract_ok ? "ok" : "fail"}
            </p>
          ) : null}
        </div>
      </TableCell>
      <TableCell>{challenge.weight}</TableCell>
      <TableCell className="text-xs text-muted-foreground">
        <div>
          <p>10.80.{challenge.service_subnet_octet}.0/24</p>
          <p>port {challenge.service_port}</p>
        </div>
      </TableCell>
      <TableCell>
        {challenge.deployed_teams}/{challenge.total_teams}
      </TableCell>
      <TableCell className="text-sm text-muted-foreground">
        ready {challenge.ready_teams} / queued {challenge.queued_teams}
      </TableCell>
      <TableCell>
        <CardActionRow>
          <Button
            disabled={pendingAction !== null}
            size="sm"
            variant="ghost"
            data-testid={`edit-challenge-${challenge.id}`}
            onClick={() => onOpenEditDialog(challenge.id)}
          >
            <Pencil className="h-4 w-4" />
          </Button>
          <Button
            disabled={pendingAction !== null}
            size="sm"
            variant="outline"
            data-testid={`validate-challenge-${challenge.id}`}
            onClick={() => onValidateChallenge(challenge)}
          >
            {pendingAction === `validate:${challenge.id}` ? (
              <LoaderCircle className="h-4 w-4 animate-spin" />
            ) : null}
            Validate
          </Button>
          <Button
            disabled={pendingAction !== null}
            size="sm"
            variant={challenge.published ? "outline" : "default"}
            data-testid={`deploy-challenge-${challenge.id}`}
            onClick={() => onDeployChallenge(challenge)}
          >
            {pendingAction === `deploy:${challenge.id}` ? (
              <LoaderCircle className="h-4 w-4 animate-spin" />
            ) : (
              <Network className="h-4 w-4" />
            )}
            {challenge.published ? "Redeploy" : "Deploy"}
          </Button>
          <Button
            disabled={pendingAction !== null}
            size="sm"
            variant="ghost"
            className="button-danger-subtle"
            data-testid={`delete-challenge-${challenge.id}`}
            onClick={() =>
              onSelectDeleteTarget({
                kind: "challenge",
                id: challenge.id,
                label: challenge.name,
                warning:
                  "This will also delete all associated service instances.",
              })
            }
          >
            <Trash2 className="h-4 w-4" />
          </Button>
        </CardActionRow>
      </TableCell>
    </TableRow>
  );
}

export function DeploymentsTab({
  deleteTarget,
  deploymentRows,
  pendingAction,
  onCloseDeleteDialog,
  onConfirmDelete,
  onReconcileDeployments,
  onSelectDeleteTarget,
}: DeploymentsTabProps): ReactElement {
  return (
    <>
      <div className="grid gap-4 xl:grid-cols-[0.78fr_1.22fr]">
        <Card>
          <CardHeader>
            <CardTitle>Controller Reconcile</CardTitle>
            <CardDescription>
              Queued deployment jobs stay in provisioning until the controller
              marks their service replicas ready.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <InfoPanel className="leading-7">
              <p>
                Deploy creates one active service instance record per team and
                puts the rollout into a queued job.
              </p>
              <p>
                Trusted reconcile flips queued instances to ready and verifies
                controller access plus WireGuard truth before rollout success is
                reported.
              </p>
              <p>
                Inactive jobs can be deleted from the table without removing
                deployed services.
              </p>
            </InfoPanel>
            <CardActionRow>
              <Button
                disabled={pendingAction !== null || deploymentRows.length === 0}
                onClick={onReconcileDeployments}
              >
                {pendingAction === "deployments:reconcile" ? (
                  <LoaderCircle className="h-4 w-4 animate-spin" />
                ) : (
                  <Network className="h-4 w-4" />
                )}
                Reconcile Deployments
              </Button>
            </CardActionRow>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Deployment Jobs</CardTitle>
            <CardDescription>
              Organizer-visible rollout queue sourced from the admin API.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Job</TableHead>
                  <TableHead>Challenge</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Ready</TableHead>
                  <TableHead>Queued</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {deploymentRows.length === 0 ? (
                  <EmptyTableRow
                    colSpan={7}
                    message="No deployment jobs yet."
                  />
                ) : (
                  deploymentRows.map((deployment) => {
                    return (
                      <TableRow
                        key={deployment.id}
                        data-testid={`deployment-row-${deployment.id}`}
                      >
                        <TableCell className="font-semibold">
                          #{deployment.id}
                        </TableCell>
                        <TableCell>{deployment.challenge_name}</TableCell>
                        <TableCell>
                          <Badge
                            className={
                              deployment.status === "completed"
                                ? challengeTone.ready
                                : challengeTone.deploying
                            }
                            variant="outline"
                          >
                            {deployment.status}
                          </Badge>
                        </TableCell>
                        <TableCell>
                          {deployment.ready_team_count}/
                          {deployment.target_team_count}
                        </TableCell>
                        <TableCell>{deployment.queued_team_count}</TableCell>
                        <TableCell className="text-xs text-muted-foreground">
                          <div>
                            <p>{formatIndonesianDate(deployment.created_at)}</p>
                            {deployment.completed_at ? (
                              <p>
                                done{" "}
                                {formatIndonesianDate(deployment.completed_at)}
                              </p>
                            ) : null}
                          </div>
                        </TableCell>
                        <TableCell className="text-right">
                          <Button
                            size="sm"
                            variant="ghost"
                            className="button-danger-subtle"
                            data-testid={`delete-deployment-${deployment.id}`}
                            disabled={pendingAction !== null}
                            title={
                              "Delete deployment job. Active queued jobs will be rejected by the API."
                            }
                            onClick={() =>
                              onSelectDeleteTarget({
                                kind: "deployment",
                                id: deployment.id,
                                label: `#${deployment.id} ${deployment.challenge_name}`,
                                warning:
                                  "This only removes the deployment job record. Deployed services remain intact.",
                              })
                            }
                          >
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        </TableCell>
                      </TableRow>
                    );
                  })
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </div>
      <DeleteConfirmDialog
        deleteTarget={deleteTarget}
        pendingAction={pendingAction}
        onClose={onCloseDeleteDialog}
        onConfirm={onConfirmDelete}
      />
    </>
  );
}

export function GameTab({
  accessStatus,
  attackPage,
  attacksLiveMode,
  highlightedAttackIDs,
  checkerRunPage,
  checkerRunsLiveMode,
  deploymentRows,
  filters,
  focusPanel = "all",
  gameState,
  operationsStatus,
  serviceMetrics,
  pendingAction,
  schedulerEventPage,
  schedulerEventsLiveMode,
  scoreRows,
  wireGuardGatewayStatus,
  onAdvanceTick,

  onApplyCheckerRunFilters,
  onApplySchedulerEventFilters,

  onCheckerRunFilterChange,
  onCopyRuntimeHealthSummary,
  onDownloadRuntimeHealthReport,

  onPageCheckerRuns,
  onPageSchedulerEvents,
  onRecomputeScores,
  onRefreshAttacks,
  onRefreshCheckerRuns,
  onRefreshDeploymentRows,
  onRefreshGameScoreboard,
  onRefreshGameStatus,
  onRefreshOperationsStatus,
  onRefreshRuntimeHealth,
  onRefreshServiceMetrics,
  onRefreshSchedulerEvents,

  onResetCheckerRunFilters,
  onResetSchedulerEventFilters,
  onSchedulerEventFilterChange,
  onStartMatch,
  onStartScheduler,
  onStopMatch,
  onStopScheduler,
  onUpdateMatchSchedule,
  onUpdateScheduler,
}: GameTabProps): ReactElement {
  const checkerAttentionActive =
    (operationsStatus?.alerts ?? []).some((alert) => alert.source === "checker") ||
    !checkerRunsLiveMode ||
    hasCheckerRunFiltersApplied(filters.checkerRun);

  if (focusPanel === "attacks") {
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
    <div className="grid gap-6">
      <section className="grid gap-3">
        <div>
          <h2 className="text-base font-semibold text-foreground">
            Incident Response
          </h2>
          <p className="text-sm text-muted-foreground">
            Trusted runtime truth first, then the supporting signals that explain
            why reconcile can or cannot be trusted.
          </p>
        </div>
        <RuntimeHealthCard
          accessStatus={accessStatus}
          deploymentRows={deploymentRows}
          operationsStatus={operationsStatus}
          pendingAction={pendingAction}
          serviceMetrics={serviceMetrics}
          wireGuardGatewayStatus={wireGuardGatewayStatus}
          onCopyRuntimeHealthSummary={onCopyRuntimeHealthSummary}
          onDownloadRuntimeHealthReport={onDownloadRuntimeHealthReport}
          onRefreshDeploymentRows={onRefreshDeploymentRows}
          onRefreshOperationsStatus={onRefreshOperationsStatus}
          onRefreshRuntimeHealth={onRefreshRuntimeHealth}
        />
        <OperationsSupportPanel
          operationsStatus={operationsStatus}
          pendingAction={pendingAction}
          serviceMetrics={serviceMetrics}
          onRefreshAlerts={onRefreshOperationsStatus}
          onRefreshMetrics={onRefreshServiceMetrics}
        />
      </section>

      <section className="grid gap-3">
        <div>
          <h2 className="text-base font-semibold text-foreground">
            Match Controls
          </h2>
          <p className="text-sm text-muted-foreground">
            Manual match, schedule, and scoring controls that change live state.
          </p>
        </div>
        <GameControlCard
          gameState={gameState}
          pendingAction={pendingAction}
          scoreRowCount={scoreRows.length}
          onAdvanceTick={onAdvanceTick}
          onRefreshGameStatus={onRefreshGameStatus}
          onRecomputeScores={onRecomputeScores}
          onStartMatch={onStartMatch}
          onStopMatch={onStopMatch}
          onUpdateMatchSchedule={onUpdateMatchSchedule}
        />
      </section>

      <section className="grid gap-3">
        <div>
          <h2 className="text-base font-semibold text-foreground">
            Match Observation
          </h2>
          <p className="text-sm text-muted-foreground">
            Current tick progress and a compact scoreboard snapshot before you
            drill into the full ranking view.
          </p>
        </div>
        <div className="grid items-start gap-4 xl:grid-cols-[0.95fr_1.05fr]">
          <CurrentTickCard currentTick={gameState.current_tick} />
          <ScoreboardSnapshotCard
            pendingAction={pendingAction}
            scoreRows={scoreRows}
            onRefresh={onRefreshGameScoreboard}
          />
        </div>
      </section>

      <section className="grid gap-3">
        <div>
          <h2 className="text-base font-semibold text-foreground">
            Diagnostics
          </h2>
          <p className="text-sm text-muted-foreground">
            Scheduler state and checker evidence stay available, but only expand
            when you need to investigate the runtime path.
          </p>
        </div>
        <div className="grid gap-4">
          <SchedulerCard
            filters={filters.schedulerEvent}
            pendingAction={pendingAction}
            scheduler={gameState.scheduler}
            schedulerEventPage={schedulerEventPage}
            schedulerEventsLiveMode={schedulerEventsLiveMode}
            onApplyFilters={onApplySchedulerEventFilters}
            onFilterChange={onSchedulerEventFilterChange}
            onPage={onPageSchedulerEvents}
            onRefresh={onRefreshSchedulerEvents}
            onResetFilters={onResetSchedulerEventFilters}
            onStartScheduler={onStartScheduler}
            onStopScheduler={onStopScheduler}
            onUpdateScheduler={onUpdateScheduler}
          />
          <details
            className="rounded-md border border-border/70 bg-muted/10 p-3"
            data-testid="checker-investigation-disclosure"
            open={checkerAttentionActive}
          >
            <summary className="cursor-pointer font-medium text-foreground">
              Checker investigation
            </summary>
            <p className="mt-2 text-sm text-muted-foreground">
              Phase-level checker evidence stays available here when runtime
              health or alerts point to checker drift.
            </p>
            <div className="mt-3">
              <CheckerRunsCard
                checkerRunPage={checkerRunPage}
                checkerRunsLiveMode={checkerRunsLiveMode}
                filters={filters.checkerRun}
                pendingAction={pendingAction}
                onApplyFilters={onApplyCheckerRunFilters}
                onFilterChange={onCheckerRunFilterChange}
                onPage={onPageCheckerRuns}
                onRefresh={onRefreshCheckerRuns}
                onResetFilters={onResetCheckerRunFilters}
              />
            </div>
          </details>
        </div>
      </section>
    </div>
  );
}

export function ScoreboardTab({
  pendingAction,
  scoreRows,
  onRefresh,
}: ScoreboardTabProps): ReactElement {
  const [teamFilter, setTeamFilter] = useState("");
  const [sortField, setSortField] = useState<
    "rank" | "team" | "attack" | "defense" | "sla" | "total"
  >("rank");
  const [sortDirection, setSortDirection] = useState<"asc" | "desc">("asc");
  const normalizedFilter = teamFilter.trim().toLowerCase();
  const filteredRows =
    normalizedFilter === ""
      ? scoreRows
      : scoreRows.filter((row) =>
          row.team.toLowerCase().includes(normalizedFilter),
        );
  const sortedRows = [...filteredRows].sort((left, right) => {
    const direction = sortDirection === "asc" ? 1 : -1;

    switch (sortField) {
      case "team":
        return left.team.localeCompare(right.team) * direction;
      case "attack":
        return (left.attack - right.attack) * direction;
      case "defense":
        return (left.defense - right.defense) * direction;
      case "sla":
        return (left.sla - right.sla) * direction;
      case "total":
        return (left.total - right.total) * direction;
      case "rank":
      default:
        return (left.rank - right.rank) * direction;
    }
  });

  return (
    <div className="grid gap-4">
      <Card>
        <CardHeader>
          <CardTitle>Scoreboard View</CardTitle>
          <CardDescription>
            Filter the authoritative ranking by team name without leaving the
            organizer page.
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 md:grid-cols-[minmax(0,1fr)_220px_180px_auto] md:items-end">
          <Field label="Team Filter" htmlFor="scoreboard-team-filter">
            <Input
              id="scoreboard-team-filter"
              placeholder="Search team name"
              value={teamFilter}
              onChange={(event) => setTeamFilter(event.target.value)}
            />
          </Field>
          <Field label="Sort By" htmlFor="scoreboard-sort-field">
            <select
              id="scoreboard-sort-field"
              className={selectClassName}
              value={sortField}
              onChange={(event) =>
                setSortField(
                  event.target.value as
                    | "rank"
                    | "team"
                    | "attack"
                    | "defense"
                    | "sla"
                    | "total",
                )
              }
            >
              <option value="rank">Rank</option>
              <option value="team">Team</option>
              <option value="attack">Attack</option>
              <option value="defense">Defense</option>
              <option value="sla">SLA</option>
              <option value="total">Total</option>
            </select>
          </Field>
          <Field label="Direction" htmlFor="scoreboard-sort-direction">
            <select
              id="scoreboard-sort-direction"
              className={selectClassName}
              value={sortDirection}
              onChange={(event) =>
                setSortDirection(event.target.value as "asc" | "desc")
              }
            >
              <option value="asc">Ascending</option>
              <option value="desc">Descending</option>
            </select>
          </Field>
          <InfoPanel className="h-fit">
            <p className="text-sm text-muted-foreground">
              Showing {sortedRows.length} of {scoreRows.length} team row(s).
            </p>
          </InfoPanel>
        </CardContent>
      </Card>
      <GameScoreboardCard
        pendingAction={pendingAction}
        scoreRows={sortedRows}
        onRefresh={onRefresh}
      />
    </div>
  );
}

function GameControlCard({
  gameState,
  pendingAction,
  scoreRowCount,
  onAdvanceTick,
  onRefreshGameStatus,
  onRecomputeScores,
  onStartMatch,
  onStopMatch,
  onUpdateMatchSchedule,
}: {
  gameState: AdminGameStatus;
  pendingAction: string | null;
  scoreRowCount: number;
  onAdvanceTick: () => void;
  onRefreshGameStatus: () => void;
  onRecomputeScores: () => void;
  onStartMatch: () => void;
  onStopMatch: () => void;
  onUpdateMatchSchedule: (schedule: {
    scheduledStartAt?: string;
    scheduledEndAt?: string;
  }) => void;
}): ReactElement {
  return (
    <div className="grid items-start gap-4 xl:grid-cols-[1.1fr_1fr_0.9fr]">
      <MatchLifecycleCard
        matchState={gameState.match}
        pendingAction={pendingAction}
        onRefreshGameStatus={onRefreshGameStatus}
        onStartMatch={onStartMatch}
        onStopMatch={onStopMatch}
      />
      <MatchScheduleCard
        matchState={gameState.match}
        pendingAction={pendingAction}
        onUpdateMatchSchedule={onUpdateMatchSchedule}
      />
      <QuickActionsCard
        currentTick={gameState.current_tick}
        pendingAction={pendingAction}
        scoreRowCount={scoreRowCount}
        totalCheckerRuns={gameState.total_checker_runs}
        totalTicks={gameState.total_ticks}
        onAdvanceTick={onAdvanceTick}
        onRecomputeScores={onRecomputeScores}
      />
    </div>
  );
}

function MatchLifecycleCard({
  matchState,
  pendingAction,
  onRefreshGameStatus,
  onStartMatch,
  onStopMatch,
}: {
  matchState: AdminGameStatus["match"];
  pendingAction: string | null;
  onRefreshGameStatus: () => void;
  onStartMatch: () => void;
  onStopMatch: () => void;
}): ReactElement {
  return (
    <Card data-testid="match-card">
      <CardHeader>
        <CardTitle>Match</CardTitle>
        <CardDescription>
          Lifecycle state and the primary organizer controls for starting and
          stopping the game.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <InfoPanel layout="grid">
          <InfoLine
            label="State"
            value={matchState?.state ?? "not loaded"}
            valueClassName="font-mono"
          />
          <InfoLine
            label="Submissions"
            value={
              matchState
                ? matchState.accepting_submissions
                  ? "open"
                  : "closed"
                : "unknown"
            }
            valueClassName="font-mono"
          />
          <InfoLine
            label="Started at"
            value={formatIndonesianDate(matchState?.started_at)}
            valueClassName="font-mono"
          />
          <InfoLine
            label="Ended at"
            value={formatIndonesianDate(matchState?.ended_at)}
            valueClassName="font-mono"
          />
        </InfoPanel>
        <CardActionRow>
          <Button
            disabled={pendingAction !== null}
            variant="outline"
            data-testid="refresh-game-status"
            onClick={onRefreshGameStatus}
          >
            {pendingAction === "game:status" ? (
              <LoaderCircle className="h-4 w-4 animate-spin" />
            ) : (
              <RefreshCw className="h-4 w-4" />
            )}
            Refresh Status
          </Button>
          <Button
            disabled={
              pendingAction !== null ||
              matchState?.state === "running" ||
              matchState?.state === "finished"
            }
            variant="outline"
            data-testid="start-match"
            onClick={onStartMatch}
          >
            {pendingAction === "game:match:start" ? (
              <LoaderCircle className="h-4 w-4 animate-spin" />
            ) : (
              <Flag className="h-4 w-4" />
            )}
            Start Game
          </Button>
          <Button
            disabled={pendingAction !== null || matchState?.state !== "running"}
            variant="outline"
            data-testid="stop-match"
            onClick={onStopMatch}
          >
            {pendingAction === "game:match:stop" ? (
              <LoaderCircle className="h-4 w-4 animate-spin" />
            ) : (
              <Flag className="h-4 w-4" />
            )}
            Stop Match
          </Button>
        </CardActionRow>
      </CardContent>
    </Card>
  );
}

function MatchScheduleCard({
  matchState,
  pendingAction,
  onUpdateMatchSchedule,
}: {
  matchState: AdminGameStatus["match"];
  pendingAction: string | null;
  onUpdateMatchSchedule: (schedule: {
    scheduledStartAt?: string;
    scheduledEndAt?: string;
  }) => void;
}): ReactElement {
  const [draftScheduledStart, setDraftScheduledStart] = useState("");
  const [draftScheduledEnd, setDraftScheduledEnd] = useState("");

  useEffect(() => {
    setDraftScheduledStart(toDateTimeLocalValue(matchState?.scheduled_start_at));
    setDraftScheduledEnd(toDateTimeLocalValue(matchState?.scheduled_end_at));
  }, [matchState?.scheduled_end_at, matchState?.scheduled_start_at]);

  const scheduleChanged =
    draftScheduledStart !== toDateTimeLocalValue(matchState?.scheduled_start_at) ||
    draftScheduledEnd !== toDateTimeLocalValue(matchState?.scheduled_end_at);
  const scheduleRangeInvalid =
    draftScheduledStart !== "" &&
    draftScheduledEnd !== "" &&
    draftScheduledEnd < draftScheduledStart;

  return (
    <Card>
      <CardHeader>
        <CardTitle>Schedule Window</CardTitle>
        <CardDescription>
          Configure the planned start and end timestamps used by game-core.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <InfoPanel layout="grid">
          <InfoLine
            label="Scheduled start"
            value={formatIndonesianDate(matchState?.scheduled_start_at)}
            valueClassName="font-mono"
          />
          <InfoLine
            label="Scheduled end"
            value={formatIndonesianDate(matchState?.scheduled_end_at)}
            valueClassName="font-mono"
          />
        </InfoPanel>
        <div className="grid gap-3 rounded-md border border-border/70 bg-background p-3">
          <div className="grid gap-3 lg:grid-cols-2">
            <div className="grid gap-3">
              <Field
                label="Scheduled Start Date"
                htmlFor="match-scheduled-start-date"
              >
                <Input
                  id="match-scheduled-start-date"
                  type="date"
                  value={datePartFromDateTimeLocalValue(draftScheduledStart)}
                  onChange={(event) =>
                    setDraftScheduledStart(
                      withUpdatedDatePart(
                        draftScheduledStart,
                        event.target.value,
                      ),
                    )
                  }
                />
              </Field>
              <Field
                label="Scheduled Start Time"
                htmlFor="match-scheduled-start-time"
              >
                <Input
                  id="match-scheduled-start-time"
                  type="time"
                  step={60}
                  value={timePartFromDateTimeLocalValue(draftScheduledStart)}
                  onChange={(event) =>
                    setDraftScheduledStart(
                      withUpdatedTimePart(
                        draftScheduledStart,
                        event.target.value,
                      ),
                    )
                  }
                />
              </Field>
            </div>
            <div className="grid gap-3">
              <Field
                label="Scheduled End Date"
                htmlFor="match-scheduled-end-date"
              >
                <Input
                  id="match-scheduled-end-date"
                  type="date"
                  value={datePartFromDateTimeLocalValue(draftScheduledEnd)}
                  onChange={(event) =>
                    setDraftScheduledEnd(
                      withUpdatedDatePart(draftScheduledEnd, event.target.value),
                    )
                  }
                />
              </Field>
              <Field
                label="Scheduled End Time"
                htmlFor="match-scheduled-end-time"
              >
                <Input
                  id="match-scheduled-end-time"
                  type="time"
                  step={60}
                  value={timePartFromDateTimeLocalValue(draftScheduledEnd)}
                  onChange={(event) =>
                    setDraftScheduledEnd(
                      withUpdatedTimePart(draftScheduledEnd, event.target.value),
                    )
                  }
                />
              </Field>
            </div>
          </div>
          <CardActionRow>
            <Button
              disabled={
                pendingAction !== null || !scheduleChanged || scheduleRangeInvalid
              }
              variant="outline"
              onClick={() =>
                onUpdateMatchSchedule({
                  scheduledStartAt: fromDateTimeLocalValue(draftScheduledStart),
                  scheduledEndAt: fromDateTimeLocalValue(draftScheduledEnd),
                })
              }
            >
              {pendingAction === "game:match:schedule" ? (
                <LoaderCircle className="h-4 w-4 animate-spin" />
              ) : (
                <RefreshCw className="h-4 w-4" />
              )}
              Save Window
            </Button>
            <Button
              disabled={
                pendingAction !== null ||
                (draftScheduledStart === "" && draftScheduledEnd === "")
              }
              variant="ghost"
              onClick={() => {
                setDraftScheduledStart("");
                setDraftScheduledEnd("");
                onUpdateMatchSchedule({});
              }}
            >
              Clear Window
            </Button>
          </CardActionRow>
          {scheduleRangeInvalid ? (
            <p className="text-danger text-sm">
              Scheduled end must not be earlier than scheduled start.
            </p>
          ) : null}
        </div>
      </CardContent>
    </Card>
  );
}

function QuickActionsCard({
  currentTick,
  pendingAction,
  scoreRowCount,
  totalCheckerRuns,
  totalTicks,
  onAdvanceTick,
  onRecomputeScores,
}: {
  currentTick?: AdminGameTickStatus;
  pendingAction: string | null;
  scoreRowCount: number;
  totalCheckerRuns: number;
  totalTicks: number;
  onAdvanceTick: () => void;
  onRecomputeScores: () => void;
}): ReactElement {
  return (
    <Card data-testid="quick-actions-card">
      <CardHeader>
        <CardTitle>Quick Actions</CardTitle>
        <CardDescription>
          Manual operations for advancing the game or recalculating rankings.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <InfoPanel layout="grid">
          <InfoLine
            label="Total ticks"
            value={totalTicks}
            valueClassName="font-mono"
          />
          <InfoLine
            label="Checker runs"
            value={totalCheckerRuns}
            valueClassName="font-mono"
          />
          <InfoLine
            label="Scoreboard rows"
            value={scoreRowCount}
            valueClassName="font-mono"
          />
          <InfoLine
            label="Latest tick"
            value={
              currentTick ? `#${currentTick.id} ${currentTick.status}` : "not started yet"
            }
            valueClassName="font-mono"
          />
        </InfoPanel>
        <CardActionRow>
          <Button
            disabled={pendingAction !== null}
            data-testid="advance-tick"
            onClick={onAdvanceTick}
          >
            {pendingAction === "game:advance" ? (
              <LoaderCircle className="h-4 w-4 animate-spin" />
            ) : (
              <Flag className="h-4 w-4" />
            )}
            Advance Tick
          </Button>
          <Button
            disabled={pendingAction !== null}
            variant="outline"
            data-testid="recompute-scores"
            onClick={onRecomputeScores}
          >
            {pendingAction === "game:scoring" ? (
              <LoaderCircle className="h-4 w-4 animate-spin" />
            ) : (
              <RefreshCw className="h-4 w-4" />
            )}
            Recompute Scores
          </Button>
        </CardActionRow>
      </CardContent>
    </Card>
  );
}

type RuntimeHealthSeverity = "healthy" | "warning" | "critical";

type RuntimeHealthSnapshot = {
  accessLabel: string;
  deploymentFailedCount: number;
  deploymentPendingCount: number;
  generatedAt: string;
  metricsAttentionCount: number;
  operationsCriticalCount: number;
  operationsWarningCount: number;
  severity: RuntimeHealthSeverity;
  summary: string;
  wireGuardLabel: string;
  warnings: string[];
};

function RuntimeHealthCard({
  accessStatus,
  deploymentRows,
  onCopyRuntimeHealthSummary,
  onDownloadRuntimeHealthReport,
  operationsStatus,
  pendingAction,
  serviceMetrics,
  wireGuardGatewayStatus,
  onRefreshDeploymentRows,
  onRefreshOperationsStatus,
  onRefreshRuntimeHealth,
}: {
  accessStatus: AdminControllerAccessStatus | null;
  deploymentRows: AdminDeploymentJob[];
  onCopyRuntimeHealthSummary: () => void;
  onDownloadRuntimeHealthReport: () => void;
  operationsStatus: AdminOperationsStatus | null;
  pendingAction: string | null;
  serviceMetrics: AdminServiceMetricSnapshot | null;
  wireGuardGatewayStatus: AdminWireGuardGatewayStatus | null;
  onRefreshDeploymentRows: () => void;
  onRefreshOperationsStatus: () => void;
  onRefreshRuntimeHealth: () => void;
}): ReactElement {
  const snapshot = buildRuntimeHealthSnapshot({
    accessStatus,
    deploymentRows,
    operationsStatus,
    serviceMetrics,
    wireGuardGatewayStatus,
  });

  return (
    <AdminRuntimeCard
      title="Runtime Health"
      description="Aggregated drift view for trusted reconcile, controller access, WireGuard, deployment backlog, and the latest runtime alerts."
    >
      <div
        className="space-y-4 p-4"
        data-testid="runtime-health-card"
        id="runtime-health-card"
      >
        <StatusBanner
          message={snapshot.summary}
          variant={
            snapshot.severity === "healthy"
              ? "success"
              : snapshot.severity === "critical"
                ? "error"
                : "warning"
          }
        />
        <InfoPanel layout="grid" tone="surface">
          <InfoLine
            label="Runtime trust"
            value={runtimeHealthSeverityLabel(snapshot.severity)}
            valueClassName="font-mono"
          />
          <InfoLine
            label="Deployments"
            value={`${snapshot.deploymentPendingCount} pending / ${snapshot.deploymentFailedCount} failed`}
            valueClassName="font-mono"
          />
          <InfoLine
            label="Access policy"
            value={snapshot.accessLabel}
            valueClassName="font-mono"
          />
          <InfoLine
            label="WireGuard"
            value={snapshot.wireGuardLabel}
            valueClassName="font-mono"
          />
        </InfoPanel>
        {snapshot.warnings.length > 0 ? (
          <RuntimeRemediationPanel
            pendingAction={pendingAction}
            severity={snapshot.severity}
            warnings={snapshot.warnings}
            onCopyRuntimeHealthSummary={onCopyRuntimeHealthSummary}
            onDownloadRuntimeHealthReport={onDownloadRuntimeHealthReport}
            onRefreshRuntimeHealth={onRefreshRuntimeHealth}
          />
        ) : null}
        {snapshot.warnings.length === 0 ? (
          <div className="flex flex-col gap-3 rounded-md border border-border/70 bg-background p-3 md:flex-row md:items-center md:justify-between">
            <RuntimeActionButton
              actionID="runtime:health"
              label="Refresh Runtime"
              pendingAction={pendingAction}
              testID="refresh-runtime-health"
              onClick={onRefreshRuntimeHealth}
            />
            <div className="flex flex-wrap items-center gap-2">
              <EvidenceActions
                pendingAction={pendingAction}
                onCopyRuntimeHealthSummary={onCopyRuntimeHealthSummary}
                onDownloadRuntimeHealthReport={onDownloadRuntimeHealthReport}
              />
            </div>
          </div>
        ) : null}
        <details className="rounded-md border border-border/70 bg-muted/10 p-3 text-sm">
          <summary className="cursor-pointer font-medium text-foreground">
            Inspect trust inputs
          </summary>
          <InfoPanel className="mt-3" compact layout="grid" tone="surface">
            <InfoLine
              label="Active alerts"
              value={`${snapshot.operationsCriticalCount} critical / ${snapshot.operationsWarningCount} warning`}
              valueClassName="font-mono"
            />
            <InfoLine
              label="Metrics needing attention"
              value={String(snapshot.metricsAttentionCount)}
              valueClassName="font-mono"
            />
            <InfoLine
              label="Snapshot"
              value={snapshot.generatedAt}
              valueClassName="font-mono"
            />
          </InfoPanel>
          <CardActionRow className="mt-3">
            <Button
              size="sm"
              variant="outline"
              data-testid="refresh-runtime-alerts"
              disabled={pendingAction !== null}
              onClick={onRefreshOperationsStatus}
            >
              {pendingAction === "operations:status" ? (
                <LoaderCircle className="h-4 w-4 animate-spin" />
              ) : (
                <RefreshCw className="h-4 w-4" />
              )}
              Refresh Alerts
            </Button>
            <Button
              size="sm"
              variant="outline"
              data-testid="refresh-runtime-deployments"
              disabled={pendingAction !== null}
              onClick={onRefreshDeploymentRows}
            >
              {pendingAction === "deployments:list" ? (
                <LoaderCircle className="h-4 w-4 animate-spin" />
              ) : (
                <RefreshCw className="h-4 w-4" />
              )}
              Refresh Jobs
            </Button>
            <Button asChild size="sm" variant="ghost">
              <Link href="/admin/deployments">Open Deployments</Link>
            </Button>
            <Button asChild size="sm" variant="ghost">
              <Link href="/admin/players">Open Access Controls</Link>
            </Button>
          </CardActionRow>
        </details>
      </div>
    </AdminRuntimeCard>
  );
}

function RuntimeRemediationPanel({
  pendingAction,
  severity,
  warnings,
  onCopyRuntimeHealthSummary,
  onDownloadRuntimeHealthReport,
  onRefreshRuntimeHealth,
}: {
  pendingAction: string | null;
  severity: RuntimeHealthSeverity;
  warnings: string[];
  onCopyRuntimeHealthSummary: () => void;
  onDownloadRuntimeHealthReport: () => void;
  onRefreshRuntimeHealth: () => void;
}): ReactElement {
  const nextActionSummary =
    severity === "critical"
      ? "Inspect scheduler and checker state after runtime truth is refreshed."
      : "Inspect deployment and access truth before trusting SSH or runtime state.";

  return (
    <div
      className={cn(
        "rounded-md border p-3",
        severity === "critical" ? "tone-danger" : "tone-warning",
      )}
    >
      <p className="text-sm font-medium text-foreground">Incident workflow</p>
      <ol className="mt-2 grid gap-2 text-sm text-muted-foreground md:grid-cols-3">
        <li>
          <span className="font-medium text-foreground">1. Refresh truth:</span>{" "}
          reload deployments, access, gateway, alerts, and metrics together.
        </li>
        <li>
          <span className="font-medium text-foreground">2. Inspect drift:</span>{" "}
          use the warning list below to open the failing runtime area.
        </li>
        <li>
          <span className="font-medium text-foreground">3. Capture evidence:</span>{" "}
          copy the summary or download the full report before state changes.
        </li>
      </ol>
      <p className="mt-3 text-sm text-muted-foreground">{nextActionSummary}</p>
      <CardActionRow>
        <RuntimeActionButton
          actionID="runtime:health"
          label="Refresh Runtime"
          pendingAction={pendingAction}
          testID="incident-refresh-runtime"
          onClick={onRefreshRuntimeHealth}
        />
        <EvidenceActions
          pendingAction={pendingAction}
          onCopyRuntimeHealthSummary={onCopyRuntimeHealthSummary}
          onDownloadRuntimeHealthReport={onDownloadRuntimeHealthReport}
        />
        <Button asChild size="sm" variant="ghost">
          <Link
            href={
              severity === "critical"
                ? "/admin/game#scheduler-card"
                : "/admin/deployments"
            }
          >
            {severity === "critical"
              ? "Inspect Scheduler"
              : "Inspect Deployments"}
          </Link>
        </Button>
        <Button asChild size="sm" variant="ghost">
          <Link
            href={
              severity === "critical"
                ? "/admin/game#checker-runs-card"
                : "/admin/players"
            }
          >
            {severity === "critical"
              ? "Inspect Checker Runs"
              : "Inspect Access Controls"}
          </Link>
        </Button>
      </CardActionRow>
      <ul className="mt-3 grid gap-2 text-sm text-muted-foreground">
        {warnings.map((warning) => (
          <li key={warning}>{warning}</li>
        ))}
      </ul>
    </div>
  );
}

function EvidenceActions({
  pendingAction,
  onCopyRuntimeHealthSummary,
  onDownloadRuntimeHealthReport,
}: {
  pendingAction: string | null;
  onCopyRuntimeHealthSummary: () => void;
  onDownloadRuntimeHealthReport: () => void;
}): ReactElement {
  return (
    <>
      <Button
        size="sm"
        variant="outline"
        data-testid="copy-runtime-health-summary"
        disabled={pendingAction !== null}
        onClick={onCopyRuntimeHealthSummary}
      >
        {pendingAction === "runtime:summary" ? (
          <LoaderCircle className="h-4 w-4 animate-spin" />
        ) : (
          <Copy className="h-4 w-4" />
        )}
        Copy Summary
      </Button>
      <Button
        size="sm"
        variant="outline"
        data-testid="download-runtime-health-report"
        disabled={pendingAction !== null}
        onClick={onDownloadRuntimeHealthReport}
      >
        {pendingAction === "runtime:report" ? (
          <LoaderCircle className="h-4 w-4 animate-spin" />
        ) : (
          <Download className="h-4 w-4" />
        )}
        Download Evidence
      </Button>
    </>
  );
}

function buildRuntimeHealthSnapshot({
  accessStatus,
  deploymentRows,
  operationsStatus,
  serviceMetrics,
  wireGuardGatewayStatus,
}: {
  accessStatus: AdminControllerAccessStatus | null;
  deploymentRows: AdminDeploymentJob[];
  operationsStatus: AdminOperationsStatus | null;
  serviceMetrics: AdminServiceMetricSnapshot | null;
  wireGuardGatewayStatus: AdminWireGuardGatewayStatus | null;
}): RuntimeHealthSnapshot {
  const deploymentPendingCount = deploymentRows.filter(
    (deployment) => !["completed", "failed", "superseded"].includes(deployment.status),
  ).length;
  const deploymentFailedCount = deploymentRows.filter(
    (deployment) =>
      deployment.status === "failed" || deployment.failed_team_count > 0,
  ).length;
  const operationsAlerts = operationsStatus?.alerts ?? [];
  const operationsCriticalCount = operationsAlerts.filter(
    (alert) => alert.severity === "critical",
  ).length;
  const operationsWarningCount = operationsAlerts.length - operationsCriticalCount;
  const metricsWarnings: string[] = [];

  if (serviceMetrics === null) {
    metricsWarnings.push("Live service metrics snapshot is unavailable.");
  } else {
    if (serviceMetrics.controller_service.access_last_apply_success === false) {
      metricsWarnings.push(
        "Controller metrics report the last access apply was unsuccessful.",
      );
    }
    if (serviceMetrics.wireguard_gateway.last_apply_success === false) {
      metricsWarnings.push(
        "WireGuard metrics report the last gateway apply was unsuccessful.",
      );
    }
    if (serviceMetrics.realtime_gateway.last_sync_success === false) {
      metricsWarnings.push(
        "Realtime metrics report the last snapshot sync was unsuccessful.",
      );
    }
  }

  const warnings: string[] = [];
  if (deploymentPendingCount > 0) {
    warnings.push(
      `${deploymentPendingCount} deployment job(s) still need trusted reconcile completion.`,
    );
  }
  if (deploymentFailedCount > 0) {
    warnings.push(
      `${deploymentFailedCount} deployment job(s) have failed team runtime work recorded.`,
    );
  }
  if (accessStatus && accessStatus.state !== "applied") {
    warnings.push(
      `Controller access policy is ${accessStatus.state}, so SSH truth may be stale.`,
    );
  } else if (!accessStatus) {
    warnings.push("Controller access policy status is not loaded yet.");
  }
  if (wireGuardGatewayStatus && wireGuardGatewayStatus.state !== "applied") {
    warnings.push(
      `WireGuard gateway is ${wireGuardGatewayStatus.state}, so peer truth may be stale.`,
    );
  } else if (!wireGuardGatewayStatus) {
    warnings.push("WireGuard gateway status is not loaded yet.");
  }
  if (operationsCriticalCount > 0) {
    warnings.push(`${operationsCriticalCount} critical runtime alert(s) are active.`);
  }
  if (operationsWarningCount > 0) {
    warnings.push(`${operationsWarningCount} warning runtime alert(s) are active.`);
  }
  warnings.push(...metricsWarnings);

  const severity: RuntimeHealthSeverity =
    operationsCriticalCount > 0 ||
    deploymentFailedCount > 0 ||
    accessStatus?.state === "error" ||
    wireGuardGatewayStatus?.state === "error"
      ? "critical"
      : warnings.length > 0
        ? "warning"
        : "healthy";

  return {
    accessLabel: accessStatus
      ? `${accessStatus.state} (${accessStatus.mode})`
      : "not loaded",
    deploymentFailedCount,
    deploymentPendingCount,
    generatedAt: formatRuntimeHealthTimestamp(
      operationsStatus?.generated_at ??
        accessStatus?.applied_at ??
        wireGuardGatewayStatus?.applied_at,
    ),
    metricsAttentionCount: metricsWarnings.length,
    operationsCriticalCount,
    operationsWarningCount,
    severity,
    summary:
      severity === "healthy"
        ? "Trusted reconcile, controller access, WireGuard, deployments, and live metrics currently agree."
        : severity === "critical"
          ? "Runtime truth is degraded. Reconcile or inspect the failing layer before trusting SSH access or deployment state."
          : "Runtime drift warnings are active. Review deployments, access, and gateway state before assuming the stack is converged.",
    wireGuardLabel: wireGuardGatewayStatus
      ? `${wireGuardGatewayStatus.state} (${wireGuardGatewayStatus.mode})`
      : "not loaded",
    warnings,
  };
}

function runtimeHealthSeverityLabel(severity: RuntimeHealthSeverity): string {
  switch (severity) {
    case "critical":
      return "critical";
    case "warning":
      return "attention";
    default:
      return "healthy";
  }
}

function formatRuntimeHealthTimestamp(value?: string): string {
  return formatIndonesianDate(value) || "n/a";
}

function OperationsSupportPanel({
  operationsStatus,
  pendingAction,
  serviceMetrics,
  onRefreshAlerts,
  onRefreshMetrics,
}: {
  operationsStatus: AdminOperationsStatus | null;
  pendingAction: string | null;
  serviceMetrics: AdminServiceMetricSnapshot | null;
  onRefreshAlerts: () => void;
  onRefreshMetrics: () => void;
}): ReactElement {
  return (
    <div className="grid gap-4">
      <div className="mb-4 flex flex-col gap-1">
        <h3 className="text-sm font-semibold text-foreground">
          Operational Signals
        </h3>
        <p className="text-sm text-muted-foreground">
          Alert pressure and live service health in one operator band.
        </p>
      </div>
      <div className="grid gap-4 xl:grid-cols-[0.9fr_1.1fr]">
        <OperationsAlertsCard
          operationsStatus={operationsStatus}
          pendingAction={pendingAction}
          onRefresh={onRefreshAlerts}
        />
        <OperationsMetricsCard
          pendingAction={pendingAction}
          serviceMetrics={serviceMetrics}
          onRefresh={onRefreshMetrics}
        />
      </div>
    </div>
  );
}

function OperationsAlertsCard({
  operationsStatus,
  pendingAction,
  onRefresh,
}: {
  operationsStatus: AdminOperationsStatus | null;
  pendingAction: string | null;
  onRefresh: () => void;
}): ReactElement {
  const alerts = operationsStatus?.alerts ?? [];

  return (
    <section
      data-testid="operations-alerts-card"
      className="rounded-md border border-border/50 bg-muted/5 p-4"
    >
      <div className="flex items-start justify-between gap-3">
        <div>
          <h4 className="text-sm font-semibold text-foreground">
            Runtime Alerts
          </h4>
          <p className="text-sm text-muted-foreground">
            Scheduler, checker, deployment, gateway, and access warnings.
          </p>
        </div>
        <Button
          disabled={pendingAction !== null}
          size="sm"
          variant="outline"
          onClick={onRefresh}
        >
          {pendingAction === "operations:status" ? (
            <LoaderCircle className="h-4 w-4 animate-spin" />
          ) : (
            <RefreshCw className="h-4 w-4" />
          )}
          Refresh
        </Button>
      </div>
      <div className="mt-4">
        {alerts.length === 0 ? (
          <StatusBanner
            message="No runtime alerts are currently active."
            variant="success"
          />
        ) : (
          <div className="space-y-3">
            {alerts.map((alert) => {
              const action = operationsAlertAction(alert);
              return (
                <div
                  key={alert.id}
                  className="rounded-md border border-border/70 bg-background p-3"
                >
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div className="flex flex-wrap items-center gap-2">
                      <Badge
                        variant="outline"
                        className={
                          alert.severity === "critical"
                            ? "tone-danger"
                            : "tone-warning"
                        }
                      >
                        {alert.severity}
                      </Badge>
                      <Badge variant="secondary">{alert.source}</Badge>
                    </div>
                    {action ? (
                      <Button asChild size="sm" variant="ghost">
                        <Link href={action.href}>{action.label}</Link>
                      </Button>
                    ) : null}
                  </div>
                  <p className="mt-2 text-sm font-medium text-foreground">
                    {alert.summary}
                  </p>
                  {alert.detail ? (
                    <p className="mt-1 text-sm text-muted-foreground">
                      {alert.detail}
                    </p>
                  ) : null}
                </div>
              );
            })}
          </div>
        )}
      </div>
    </section>
  );
}

function OperationsMetricsCard({
  pendingAction,
  serviceMetrics,
  onRefresh,
}: {
  pendingAction: string | null;
  serviceMetrics: AdminServiceMetricSnapshot | null;
  onRefresh: () => void;
}): ReactElement {
  const serviceHealth = serviceMetrics
    ? buildServiceHealthSummaries(serviceMetrics)
    : [];
  const unhealthyCount = serviceHealth.filter(
    (entry) => entry.status !== "healthy",
  ).length;

  return (
    <section
      data-testid="operations-metrics-card"
      className="rounded-md border border-border/50 bg-muted/5 p-4"
    >
      <div className="flex items-start justify-between gap-3">
        <div>
          <h4 className="text-sm font-semibold text-foreground">
            Service Metrics
          </h4>
          <p className="text-sm text-muted-foreground">
            Live counters from game-core, submission, controller, realtime, and
            WireGuard.
          </p>
        </div>
        <Button
          disabled={pendingAction !== null}
          size="sm"
          variant="outline"
          onClick={onRefresh}
        >
          {pendingAction === "operations:metrics" ? (
            <LoaderCircle className="h-4 w-4 animate-spin" />
          ) : (
            <RefreshCw className="h-4 w-4" />
          )}
          Refresh
        </Button>
      </div>
      <div className="mt-4 space-y-4">
        {serviceMetrics === null ? (
          <StatusBanner
            message="Live service metrics are currently unavailable."
            variant="warning"
          />
        ) : (
          <>
            <div className="grid gap-3">
              <StatusBanner
                message={
                  unhealthyCount === 0
                    ? "All tracked service metrics currently look healthy."
                    : `${unhealthyCount} service area(s) need operator attention.`
                }
                variant={unhealthyCount === 0 ? "success" : "warning"}
              />
              <div className="rounded-md border border-border/70 bg-background p-3">
                <div className="flex flex-wrap gap-2">
                  {serviceHealth.map((entry) => (
                    <Badge
                      key={entry.title}
                      variant="outline"
                      className={entry.statusClassName}
                    >
                      {entry.title}: {entry.statusLabel}
                    </Badge>
                  ))}
                </div>
              </div>
            </div>
            <InfoPanel tone="surface">
              <p className="text-sm text-muted-foreground">
                Metrics snapshot generated at{" "}
                <span className="font-mono text-foreground">
                  {formatIndonesianDate(serviceMetrics.generated_at)}
                </span>
                .
              </p>
            </InfoPanel>
            <div className="grid gap-3 lg:grid-cols-[0.9fr_1.1fr]">
              <InfoPanel layout="grid" tone="surface">
                <InfoLine
                  label="Checker runs"
                  value={formatMetricNumber(
                    serviceMetrics.game_core.checker_runs_total,
                  )}
                  valueClassName="font-mono"
                />
                <InfoLine
                  label="Submit requests"
                  value={formatMetricNumber(
                    serviceMetrics.submission_service.submit_requests_total,
                  )}
                  valueClassName="font-mono"
                />
                <InfoLine
                  label="Active peers"
                  value={formatMetricNumber(
                    serviceMetrics.wireguard_gateway.peers_active,
                  )}
                  valueClassName="font-mono"
                />
                <InfoLine
                  label="Sync errors"
                  value={formatMetricNumber(
                    serviceMetrics.realtime_gateway.sync_errors_total,
                  )}
                  valueClassName="font-mono"
                />
              </InfoPanel>
              <div className="rounded-md border border-border/70 bg-background p-3">
                <p className="text-sm font-medium text-foreground">
                  Service watchlist
                </p>
                <div className="mt-2 grid gap-2 text-sm text-muted-foreground">
                  {serviceHealth.map((entry) => (
                    <p key={`${entry.title}-detail`}>
                      <span className="font-medium text-foreground">
                        {entry.title}
                      </span>
                      : {entry.detail}
                    </p>
                  ))}
                </div>
              </div>
            </div>
            <details className="rounded-md border border-border/70 bg-background p-3">
              <summary className="cursor-pointer text-sm font-medium text-foreground">
                Inspect per-service metrics
              </summary>
              <div className="mt-4 grid gap-4 xl:grid-cols-[1fr_1fr] 2xl:grid-cols-[1fr_1fr_1fr]">
                <MetricsServicePanel
                  title="Game Core"
                  health={serviceHealth[0]}
                  lines={[
                    {
                      label: "Match state",
                      value: serviceMetrics.game_core.match_state,
                    },
                    {
                      label: "Total ticks",
                      value: formatMetricNumber(serviceMetrics.game_core.total_ticks),
                    },
                    {
                      label: "Checker runs",
                      value: formatMetricNumber(
                        serviceMetrics.game_core.checker_runs_total,
                      ),
                    },
                    {
                      label: "Checker failures",
                      value: formatMetricNumber(
                        serviceMetrics.game_core.checker_runs_failed,
                      ),
                    },
                    {
                      label: "Scheduler running",
                      value: formatMetricBool(
                        serviceMetrics.game_core.scheduler_running,
                      ),
                    },
                  ]}
                />
                <MetricsServicePanel
                  title="Submission Service"
                  health={serviceHealth[1]}
                  lines={[
                    {
                      label: "Submit requests",
                      value: formatMetricNumber(
                        serviceMetrics.submission_service.submit_requests_total,
                      ),
                    },
                    {
                      label: "Submit failures",
                      value: formatMetricNumber(
                        serviceMetrics.submission_service.submit_failures_total,
                      ),
                    },
                    {
                      label: "Attack-feed requests",
                      value: formatMetricNumber(
                        serviceMetrics.submission_service.attack_feed_requests_total,
                      ),
                    },
                    {
                      label: "Correct verdicts",
                      value: formatMetricNumber(
                        serviceMetrics.submission_service.verdicts.correct,
                      ),
                    },
                    {
                      label: "Invalid verdicts",
                      value: formatMetricNumber(
                        serviceMetrics.submission_service.verdicts.invalid,
                      ),
                    },
                  ]}
                />
                <MetricsServicePanel
                  title="Controller Service"
                  health={serviceHealth[2]}
                  lines={[
                    {
                      label: "Deployment reconciles",
                      value: formatMetricNumber(
                        serviceMetrics.controller_service
                          .deployment_reconcile_requests,
                      ),
                    },
                    {
                      label: "Access reconciles",
                      value: formatMetricNumber(
                        serviceMetrics.controller_service.access_reconcile_requests,
                      ),
                    },
                    {
                      label: "Service access reconciles",
                      value: formatMetricNumber(
                        serviceMetrics.controller_service
                          .service_access_reconcile_requests,
                      ),
                    },
                    {
                      label: "SSH credentials",
                      value: formatMetricNumber(
                        serviceMetrics.controller_service.ssh_credential_requests,
                      ),
                    },
                    {
                      label: "Last apply success",
                      value: formatMetricBool(
                        serviceMetrics.controller_service.access_last_apply_success,
                      ),
                    },
                  ]}
                />
                <MetricsServicePanel
                  title="Realtime Gateway"
                  health={serviceHealth[3]}
                  lines={[
                    {
                      label: "Last sync success",
                      value: formatMetricBool(
                        serviceMetrics.realtime_gateway.last_sync_success,
                      ),
                    },
                    {
                      label: "Sync errors",
                      value: formatMetricNumber(
                        serviceMetrics.realtime_gateway.sync_errors_total,
                      ),
                    },
                    {
                      label: "Subscribers",
                      value: formatMetricNumber(
                        serviceMetrics.realtime_gateway.subscribers_total,
                      ),
                    },
                    {
                      label: "Snapshot bytes",
                      value: formatMetricNumber(
                        serviceMetrics.realtime_gateway.snapshot_bytes_total,
                      ),
                    },
                  ]}
                />
                <MetricsServicePanel
                  title="WireGuard Gateway"
                  health={serviceHealth[4]}
                  lines={[
                    {
                      label: "Reconcile requests",
                      value: formatMetricNumber(
                        serviceMetrics.wireguard_gateway.reconcile_requests,
                      ),
                    },
                    {
                      label: "Total peers",
                      value: formatMetricNumber(
                        serviceMetrics.wireguard_gateway.peers_total,
                      ),
                    },
                    {
                      label: "Active peers",
                      value: formatMetricNumber(
                        serviceMetrics.wireguard_gateway.peers_active,
                      ),
                    },
                    {
                      label: "Revoked peers",
                      value: formatMetricNumber(
                        serviceMetrics.wireguard_gateway.peers_revoked,
                      ),
                    },
                    {
                      label: "Last apply success",
                      value: formatMetricBool(
                        serviceMetrics.wireguard_gateway.last_apply_success,
                      ),
                    },
                  ]}
                />
              </div>
            </details>
          </>
        )}
      </div>
    </section>
  );
}

function MetricsServicePanel({
  title,
  health,
  lines,
}: {
  title: string;
  health?: ServiceHealthSummary;
  lines: { label: string; value: string }[];
}): ReactElement {
  return (
    <div className="rounded-md border border-border/70 bg-muted/20 p-4">
      <div className="flex items-start justify-between gap-3">
        <h3 className="text-sm font-semibold text-foreground">{title}</h3>
        {health ? (
          <Badge variant="outline" className={health.statusClassName}>
            {health.statusLabel}
          </Badge>
        ) : null}
      </div>
      {health?.detail ? (
        <p className="mt-2 text-xs leading-5 text-muted-foreground">
          {health.detail}
        </p>
      ) : null}
      <div className="mt-3 grid gap-2">
        {lines.map((line) => (
          <div
            key={`${title}-${line.label}`}
            className="flex items-center justify-between gap-3 text-sm"
          >
            <span className="text-muted-foreground">{line.label}</span>
            <span className="font-mono text-foreground">{line.value}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

function formatMetricNumber(value: number | null): string {
  if (value === null) {
    return "n/a";
  }
  return Number.isInteger(value) ? String(value) : value.toFixed(2);
}

function formatMetricBool(value: boolean | null): string {
  if (value === null) {
    return "n/a";
  }
  return value ? "yes" : "no";
}

type ServiceHealthSummary = {
  title: string;
  status: "healthy" | "warning";
  statusLabel: string;
  statusClassName: string;
  detail: string;
};

function buildServiceHealthSummaries(
  metrics: AdminServiceMetricSnapshot,
): ServiceHealthSummary[] {
  const gameCoreWarnings: string[] = [];
  if (
    metrics.game_core.match_state === "running" &&
    metrics.game_core.scheduler_running === false
  ) {
    gameCoreWarnings.push("scheduler stopped");
  }
  if ((metrics.game_core.checker_runs_failed ?? 0) > 0) {
    gameCoreWarnings.push(
      `${formatMetricNumber(metrics.game_core.checker_runs_failed)} checker failures`,
    );
  }

  const submissionWarnings: string[] = [];
  if ((metrics.submission_service.submit_failures_total ?? 0) > 0) {
    submissionWarnings.push(
      `${formatMetricNumber(metrics.submission_service.submit_failures_total)} submit failures`,
    );
  }

  const controllerWarnings: string[] = [];
  if (metrics.controller_service.access_last_apply_success === false) {
    controllerWarnings.push("access apply not successful");
  }

  const realtimeWarnings: string[] = [];
  if (metrics.realtime_gateway.last_sync_success === false) {
    realtimeWarnings.push("last sync failed");
  }
  if ((metrics.realtime_gateway.sync_errors_total ?? 0) > 0) {
    realtimeWarnings.push(
      `${formatMetricNumber(metrics.realtime_gateway.sync_errors_total)} sync errors`,
    );
  }

  const wireguardWarnings: string[] = [];
  if (metrics.wireguard_gateway.last_apply_success === false) {
    wireguardWarnings.push("last apply failed");
  }

  return [
    summarizeServiceHealth("Game Core", gameCoreWarnings),
    summarizeServiceHealth("Submission", submissionWarnings),
    summarizeServiceHealth("Controller", controllerWarnings),
    summarizeServiceHealth("Realtime", realtimeWarnings),
    summarizeServiceHealth("WireGuard", wireguardWarnings),
  ];
}

function summarizeServiceHealth(
  title: string,
  warnings: string[],
): ServiceHealthSummary {
  if (warnings.length === 0) {
    return {
      title,
      status: "healthy",
      statusLabel: "healthy",
      statusClassName: "tone-success",
      detail: "No obvious issues in the current metrics snapshot.",
    };
  }

  return {
    title,
    status: "warning",
    statusLabel: "attention",
    statusClassName: "tone-warning",
    detail: warnings.join(", "),
  };
}

function operationsAlertAction(alert: AdminOperationsStatus["alerts"][number]):
  | { href: string; label: string }
  | null {
  switch (alert.source) {
    case "scheduler":
      return { href: "/admin/game#scheduler-card", label: "Scheduler Controls" };
    case "checker":
      return { href: "/admin/game#checker-runs-card", label: "Checker History" };
    case "deployments":
      return { href: "/admin/deployments", label: "Deployment Jobs" };
    case "wireguard":
    case "access":
      return { href: "/admin/players", label: "Runtime Access" };
    default:
      return null;
  }
}

type SchedulerCardProps = {
  filters: GameFilters["schedulerEvent"];
  pendingAction: string | null;
  scheduler: AdminGameStatus["scheduler"];
  schedulerEventPage: AdminSchedulerEventPage;
  schedulerEventsLiveMode: boolean;
  onApplyFilters: () => void;
  onFilterChange: (next: GameFilters["schedulerEvent"]) => void;
  onPage: (direction: "prev" | "next") => void;
  onRefresh: () => void;
  onResetFilters: () => void;
  onStartScheduler: () => void;
  onStopScheduler: () => void;
  onUpdateScheduler: (intervalSeconds: number) => void;
};

function SchedulerCard({
  filters,
  pendingAction,
  scheduler,
  schedulerEventPage,
  schedulerEventsLiveMode,
  onApplyFilters,
  onFilterChange,
  onPage,
  onRefresh,
  onResetFilters,
  onStartScheduler,
  onStopScheduler,
  onUpdateScheduler,
}: SchedulerCardProps): ReactElement {
  const [draftInterval, setDraftInterval] = useState<string>(
    String(scheduler?.interval_seconds ?? 60),
  );
  const auditExpanded =
    !schedulerEventsLiveMode || hasSchedulerFiltersApplied(filters);

  useEffect(() => {
    setDraftInterval(String(scheduler?.interval_seconds ?? 60));
  }, [scheduler?.interval_seconds]);

  const parsedInterval = Number.parseInt(draftInterval, 10);
  const intervalChanged =
    parsedInterval !== scheduler?.interval_seconds && parsedInterval > 0;

  return (
    <Card data-testid="scheduler-card" id="scheduler-card">
      <CardHeader>
        <div className="flex items-start justify-between gap-3">
          <div>
            <CardTitle>Scheduler</CardTitle>
            <CardDescription>
              Core cadence controls stay visible. Audit history expands only when
              you need to inspect or filter it.
            </CardDescription>
          </div>
          <Button
            disabled={pendingAction !== null}
            size="sm"
            variant="outline"
            onClick={onRefresh}
          >
            {pendingAction === "game:scheduler-events" ? (
              <LoaderCircle className="h-4 w-4 animate-spin" />
            ) : (
              <RefreshCw className="h-4 w-4" />
            )}
            Refresh
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        <div className="grid gap-4">
          <SchedulerStatusPanel
            draftInterval={draftInterval}
            intervalChanged={intervalChanged}
            parsedInterval={parsedInterval}
            pendingAction={pendingAction}
            scheduler={scheduler}
            onDraftIntervalChange={setDraftInterval}
            onUpdateScheduler={onUpdateScheduler}
          />
          <SchedulerActionRow
            pendingAction={pendingAction}
            schedulerState={scheduler?.state}
            onStartScheduler={onStartScheduler}
            onStopScheduler={onStopScheduler}
          />
          <details
            className="rounded-md border border-border/70 bg-muted/10 p-3"
            data-testid="scheduler-audit-disclosure"
            open={auditExpanded}
          >
            <summary className="cursor-pointer font-medium text-foreground">
              Scheduler Audit Trail
            </summary>
            <p className="mt-2 text-sm text-muted-foreground">
              Event history, filters, and paging for scheduler state changes and
              tick execution.
            </p>
            <div className="mt-3">
              <SchedulerAuditPanel
                filters={filters}
                pendingAction={pendingAction}
                schedulerEventPage={schedulerEventPage}
                schedulerEventsLiveMode={schedulerEventsLiveMode}
                onApplyFilters={onApplyFilters}
                onFilterChange={onFilterChange}
                onPage={onPage}
                onResetFilters={onResetFilters}
              />
            </div>
          </details>
        </div>
      </CardContent>
    </Card>
  );
}

function SchedulerStatusPanel({
  draftInterval,
  intervalChanged,
  parsedInterval,
  pendingAction,
  scheduler,
  onDraftIntervalChange,
  onUpdateScheduler,
}: {
  draftInterval: string;
  intervalChanged: boolean;
  parsedInterval: number;
  pendingAction: string | null;
  scheduler: AdminGameStatus["scheduler"];
  onDraftIntervalChange: (value: string) => void;
  onUpdateScheduler: (intervalSeconds: number) => void;
}): ReactElement {
  return (
    <InfoPanel layout="grid" tone="surface">
      <SchedulerStateBadge state={scheduler?.state} />
      <SchedulerIntervalControl
        draftInterval={draftInterval}
        intervalChanged={intervalChanged}
        parsedInterval={parsedInterval}
        pendingAction={pendingAction}
        onDraftIntervalChange={onDraftIntervalChange}
        onUpdateScheduler={onUpdateScheduler}
      />
      <InfoLine
        label="Last run"
        value={formatIndonesianDate(scheduler?.last_run_at)}
        valueClassName="font-mono"
      />
      <InfoLine
        label="Next run"
        value={formatIndonesianDate(scheduler?.next_run_at)}
        valueClassName="font-mono"
      />
      <InfoLine
        label="Last tick id"
        value={scheduler?.last_tick_id ?? 0}
        valueClassName="font-mono"
      />
      {scheduler?.last_error ? (
        <p className="text-danger">Last error: {scheduler.last_error}</p>
      ) : null}
    </InfoPanel>
  );
}

function SchedulerStateBadge({
  state,
}: {
  state?: string;
}): ReactElement {
  return (
    <div className="flex items-center justify-between gap-3">
      <span>State</span>
      <Badge
        className={state === "running" ? challengeTone.ready : challengeTone.draft}
        variant="outline"
      >
        {state ?? "unknown"}
      </Badge>
    </div>
  );
}

function SchedulerIntervalControl({
  draftInterval,
  intervalChanged,
  parsedInterval,
  pendingAction,
  onDraftIntervalChange,
  onUpdateScheduler,
}: {
  draftInterval: string;
  intervalChanged: boolean;
  parsedInterval: number;
  pendingAction: string | null;
  onDraftIntervalChange: (value: string) => void;
  onUpdateScheduler: (intervalSeconds: number) => void;
}): ReactElement {
  const updateDisabled =
    pendingAction !== null || !intervalChanged || Number.isNaN(parsedInterval);

  return (
    <div className="flex items-center justify-between gap-3">
      <span className="text-sm text-muted-foreground">Interval</span>
      <div className="flex items-center gap-2">
        <div className="flex h-8 items-center rounded-md border border-input bg-background px-2 py-1 text-xs ring-offset-background focus-within:ring-1 focus-within:ring-ring">
          <input
            type="number"
            min="1"
            className="w-12 bg-transparent outline-none font-mono"
            value={draftInterval}
            onChange={(event) => onDraftIntervalChange(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter" && intervalChanged) {
                onUpdateScheduler(parsedInterval);
              }
            }}
          />
          <span className="ml-1 text-[10px] text-muted-foreground uppercase font-bold">
            Sec
          </span>
        </div>
        <Button
          size="sm"
          variant={intervalChanged ? "default" : "outline"}
          className="h-8 px-2 text-xs"
          disabled={updateDisabled}
          onClick={() => onUpdateScheduler(parsedInterval)}
        >
          {pendingAction === "game:scheduler:update" ? (
            <LoaderCircle className="h-3 w-3 animate-spin" />
          ) : (
            "Update"
          )}
        </Button>
      </div>
    </div>
  );
}

function SchedulerActionRow({
  pendingAction,
  schedulerState,
  onStartScheduler,
  onStopScheduler,
}: {
  pendingAction: string | null;
  schedulerState?: string;
  onStartScheduler: () => void;
  onStopScheduler: () => void;
}): ReactElement {
  return (
    <CardActionRow className="rounded-md border border-border/70 bg-muted/10 p-3 pt-3">
      <Button
        disabled={pendingAction !== null || schedulerState === "running"}
        variant="outline"
        onClick={onStartScheduler}
      >
        {pendingAction === "game:scheduler:start" ? (
          <LoaderCircle className="h-4 w-4 animate-spin" />
        ) : (
          <RefreshCw className="h-4 w-4" />
        )}
        Resume Scheduler
      </Button>
      <Button
        disabled={pendingAction !== null || schedulerState !== "running"}
        variant="outline"
        onClick={onStopScheduler}
      >
        {pendingAction === "game:scheduler:stop" ? (
          <LoaderCircle className="h-4 w-4 animate-spin" />
        ) : (
          <RefreshCw className="h-4 w-4" />
        )}
        Stop Scheduler
      </Button>
    </CardActionRow>
  );
}

function SchedulerAuditPanel({
  filters,
  pendingAction,
  schedulerEventPage,
  schedulerEventsLiveMode,
  onApplyFilters,
  onFilterChange,
  onPage,
  onResetFilters,
}: Pick<
  SchedulerCardProps,
  | "filters"
  | "pendingAction"
  | "schedulerEventPage"
  | "schedulerEventsLiveMode"
  | "onApplyFilters"
  | "onFilterChange"
  | "onPage"
  | "onResetFilters"
>): ReactElement {
  return (
    <InfoPanel tone="surface">
      <SchedulerAuditHeader schedulerEventPage={schedulerEventPage} />
      <SchedulerAuditFilters filters={filters} onFilterChange={onFilterChange} />
      <SchedulerAuditActions
        pendingAction={pendingAction}
        schedulerEventPage={schedulerEventPage}
        schedulerEventsLiveMode={schedulerEventsLiveMode}
        onApplyFilters={onApplyFilters}
        onPage={onPage}
        onResetFilters={onResetFilters}
      />
      <SchedulerEventList events={schedulerEventPage.items} />
    </InfoPanel>
  );
}

function SchedulerAuditHeader({
  schedulerEventPage,
}: {
  schedulerEventPage: AdminSchedulerEventPage;
}): ReactElement {
  return (
    <div className="mb-3 flex flex-col gap-3 2xl:flex-row 2xl:items-start 2xl:justify-between">
      <div>
        <p className="text-base font-semibold text-foreground">
          Scheduler Audit Trail
        </p>
        <p className="text-sm text-muted-foreground">
          Saved across restarts. Filtered results stop live refresh.
        </p>
      </div>
      <div className="shrink-0 self-start">
        <SliceCountBadge
          suffix="event(s)"
          totalCount={schedulerEventPage.total_count}
          visibleCount={schedulerEventPage.items.length}
          offset={schedulerEventPage.offset}
        />
      </div>
    </div>
  );
}

function SchedulerAuditFilters({
  filters,
  onFilterChange,
}: {
  filters: GameFilters["schedulerEvent"];
  onFilterChange: (next: GameFilters["schedulerEvent"]) => void;
}): ReactElement {
  return (
    <div
      data-testid="scheduler-audit-filters"
      className="mb-4 flex flex-wrap gap-3 rounded-md border border-border/70 bg-muted/20 p-3"
    >
      <Field
        label="Page Size"
        htmlFor="scheduler-limit"
        className="w-full sm:w-auto sm:min-w-[9rem] sm:max-w-[9rem] sm:flex-none"
      >
        <select
          id="scheduler-limit"
          className={selectClassName}
          value={filters.limit}
          onChange={(event) =>
            onFilterChange({ ...filters, limit: event.target.value })
          }
        >
          <option value="12">12 per page</option>
          <option value="24">24 per page</option>
          <option value="48">48 per page</option>
          <option value="96">96 per page</option>
        </select>
      </Field>
      <SchedulerAuditInput
        id="scheduler-event-type"
        label="Event Type"
        placeholder="started"
        value={filters.eventType}
        onChange={(eventType) => onFilterChange({ ...filters, eventType })}
      />
      <SchedulerAuditInput
        id="scheduler-source"
        label="Source"
        placeholder="organizer"
        value={filters.source}
        onChange={(source) => onFilterChange({ ...filters, source })}
      />
      <SchedulerAuditInput
        id="scheduler-state"
        label="State"
        placeholder="running"
        value={filters.state}
        onChange={(state) => onFilterChange({ ...filters, state })}
      />
    </div>
  );
}

function SchedulerAuditInput({
  id,
  label,
  placeholder,
  value,
  onChange,
}: {
  id: string;
  label: string;
  placeholder: string;
  value: string;
  onChange: (value: string) => void;
}): ReactElement {
  return (
    <Field label={label} htmlFor={id} className="min-w-[11rem] flex-1">
      <Input
        id={id}
        placeholder={placeholder}
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
    </Field>
  );
}

function SchedulerAuditActions({
  pendingAction,
  schedulerEventPage,
  schedulerEventsLiveMode,
  onApplyFilters,
  onPage,
  onResetFilters,
}: Pick<
  SchedulerCardProps,
  | "pendingAction"
  | "schedulerEventPage"
  | "schedulerEventsLiveMode"
  | "onApplyFilters"
  | "onPage"
  | "onResetFilters"
>): ReactElement {
  return (
    <div className="mb-4 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
      <div className="flex flex-wrap gap-2">
        <LiveModeBadge
          filteredLabel="Filtered"
          liveLabel="Live"
          liveMode={schedulerEventsLiveMode}
        />
      </div>
      <PagedFilterActions
        applyLabel="Apply Event Filters"
        canPageNext={schedulerEventPage.has_next}
        canPagePrev={schedulerEventPage.has_prev}
        disabled={pendingAction !== null}
        liveMode={schedulerEventsLiveMode}
        onApply={onApplyFilters}
        onPage={onPage}
        onReset={onResetFilters}
        resetLabel="Reset Event Filters"
        showLiveModeBadge={false}
      />
    </div>
  );
}

function SchedulerEventList({
  events,
}: {
  events: AdminSchedulerEventPage["items"];
}): ReactElement {
  return (
    <div className="space-y-3">
      {events.length === 0 ? (
        <EmptyStateText message="No scheduler events recorded yet." />
      ) : (
        events.map((event) => <SchedulerEventRow key={event.id} event={event} />)
      )}
    </div>
  );
}

function SchedulerEventRow({
  event,
}: {
  event: AdminSchedulerEventPage["items"][number];
}): ReactElement {
  return (
    <div className="rounded-md border border-border/70 bg-muted/20 p-3 text-sm text-muted-foreground">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant="outline">{event.event_type}</Badge>
          <Badge variant="secondary">{event.source}</Badge>
          <span className="font-mono text-xs text-foreground">
            {formatIndonesianDate(event.created_at)}
          </span>
        </div>
        <span className="font-mono text-xs text-foreground">{event.state}</span>
      </div>
      <p className="mt-2">{event.message ?? "no message recorded"}</p>
      {event.tick_id ? (
        <p className="mt-1 font-mono text-xs text-foreground">
          tick #{event.tick_id}
        </p>
      ) : null}
    </div>
  );
}

function CurrentTickCard({
  currentTick,
}: {
  currentTick?: AdminGameTickStatus;
}): ReactElement {
  return (
    <section
      className="rounded-md border border-border/70 bg-muted/10 p-4"
      data-testid="current-tick-card"
      id="current-tick-card"
    >
      <div className="mb-4">
        <h3 className="text-sm font-semibold text-foreground">Current Tick</h3>
        <p className="text-sm text-muted-foreground">
          The latest persisted tick snapshot from game-core.
        </p>
      </div>
      <div>
        {currentTick ? (
          <InfoPanel layout="grid" tone="surface">
            <div className="flex items-center justify-between gap-3">
              <span>Tick #{currentTick.id}</span>
              <Badge
                className={getTickStatusTone(currentTick.status)}
                variant="outline"
              >
                {currentTick.status}
              </Badge>
            </div>
            <InfoLine
              label="Started"
              value={formatIndonesianDate(currentTick.started_at)}
              valueClassName="font-mono"
            />
            <InfoLine
              label="Completed"
              value={formatIndonesianDate(currentTick.completed_at)}
              valueClassName="font-mono"
            />
            <InfoLine
              label="Runs"
              value={currentTick.total_checker_runs}
              valueClassName="font-mono"
            />
            <InfoLine
              label="Success / failed / skipped"
              value={`${currentTick.successful_checker_runs} / ${currentTick.failed_checker_runs} / ${currentTick.skipped_checker_runs}`}
              valueClassName="font-mono"
            />
            {currentTick.message ? <p>{currentTick.message}</p> : null}
          </InfoPanel>
        ) : (
          <EmptyStateText message="No persisted tick yet." />
        )}
      </div>
    </section>
  );
}

function ScoreboardSnapshotCard({
  pendingAction,
  scoreRows,
  onRefresh,
}: {
  pendingAction: string | null;
  scoreRows: AdminGameScoreRow[];
  onRefresh: () => void;
}): ReactElement {
  const leader = scoreRows[0];
  const lastPlace = scoreRows[scoreRows.length - 1];

  return (
    <section className="rounded-md border border-border/70 bg-muted/10 p-4">
      <div className="mb-4">
        <h3 className="text-sm font-semibold text-foreground">Scoreboard</h3>
        <p className="text-sm text-muted-foreground">
          The full authoritative ranking now lives on its own organizer page.
        </p>
      </div>
      <div className="space-y-4">
        <InfoPanel layout="grid">
          <InfoLine
            label="Rows"
            value={String(scoreRows.length)}
            valueClassName="font-mono"
          />
          <InfoLine
            label="Leader"
            value={leader ? `${leader.team} (${leader.total})` : "not available"}
            valueClassName="font-mono"
          />
          <InfoLine
            label="Last ranked"
            value={
              lastPlace
                ? `#${lastPlace.rank} ${lastPlace.team}`
                : "not available"
            }
            valueClassName="font-mono"
          />
        </InfoPanel>
        <CardActionRow>
          <Button disabled={pendingAction !== null} variant="outline" onClick={onRefresh}>
            {pendingAction === "game:scoreboard" ? (
              <LoaderCircle className="h-4 w-4 animate-spin" />
            ) : (
              <RefreshCw className="h-4 w-4" />
            )}
            Refresh
          </Button>
          <Button asChild variant="outline">
            <Link href="/admin/scoreboard">
              Open Scoreboard
            </Link>
          </Button>
        </CardActionRow>
      </div>
    </section>
  );
}

export function GameAttacksCard({
  attackPage,
  attacksLiveMode,
  highlightedAttackIDs,
  pendingAction,
  onRefresh,
}: {
  attackPage: AdminAttackFeedPage;
  attacksLiveMode: boolean;
  highlightedAttackIDs: string[];
  pendingAction: string | null;
  onRefresh: () => void;
}): ReactElement {
  const attackRows = attackPage.items;

  return (
    <Card id="attack-map">
      <CardHeader>
        <div className="flex items-start justify-between gap-3">
          <div>
            <CardTitle>Accepted Attacks</CardTitle>
            <CardDescription>
              Organizer view of the broad live attack feed. Hover or search to
              reveal team labels without crowding the map.
            </CardDescription>
          </div>
          <Button
            disabled={pendingAction !== null}
            size="sm"
            variant="outline"
            onClick={onRefresh}
          >
            {pendingAction === "game:attacks" ? (
              <LoaderCircle className="h-4 w-4 animate-spin" />
            ) : (
              <RefreshCw className="h-4 w-4" />
            )}
            Refresh
          </Button>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="mb-4 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex flex-wrap gap-2">
            <Badge variant="outline">
              Loaded {attackRows.length} of {attackPage.total_count} attack(s)
            </Badge>
            <Badge variant="outline">Broad feed</Badge>
            {attacksLiveMode ? <Badge variant="outline">Live stream</Badge> : null}
          </div>
        </div>

        <AttackSliceSummaryGrid rows={attackRows} />

        <AttackMapPanel
          attackRows={attackRows}
          description="Accepted submissions rendered as attacker-to-victim links for the broad organizer feed."
          highlightedAttackIDs={highlightedAttackIDs}
          title="Organizer attack map"
        />

        <AttackFeedTable
          attackRows={attackRows}
          emptyMessage="No accepted attacks match the current organizer slice."
        />
      </CardContent>
    </Card>
  );
}

function GameScoreboardCard({
  pendingAction,
  scoreRows,
  onRefresh,
}: {
  pendingAction: string | null;
  scoreRows: AdminGameScoreRow[];
  onRefresh: () => void;
}): ReactElement {
  return (
    <Card data-testid="authoritative-scoreboard-card">
      <CardHeader>
        <div className="flex items-start justify-between gap-3">
          <div>
            <CardTitle>Authoritative Scoreboard</CardTitle>
            <CardDescription>
              Dense service-by-service rankings derived from persisted
              game-core scoring state.
            </CardDescription>
          </div>
          <Button
            disabled={pendingAction !== null}
            size="sm"
            variant="outline"
            onClick={onRefresh}
          >
            {pendingAction === "game:scoreboard" ? (
              <LoaderCircle className="h-4 w-4 animate-spin" />
            ) : (
              <RefreshCw className="h-4 w-4" />
            )}
            Refresh
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        <ScoreboardTable
          scoreRows={scoreRows}
          emptyMessage="No score rows persisted yet."
        />
      </CardContent>
    </Card>
  );
}

function CheckerRunsCard({
  checkerRunPage,
  checkerRunsLiveMode,
  filters,
  pendingAction,
  onApplyFilters,
  onFilterChange,
  onPage,
  onRefresh,
  onResetFilters,
}: {
  checkerRunPage: AdminCheckerRunPage;
  checkerRunsLiveMode: boolean;
  filters: GameFilters["checkerRun"];
  pendingAction: string | null;
  onApplyFilters: () => void;
  onFilterChange: (next: GameFilters["checkerRun"]) => void;
  onPage: (direction: "prev" | "next") => void;
  onRefresh: () => void;
  onResetFilters: () => void;
}): ReactElement {
  const checkerRunRows = checkerRunPage.items;
  const sliceStatusCounts = checkerRunRows.reduce(
    (counts, run) => {
      counts[run.status] = (counts[run.status] ?? 0) + 1;
      return counts;
    },
    {} as Record<string, number>,
  );
  const detailExpanded =
    !checkerRunsLiveMode || hasCheckerRunFiltersApplied(filters);

  return (
    <Card data-testid="checker-runs-card" id="checker-runs-card">
      <CardHeader>
        <div className="flex items-start justify-between gap-3">
          <div>
            <CardTitle>Checker Investigation</CardTitle>
            <CardDescription>
              Summary-first checker diagnostics. Expand the history only
              when runtime health points you here.
            </CardDescription>
          </div>
          <Button
            disabled={pendingAction !== null}
            size="sm"
            variant="outline"
            onClick={onRefresh}
          >
            {pendingAction === "game:checker-runs" ? (
              <LoaderCircle className="h-4 w-4 animate-spin" />
            ) : (
              <RefreshCw className="h-4 w-4" />
            )}
            Refresh
          </Button>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <InfoPanel layout="grid" tone="surface">
          <InfoLine
            label="Visible runs"
            value={`${checkerRunRows.length} of ${checkerRunPage.total_count}`}
            valueClassName="font-mono"
          />
          <InfoLine
            label="Failed on slice"
            value={String(sliceStatusCounts.failed ?? 0)}
            valueClassName="font-mono"
          />
          <InfoLine
            label="Skipped on slice"
            value={String(sliceStatusCounts.skipped ?? 0)}
            valueClassName="font-mono"
          />
          <InfoLine
            label="Stream mode"
            value={checkerRunsLiveMode ? "live stream" : "filters applied"}
            valueClassName="font-mono"
          />
        </InfoPanel>
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex flex-wrap gap-2">
            <SliceCountBadge
              totalCount={checkerRunPage.total_count}
              visibleCount={checkerRunRows.length}
              offset={checkerRunPage.offset}
            />
            <LiveModeBadge
              filteredLabel="Filters applied"
              liveLabel="Live stream"
              liveMode={checkerRunsLiveMode}
            />
          </div>
          <Button asChild size="sm" variant="ghost">
            <Link href="/admin/game#runtime-health-card">Back to Runtime Health</Link>
          </Button>
        </div>
        <details
          className="rounded-md border border-border/70 bg-muted/10 p-3"
          data-testid="checker-runs-disclosure"
          open={detailExpanded}
        >
          <summary className="cursor-pointer font-medium text-foreground">
            Checker history
          </summary>
          <p className="mt-2 text-sm text-muted-foreground">
            Phase-level history, filters, and paging for checker investigation.
          </p>
          <div className="mt-3 space-y-4">
            <div className="space-y-3 rounded-md border border-border/70 bg-muted/20 p-3">
              <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
                <Field label="Page Size" htmlFor="checker-limit">
                  <select
                    id="checker-limit"
                    className={selectClassName}
                    value={filters.limit}
                    onChange={(event) =>
                      onFilterChange({ ...filters, limit: event.target.value })
                    }
                  >
                    <option value="18">18 per page</option>
                    <option value="36">36 per page</option>
                    <option value="72">72 per page</option>
                    <option value="144">144 per page</option>
                  </select>
                </Field>
                <Field label="Tick ID" htmlFor="checker-tick">
                  <Input
                    id="checker-tick"
                    type="number"
                    min="1"
                    value={filters.tickId}
                    onChange={(event) =>
                      onFilterChange({ ...filters, tickId: event.target.value })
                    }
                  />
                </Field>
                <Field label="Team ID" htmlFor="checker-team">
                  <Input
                    id="checker-team"
                    type="number"
                    min="1"
                    value={filters.teamId}
                    onChange={(event) =>
                      onFilterChange({ ...filters, teamId: event.target.value })
                    }
                  />
                </Field>
                <Field label="Challenge ID" htmlFor="checker-challenge">
                  <Input
                    id="checker-challenge"
                    type="number"
                    min="1"
                    value={filters.challengeId}
                    onChange={(event) =>
                      onFilterChange({
                        ...filters,
                        challengeId: event.target.value,
                      })
                    }
                  />
                </Field>
              </div>
              <div className="grid gap-3 md:grid-cols-2 xl:max-w-xl">
                <Field label="Phase" htmlFor="checker-phase">
                  <Input
                    id="checker-phase"
                    placeholder="put"
                    value={filters.phase}
                    onChange={(event) =>
                      onFilterChange({ ...filters, phase: event.target.value })
                    }
                  />
                </Field>
                <Field label="Status" htmlFor="checker-status">
                  <Input
                    id="checker-status"
                    placeholder="failed"
                    value={filters.status}
                    onChange={(event) =>
                      onFilterChange({ ...filters, status: event.target.value })
                    }
                  />
                </Field>
              </div>
            </div>
            <PagedFilterActions
              applyLabel="Apply Run Filters"
              canPageNext={checkerRunPage.has_next}
              canPagePrev={checkerRunPage.has_prev}
              disabled={pendingAction !== null}
              liveMode={checkerRunsLiveMode}
              onApply={onApplyFilters}
              onPage={onPage}
              onReset={onResetFilters}
              resetLabel="Reset Run Filters"
            />
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-[100px]">Tick</TableHead>
                  <TableHead>Team</TableHead>
                  <TableHead>Challenge</TableHead>
                  <TableHead>Service State</TableHead>
                  <TableHead>Phase</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Target</TableHead>
                  <TableHead>Checked</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {checkerRunRows.length === 0 ? (
                  <EmptyTableRow
                    colSpan={8}
                    message="No checker runs persisted yet."
                  />
                ) : (
                  checkerRunRows.map((run) => (
                    <TableRow key={run.id}>
                      <TableCell className="font-semibold">
                        #{run.tick_id}
                      </TableCell>
                      <TableCell>{run.team_name}</TableCell>
                      <TableCell>
                        <div>
                          <p>{run.challenge_name}</p>
                          <p className="text-xs text-muted-foreground">
                            #{run.challenge_id}
                          </p>
                        </div>
                      </TableCell>
                      <TableCell className="font-mono text-xs uppercase">
                        {run.phase}
                      </TableCell>
                      <TableCell>
                        <Badge
                          className={getServiceStateTone(run.service_state)}
                          variant="outline"
                        >
                          {formatServiceStateLabel(run.service_state)}
                        </Badge>
                        {run.state_message ? (
                          <p className="mt-2 max-w-[16rem] text-xs leading-5 text-muted-foreground">
                            {run.state_message}
                          </p>
                        ) : null}
                      </TableCell>
                      <TableCell>
                        <Badge
                          className={getCheckerRunStatusTone(run.status)}
                          variant="outline"
                        >
                          {run.status}
                        </Badge>
                        {run.message ? (
                          <p className="mt-2 max-w-[16rem] text-xs leading-5 text-muted-foreground">
                            {run.message}
                          </p>
                        ) : null}
                      </TableCell>
                      <TableCell className="font-mono text-xs">
                        {run.target}
                      </TableCell>
                      <TableCell className="text-xs text-muted-foreground">
                        <div>
                          <p>{formatIndonesianDate(run.checked_at)}</p>
                          <p>exit {run.exit_code}</p>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>
        </details>
      </CardContent>
    </Card>
  );
}

function hasSchedulerFiltersApplied(
  filters: GameFilters["schedulerEvent"],
): boolean {
  return (
    filters.eventType.trim() !== "" ||
    filters.source.trim() !== "" ||
    filters.state.trim() !== ""
  );
}

function hasCheckerRunFiltersApplied(
  filters: GameFilters["checkerRun"],
): boolean {
  return (
    filters.tickId.trim() !== "" ||
    filters.teamId.trim() !== "" ||
    filters.challengeId.trim() !== "" ||
    filters.phase.trim() !== "" ||
    filters.status.trim() !== ""
  );
}

export function ErrorBanner({ message }: { message: string }): ReactElement {
  return <StatusBanner message={message} variant="error" />;
}

export function NoteBanner({ message }: { message: string }): ReactElement {
  return <StatusBanner message={message} variant="success" />;
}

function WireGuardDialog({
  open,
  peer,
  onClose,
  onDownload,
}: {
  open: boolean;
  peer: AdminWireGuardPeer | null;
  onClose: () => void;
  onDownload: (peer: AdminWireGuardPeer) => void;
}): ReactElement {
  return (
    <AppDialog
      body={
        peer ? (
          <div className="grid gap-4">
            <InfoPanel layout="grid" className="sm:grid-cols-2">
              <InfoLine
                label="Address"
                value={peer.address}
                valueClassName="font-mono"
              />
              <InfoLine
                label="Status"
                value={peer.status}
                valueClassName="font-mono"
              />
              <InfoLine
                label="Issued"
                value={formatIndonesianDate(peer.issued_at)}
                valueClassName="font-mono"
              />
              <InfoLine
                label="Endpoint"
                value={peer.server_endpoint}
                valueClassName="font-mono"
              />
            </InfoPanel>
            <pre className="max-h-[22rem] overflow-auto rounded-md border border-border/70 bg-card p-4 text-xs leading-6 text-foreground">
              {peer.config}
            </pre>
          </div>
        ) : null
      }
      contentClassName="w-[min(94vw,46rem)]"
      description={
        peer
          ? `${peer.display_name} on ${peer.team_name} using peer ${peer.wireguard_peer}.`
          : "Select a player to inspect the generated peer config."
      }
      footer={
        <>
          <Button variant="outline" onClick={onClose}>
            Close
          </Button>
          <Button disabled={!peer} onClick={() => peer && onDownload(peer)}>
            <Download className="h-4 w-4" />
            Download .conf
          </Button>
        </>
      }
      onClose={onClose}
      open={open}
      title="WireGuard Config"
    />
  );
}

function RuntimeStatusBlock({
  tone,
  state,
  rows,
  error,
}: {
  tone: string;
  state: string;
  rows: Array<{ label: string; value?: string }>;
  error?: string;
}): ReactElement {
  return (
    <InfoPanel layout="grid">
      <div className="flex items-center justify-between gap-3">
        <span>Runtime status</span>
        <Badge className={tone} variant="outline">
          {state}
        </Badge>
      </div>
      {rows.map((row) => {
        if (!row.value) {
          return null;
        }

        return (
          <InfoLine
            key={row.label}
            label={row.label}
            value={row.value}
            valueClassName="font-mono"
          />
        );
      })}
      {error ? <p className="text-danger">Last error: {error}</p> : null}
    </InfoPanel>
  );
}

function Field({
  label,
  htmlFor,
  children,
  className,
}: {
  label: string;
  htmlFor: string;
  children: ReactNode;
  className?: string;
}): ReactElement {
  return (
    <div className={cn("min-w-0 space-y-2", className)}>
      <Label htmlFor={htmlFor}>{label}</Label>
      {children}
    </div>
  );
}

function getChallengeRuntimeTone(
  status: AdminChallenge["runtime_status"],
): string {
  if (status === "ready") {
    return challengeTone.ready;
  }
  if (status === "deploying") {
    return challengeTone.deploying;
  }
  return challengeTone.draft;
}

function getValidationStatusTone(status: string): string {
  if (status === "valid") {
    return validationTone.valid;
  }
  if (status === "invalid") {
    return validationTone.invalid;
  }
  if (status === "unavailable") {
    return validationTone.unavailable;
  }
  return validationTone.unchecked;
}

function getGatewayTone(state: string | undefined): string {
  if (state === "applied") {
    return gatewayTone.applied;
  }
  if (state === "error") {
    return gatewayTone.error;
  }
  if (state === "disabled") {
    return gatewayTone.disabled;
  }
  return gatewayTone.idle;
}

function getTickStatusTone(status: string): string {
  if (status === "completed") {
    return challengeTone.ready;
  }
  if (status === "running") {
    return challengeTone.deploying;
  }
  return validationTone.invalid;
}

function getCheckerRunStatusTone(status: string): string {
  if (status === "success") {
    return checkerRunTone.success;
  }
  if (status === "failed") {
    return checkerRunTone.failed;
  }
  if (status === "skipped") {
    return checkerRunTone.skipped;
  }
  return validationTone.unchecked;
}

function getServiceStateTone(status: string | undefined): string {
  switch ((status ?? "").trim().toLowerCase()) {
    case "ok":
      return checkerRunTone.success;
    case "recovering":
      return challengeTone.deploying;
    case "flag_not_found":
    case "faulty":
    case "down":
      return checkerRunTone.failed;
    default:
      return validationTone.unchecked;
  }
}

function formatServiceStateLabel(status: string | undefined): string {
  switch ((status ?? "").trim().toLowerCase()) {
    case "ok":
      return "ok";
    case "recovering":
      return "recovering";
    case "flag_not_found":
      return "flag not found";
    case "faulty":
      return "faulty";
    case "down":
      return "down";
    default:
      return "unknown";
  }
}

type ChallengeDraftField =
  | "name"
  | "baselineImage"
  | "checkerImage"
  | "sourceBundlePath"
  | "servicePort"
  | "serviceSubnetOctet"
  | "weight";

type ChallengeDraftValidation = {
  effectiveChallengeID: number;
  effectiveServicePort: number;
  effectiveServiceSubnetOctet: number;
  effectiveWeight: number;
  endpointExample: string;
  errors: string[];
  fieldErrors: Partial<Record<ChallengeDraftField, string>>;
};

type TeamDraftField = "name" | "contactEmail";

type TeamDraftValidation = {
  errors: string[];
  fieldErrors: Partial<Record<TeamDraftField, string>>;
};

type PlayerDraftField =
  | "teamId"
  | "displayName"
  | "email"
  | "password"
  | "role";

type PlayerDraftValidation = {
  errors: string[];
  fieldErrors: Partial<Record<PlayerDraftField, string>>;
};

const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

function validateTeamDraft(teamDraft: TeamDraft): TeamDraftValidation {
  const fieldErrors: Partial<Record<TeamDraftField, string>> = {};
  const errors: string[] = [];

  if (teamDraft.name.trim() === "") {
    fieldErrors.name = "Enter a team name.";
    errors.push("Team name is required.");
  }

  const email = teamDraft.contactEmail.trim();
  if (email === "") {
    fieldErrors.contactEmail = "Enter a contact email.";
    errors.push("Contact email is required.");
  } else if (!emailPattern.test(email)) {
    fieldErrors.contactEmail = "Enter a valid email address.";
    errors.push("Contact email must be a valid email address.");
  }

  return { errors, fieldErrors };
}

function validatePlayerDraft(
  playerDraft: PlayerDraft,
  formMode: FormMode,
): PlayerDraftValidation {
  const fieldErrors: Partial<Record<PlayerDraftField, string>> = {};
  const errors: string[] = [];

  if (formMode === "create" && playerDraft.teamId <= 0) {
    fieldErrors.teamId = "Select a team.";
    errors.push("Team selection is required.");
  }

  if (playerDraft.displayName.trim() === "") {
    fieldErrors.displayName = "Enter a display name.";
    errors.push("Display name is required.");
  }

  const email = playerDraft.email.trim();
  if (email === "") {
    fieldErrors.email = "Enter an email address.";
    errors.push("Email is required.");
  } else if (!emailPattern.test(email)) {
    fieldErrors.email = "Enter a valid email address.";
    errors.push("Email must be a valid email address.");
  }

  if (formMode === "create" && playerDraft.password.trim() === "") {
    fieldErrors.password = "Enter a password.";
    errors.push("Password is required.");
  }

  if (playerDraft.role !== "member" && playerDraft.role !== "captain") {
    fieldErrors.role = "Select a valid role.";
    errors.push("Role must be member or captain.");
  }

  return { errors, fieldErrors };
}

function parseOptionalInteger(raw: string): number | null {
  const trimmed = raw.trim();
  if (trimmed === "") {
    return null;
  }
  if (!/^\d+$/.test(trimmed)) {
    return Number.NaN;
  }
  return Number(trimmed);
}

function nextChallengeID(challengeRows: AdminChallenge[]): number {
  return (
    challengeRows.reduce((maxID, challenge) => Math.max(maxID, challenge.id), 0) +
    1
  );
}

function validateChallengeDraft(
  challengeDraft: ChallengeDraft,
  challengeRows: AdminChallenge[],
  formMode: FormMode,
  editingID: number | null,
  teamRows: AdminTeam[],
): ChallengeDraftValidation {
  const fieldErrors: Partial<Record<ChallengeDraftField, string>> = {};
  const errors: string[] = [];

  const effectiveChallengeID =
    formMode === "edit" && editingID !== null
      ? editingID
      : nextChallengeID(challengeRows);
  const parsedWeight = parseOptionalInteger(challengeDraft.weight);
  const effectiveWeight =
    parsedWeight === null || Number.isNaN(parsedWeight) || parsedWeight <= 0
      ? 1
      : parsedWeight;

  if (challengeDraft.name.trim() === "") {
    fieldErrors.name = "Enter a challenge name.";
    errors.push("Challenge name is required.");
  }
  if (challengeDraft.baselineImage.trim() === "") {
    fieldErrors.baselineImage = "Enter the baseline runtime image.";
    errors.push("Baseline image is required.");
  }
  if (challengeDraft.checkerImage.trim() === "") {
    fieldErrors.checkerImage = "Enter the checker image.";
    errors.push("Checker image is required.");
  }

  const trimmedSourceBundlePath = challengeDraft.sourceBundlePath.trim();
  if (
    trimmedSourceBundlePath !== "" &&
    (trimmedSourceBundlePath.startsWith("/") ||
      trimmedSourceBundlePath === "." ||
      trimmedSourceBundlePath === ".." ||
      trimmedSourceBundlePath.split("/").some((segment) => segment === ".."))
  ) {
    fieldErrors.sourceBundlePath =
      "Use a relative path inside AD_CHALLENGE_SOURCE_ROOT.";
    errors.push("Source bundle path must stay inside AD_CHALLENGE_SOURCE_ROOT.");
  }

  if (
    parsedWeight !== null &&
    (Number.isNaN(parsedWeight) || parsedWeight <= 0)
  ) {
    fieldErrors.weight = "Enter a positive integer.";
    errors.push("Weight must be a positive integer.");
  }

  const parsedServicePort = parseOptionalInteger(challengeDraft.servicePort);
  const effectiveServicePort =
    parsedServicePort === null || Number.isNaN(parsedServicePort)
      ? 10000 + effectiveChallengeID
      : parsedServicePort;

  const parsedServiceSubnetOctet = parseOptionalInteger(
    challengeDraft.serviceSubnetOctet,
  );
  const effectiveServiceSubnetOctet =
    parsedServiceSubnetOctet === null || Number.isNaN(parsedServiceSubnetOctet)
      ? Math.max(1, effectiveChallengeID)
      : parsedServiceSubnetOctet;

  if (formMode === "create") {
    if (
      parsedServicePort !== null &&
      (Number.isNaN(parsedServicePort) ||
        parsedServicePort <= 0 ||
        parsedServicePort > 65535)
    ) {
      fieldErrors.servicePort = "Enter a port between 1 and 65535.";
      errors.push("Service port must be between 1 and 65535.");
    }
    if (
      parsedServiceSubnetOctet !== null &&
      (Number.isNaN(parsedServiceSubnetOctet) ||
        parsedServiceSubnetOctet <= 0 ||
        parsedServiceSubnetOctet > 254)
    ) {
      fieldErrors.serviceSubnetOctet = "Enter an octet between 1 and 254.";
      errors.push("Subnet octet must be between 1 and 254.");
    }

    if (
      !Number.isNaN(effectiveServiceSubnetOctet) &&
      effectiveServiceSubnetOctet > 0 &&
      effectiveServiceSubnetOctet <= 254
    ) {
      const conflictingChallenge = challengeRows.find(
        (challenge) =>
          challenge.id !== editingID &&
          challenge.service_subnet_octet === effectiveServiceSubnetOctet,
      );
      if (conflictingChallenge) {
        fieldErrors.serviceSubnetOctet = `Already assigned to ${conflictingChallenge.name}.`;
        errors.push(
          `Subnet octet ${effectiveServiceSubnetOctet} is already assigned to ${conflictingChallenge.name}.`,
        );
      }
    }
  }

  const exampleTeamID = teamRows[0]?.id ?? 101;
  const endpointExample = `10.80.${effectiveServiceSubnetOctet}.${exampleTeamID >= 101 ? exampleTeamID - 90 : 11}:${effectiveServicePort}`;

  return {
    effectiveChallengeID,
    effectiveServicePort,
    effectiveServiceSubnetOctet,
    effectiveWeight,
    endpointExample,
    errors,
    fieldErrors,
  };
}

export function EntityFormDialog({
  formMode,
  formEntity,

  pendingAction,
  teamDraft,
  playerDraft,
  challengeDraft,
  challengeRows,
  editingID,
  teamRows,
  onTeamDraftChange,
  onPlayerDraftChange,
  onChallengeDraftChange,
  onClose,
  onSubmit,
}: {
  formMode: FormMode;
  formEntity: FormEntity;

  pendingAction: string | null;
  teamDraft: TeamDraft;
  playerDraft: PlayerDraft;
  challengeDraft: ChallengeDraft;
  challengeRows: AdminChallenge[];
  editingID: number | null;
  teamRows: AdminTeam[];
  onTeamDraftChange: (next: TeamDraft) => void;
  onPlayerDraftChange: (next: PlayerDraft) => void;
  onChallengeDraftChange: (next: ChallengeDraft) => void;
  onClose: () => void;
  onSubmit: () => void;
}): ReactElement {
  const isOpen = formMode !== null && formEntity !== null;
  const title =
    formMode === "edit"
      ? `Edit ${formEntity ?? "item"}`
      : `Create ${formEntity ?? "item"}`;
  const isSubmitting =
    pendingAction !== null &&
    (pendingAction.includes(":create") || pendingAction.includes(":update"));
  const teamValidation =
    formEntity === "team" ? validateTeamDraft(teamDraft) : null;
  const playerValidation =
    formEntity === "player"
      ? validatePlayerDraft(playerDraft, formMode)
      : null;
  const challengeValidation =
    formEntity === "challenge"
      ? validateChallengeDraft(
          challengeDraft,
          challengeRows,
          formMode,
          editingID,
          teamRows,
        )
      : null;
  const teamFormInvalid = teamValidation !== null && teamValidation.errors.length > 0;
  const playerFormInvalid =
    playerValidation !== null && playerValidation.errors.length > 0;
  const challengeFormInvalid =
    challengeValidation !== null && challengeValidation.errors.length > 0;
  const formInvalid = teamFormInvalid || playerFormInvalid || challengeFormInvalid;

  let body: ReactNode = null;

  if (formEntity === "team") {
    const fieldErrors = teamValidation?.fieldErrors ?? {};
    body = (
      <div className="space-y-4">
        <Field label="Team name" htmlFor="form-team-name">
          <div className="space-y-2">
            <Input
              id="form-team-name"
              aria-invalid={fieldErrors.name ? true : undefined}
              className={
                fieldErrors.name
                  ? "border-destructive focus-visible:ring-destructive/20"
                  : undefined
              }
              value={teamDraft.name}
              onChange={(event) =>
                onTeamDraftChange({ ...teamDraft, name: event.target.value })
              }
            />
            {fieldErrors.name ? (
              <p className="text-xs leading-5 text-destructive">
                {fieldErrors.name}
              </p>
            ) : null}
          </div>
        </Field>
        <Field label="Contact email" htmlFor="form-team-contact">
          <div className="space-y-2">
            <Input
              id="form-team-contact"
              type="email"
              aria-invalid={fieldErrors.contactEmail ? true : undefined}
              className={
                fieldErrors.contactEmail
                  ? "border-destructive focus-visible:ring-destructive/20"
                  : undefined
              }
              value={teamDraft.contactEmail}
              onChange={(event) =>
                onTeamDraftChange({
                  ...teamDraft,
                  contactEmail: event.target.value,
                })
              }
            />
            {fieldErrors.contactEmail ? (
              <p className="text-xs leading-5 text-destructive">
                {fieldErrors.contactEmail}
              </p>
            ) : null}
          </div>
        </Field>
      </div>
    );
  } else if (formEntity === "player") {
    const fieldErrors = playerValidation?.fieldErrors ?? {};
    body = (
      <div className="space-y-4">
        {formMode === "create" && (
          <Field label="Team" htmlFor="form-player-team">
            <div className="space-y-2">
              <select
                id="form-player-team"
                aria-invalid={fieldErrors.teamId ? true : undefined}
                className={cn(
                  selectClassName,
                  fieldErrors.teamId
                    ? "border-destructive focus-visible:ring-destructive/20"
                    : undefined,
                )}
                value={playerDraft.teamId}
                onChange={(event) =>
                  onPlayerDraftChange({
                    ...playerDraft,
                    teamId: Number(event.target.value),
                  })
                }
              >
                <option value={0} disabled>
                  Select a team
                </option>
                {teamRows.map((team) => (
                  <option key={team.id} value={team.id}>
                    {team.name}
                  </option>
                ))}
              </select>
              {fieldErrors.teamId ? (
                <p className="text-xs leading-5 text-destructive">
                  {fieldErrors.teamId}
                </p>
              ) : null}
            </div>
          </Field>
        )}
        <Field label="Display name" htmlFor="form-player-name">
          <div className="space-y-2">
            <Input
              id="form-player-name"
              aria-invalid={fieldErrors.displayName ? true : undefined}
              className={
                fieldErrors.displayName
                  ? "border-destructive focus-visible:ring-destructive/20"
                  : undefined
              }
              value={playerDraft.displayName}
              onChange={(event) =>
                onPlayerDraftChange({
                  ...playerDraft,
                  displayName: event.target.value,
                })
              }
            />
            {fieldErrors.displayName ? (
              <p className="text-xs leading-5 text-destructive">
                {fieldErrors.displayName}
              </p>
            ) : null}
          </div>
        </Field>
        <Field label="Email" htmlFor="form-player-email">
          <div className="space-y-2">
            <Input
              id="form-player-email"
              type="email"
              aria-invalid={fieldErrors.email ? true : undefined}
              className={
                fieldErrors.email
                  ? "border-destructive focus-visible:ring-destructive/20"
                  : undefined
              }
              value={playerDraft.email}
              onChange={(event) =>
                onPlayerDraftChange({
                  ...playerDraft,
                  email: event.target.value,
                })
              }
            />
            {fieldErrors.email ? (
              <p className="text-xs leading-5 text-destructive">
                {fieldErrors.email}
              </p>
            ) : null}
          </div>
        </Field>
        {formMode === "create" && (
          <Field label="Password" htmlFor="form-player-password">
            <div className="space-y-2">
              <Input
                id="form-player-password"
                type="password"
                aria-invalid={fieldErrors.password ? true : undefined}
                className={
                  fieldErrors.password
                    ? "border-destructive focus-visible:ring-destructive/20"
                    : undefined
                }
                value={playerDraft.password}
                onChange={(event) =>
                  onPlayerDraftChange({
                    ...playerDraft,
                    password: event.target.value,
                  })
                }
              />
              {fieldErrors.password ? (
                <p className="text-xs leading-5 text-destructive">
                  {fieldErrors.password}
                </p>
              ) : null}
            </div>
          </Field>
        )}
        <Field label="Role" htmlFor="form-player-role">
          <div className="space-y-2">
            <select
              id="form-player-role"
              aria-invalid={fieldErrors.role ? true : undefined}
              className={cn(
                selectClassName,
                fieldErrors.role
                  ? "border-destructive focus-visible:ring-destructive/20"
                  : undefined,
              )}
              value={playerDraft.role}
              onChange={(event) =>
                onPlayerDraftChange({ ...playerDraft, role: event.target.value })
              }
            >
              <option value="member">member</option>
              <option value="captain">captain</option>
            </select>
            {fieldErrors.role ? (
              <p className="text-xs leading-5 text-destructive">
                {fieldErrors.role}
              </p>
            ) : null}
          </div>
        </Field>
      </div>
    );
  } else if (formEntity === "challenge") {
    const fieldErrors = challengeValidation?.fieldErrors ?? {};
    const sourceBundlePath = challengeDraft.sourceBundlePath.trim();
    body = (
      <div className="space-y-4">
        <Field label="Challenge name" htmlFor="form-challenge-name">
          <div className="space-y-2">
            <Input
              id="form-challenge-name"
              aria-invalid={fieldErrors.name ? true : undefined}
              className={
                fieldErrors.name
                  ? "border-destructive focus-visible:ring-destructive/20"
                  : undefined
              }
              value={challengeDraft.name}
              onChange={(event) =>
                onChallengeDraftChange({
                  ...challengeDraft,
                  name: event.target.value,
                })
              }
            />
            {fieldErrors.name ? (
              <p className="text-xs leading-5 text-destructive">
                {fieldErrors.name}
              </p>
            ) : null}
          </div>
        </Field>
        <Field label="Baseline image" htmlFor="form-challenge-baseline">
          <div className="space-y-2">
            <Input
              id="form-challenge-baseline"
              aria-invalid={fieldErrors.baselineImage ? true : undefined}
              className={
                fieldErrors.baselineImage
                  ? "border-destructive focus-visible:ring-destructive/20"
                  : undefined
              }
              value={challengeDraft.baselineImage}
              onChange={(event) =>
                onChallengeDraftChange({
                  ...challengeDraft,
                  baselineImage: event.target.value,
                })
              }
            />
            {fieldErrors.baselineImage ? (
              <p className="text-xs leading-5 text-destructive">
                {fieldErrors.baselineImage}
              </p>
            ) : null}
          </div>
        </Field>
        <Field label="Checker image" htmlFor="form-challenge-checker">
          <div className="space-y-2">
            <Input
              id="form-challenge-checker"
              aria-invalid={fieldErrors.checkerImage ? true : undefined}
              className={
                fieldErrors.checkerImage
                  ? "border-destructive focus-visible:ring-destructive/20"
                  : undefined
              }
              value={challengeDraft.checkerImage}
              onChange={(event) =>
                onChallengeDraftChange({
                  ...challengeDraft,
                  checkerImage: event.target.value,
                })
              }
            />
            {fieldErrors.checkerImage ? (
              <p className="text-xs leading-5 text-destructive">
                {fieldErrors.checkerImage}
              </p>
            ) : null}
          </div>
        </Field>
        <Field label="Source bundle path" htmlFor="form-challenge-source-bundle">
          <div className="space-y-2">
            <Input
              id="form-challenge-source-bundle"
              aria-invalid={fieldErrors.sourceBundlePath ? true : undefined}
              className={
                fieldErrors.sourceBundlePath
                  ? "border-destructive focus-visible:ring-destructive/20"
                  : undefined
              }
              value={challengeDraft.sourceBundlePath}
              onChange={(event) =>
                onChallengeDraftChange({
                  ...challengeDraft,
                  sourceBundlePath: event.target.value,
                })
              }
            />
            <p className="text-xs leading-5 text-muted-foreground">
              Relative to <code>AD_CHALLENGE_SOURCE_ROOT</code>. The download is
              served verbatim, so use a sanitized whitebox directory or archive
              with dummy placeholder secrets and inject real secrets only at
              runtime.
            </p>
            {fieldErrors.sourceBundlePath ? (
              <p className="text-xs leading-5 text-destructive">
                {fieldErrors.sourceBundlePath}
              </p>
            ) : null}
          </div>
        </Field>
        {formMode === "create" && (
          <>
            <Field label="Service port" htmlFor="form-challenge-port">
              <div className="space-y-2">
                <Input
                  id="form-challenge-port"
                  type="number"
                  min={1}
                  max={65535}
                  placeholder={String(
                    challengeValidation?.effectiveServicePort ?? 10001,
                  )}
                  aria-invalid={fieldErrors.servicePort ? true : undefined}
                  className={
                    fieldErrors.servicePort
                      ? "border-destructive focus-visible:ring-destructive/20"
                      : undefined
                  }
                  value={challengeDraft.servicePort}
                  onChange={(event) =>
                    onChallengeDraftChange({
                      ...challengeDraft,
                      servicePort: event.target.value,
                    })
                  }
                />
                <p className="text-xs leading-5 text-muted-foreground">
                  Leave blank to auto-assign{" "}
                  <code>{challengeValidation?.effectiveServicePort ?? 10001}</code>.
                </p>
                {fieldErrors.servicePort ? (
                  <p className="text-xs leading-5 text-destructive">
                    {fieldErrors.servicePort}
                  </p>
                ) : null}
              </div>
            </Field>
            <Field label="Subnet octet" htmlFor="form-challenge-octet">
              <div className="space-y-2">
                <Input
                  id="form-challenge-octet"
                  type="number"
                  min={1}
                  max={254}
                  placeholder={String(
                    challengeValidation?.effectiveServiceSubnetOctet ?? 1,
                  )}
                  aria-invalid={fieldErrors.serviceSubnetOctet ? true : undefined}
                  className={
                    fieldErrors.serviceSubnetOctet
                      ? "border-destructive focus-visible:ring-destructive/20"
                      : undefined
                  }
                  value={challengeDraft.serviceSubnetOctet}
                  onChange={(event) =>
                    onChallengeDraftChange({
                      ...challengeDraft,
                      serviceSubnetOctet: event.target.value,
                    })
                  }
                />
                <p className="text-xs leading-5 text-muted-foreground">
                  Leave blank to auto-assign{" "}
                  <code>
                    {challengeValidation?.effectiveServiceSubnetOctet ?? 1}
                  </code>
                  .
                </p>
                {fieldErrors.serviceSubnetOctet ? (
                  <p className="text-xs leading-5 text-destructive">
                    {fieldErrors.serviceSubnetOctet}
                  </p>
                ) : null}
              </div>
            </Field>
          </>
        )}
        <Field label="Weight" htmlFor="form-challenge-weight">
          <div className="space-y-2">
            <Input
              id="form-challenge-weight"
              type="number"
              min={1}
              aria-invalid={fieldErrors.weight ? true : undefined}
              className={
                fieldErrors.weight
                  ? "border-destructive focus-visible:ring-destructive/20"
                  : undefined
              }
              value={challengeDraft.weight}
              onChange={(event) =>
                onChallengeDraftChange({
                  ...challengeDraft,
                  weight: event.target.value,
                })
              }
            />
            {fieldErrors.weight ? (
              <p className="text-xs leading-5 text-destructive">
                {fieldErrors.weight}
              </p>
            ) : null}
          </div>
        </Field>
        {challengeValidation ? (
          <InfoPanel tone={challengeFormInvalid ? "warning" : "surface"}>
            <InfoLine
              label="Challenge ID"
              value={`#${challengeValidation.effectiveChallengeID}`}
            />
            <InfoLine
              label="Runtime network"
              value={`10.80.${challengeValidation.effectiveServiceSubnetOctet}.0/24`}
            />
            <InfoLine
              label="Service port"
              value={challengeValidation.effectiveServicePort}
            />
            <InfoLine
              label="Example team endpoint"
              value={challengeValidation.endpointExample}
            />
            <InfoLine
              label="Effective weight"
              value={challengeValidation.effectiveWeight}
            />
            <InfoLine
              label="Participant source download"
              value={
                sourceBundlePath === ""
                  ? "disabled until a bundle path is set"
                  : sourceBundlePath
              }
            />
            {challengeFormInvalid ? (
              <ul className="space-y-1 text-xs leading-5 text-destructive">
                {challengeValidation.errors.map((error) => (
                  <li key={error}>{error}</li>
                ))}
              </ul>
            ) : (
              <p className="text-xs leading-5 text-muted-foreground">
                This challenge will create one service runtime per team after deployment.
              </p>
            )}
          </InfoPanel>
        ) : null}
      </div>
    );
  }

  return (
    <AppDialog
      open={isOpen}
      onClose={onClose}
      title={title}
      description={
        formMode === "edit"
          ? `Update the fields below and click Update to save changes.`
          : `Fill in the fields below and click Create to add a new ${formEntity ?? "item"}.`
      }
      body={body}
      footer={
        <>
          <Button variant="outline" onClick={onClose} disabled={isSubmitting}>
            Cancel
          </Button>
          <Button
            onClick={onSubmit}
            disabled={isSubmitting || formInvalid}
          >
            {isSubmitting ? (
              <LoaderCircle className="h-4 w-4 animate-spin" />
            ) : null}
            {formMode === "edit" ? "Update" : "Create"}
          </Button>
        </>
      }
    />
  );
}

function DeleteConfirmDialog({
  deleteTarget,
  pendingAction,
  onClose,
  onConfirm,
}: {
  deleteTarget: DeleteTarget | null;
  pendingAction: string | null;
  onClose: () => void;
  onConfirm: () => void;
}): ReactElement {
  const isDeleting =
    deleteTarget !== null &&
    pendingAction === `${deleteTarget.kind}:delete:${deleteTarget.id}`;
  const kindLabel =
    deleteTarget?.kind === "deployment"
      ? "deployment job"
      : deleteTarget?.kind ?? "item";
  const deleteActionLabel =
    deleteTarget?.kind === "deployment"
      ? "Delete Job"
      : `Delete ${deleteTarget?.kind ? deleteTarget.kind.charAt(0).toUpperCase() + deleteTarget.kind.slice(1) : "Item"}`;
  const impactLines =
    deleteTarget?.kind === "team"
      ? [
          "All associated player accounts are removed from the roster.",
          "Published service state owned by this team is removed from organizer views.",
          "This changes scoreboard ownership and team service visibility immediately.",
        ]
      : deleteTarget?.kind === "player"
        ? [
            "The player account is removed from the roster immediately.",
            "Existing WireGuard or SSH access must be reconciled separately after deletion.",
          ]
        : deleteTarget?.kind === "challenge"
          ? [
              "Associated service instances are removed from organizer and participant views.",
              "Future deployments for this challenge must be recreated from the catalog.",
            ]
          : deleteTarget?.kind === "deployment"
            ? [
                "Only the deployment job record is removed.",
                "Already deployed runtimes remain intact.",
              ]
            : [];

  return (
    <AppDialog
      open={deleteTarget !== null}
      onClose={onClose}
      title={`Delete ${kindLabel}`}
      description={`Delete ${kindLabel} "${deleteTarget?.label ?? ""}"? This cannot be undone from the dashboard.`}
      body={
        deleteTarget ? (
          <InfoPanel tone="warning" data-testid="delete-impact-panel">
            <InfoLine label="Target" value={deleteTarget.label} />
            <InfoLine label="Effect" value={`Delete this ${kindLabel} immediately`} />
            {deleteTarget.warning ? (
              <p className="text-sm leading-6 text-muted-foreground">
                {deleteTarget.warning}
              </p>
            ) : null}
            {impactLines.length > 0 ? (
              <ul className="space-y-1 text-xs leading-5 text-foreground">
                {impactLines.map((line) => (
                  <li key={line}>{line}</li>
                ))}
              </ul>
            ) : null}
          </InfoPanel>
        ) : null
      }
      footer={
        <>
          <Button variant="outline" onClick={onClose} disabled={isDeleting}>
            Cancel
          </Button>
          <Button
            className="button-danger-solid"
            onClick={onConfirm}
            disabled={isDeleting}
          >
            {isDeleting ? (
              <LoaderCircle className="h-4 w-4 animate-spin" />
            ) : (
              <Trash2 className="h-4 w-4" />
            )}
            {deleteActionLabel}
          </Button>
        </>
      }
    />
  );
}
