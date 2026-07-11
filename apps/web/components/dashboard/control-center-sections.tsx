"use client";
import type { ReactElement, ReactNode } from "react";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { Download, LoaderCircle, Timer, Calendar, Activity, RefreshCw } from "lucide-react";
import { ScoreboardTable } from "@/components/ui/scoreboard-table";
import { AttackFeedTable } from "@/components/ui/attack-feed-table";

import type {
  AttackEvent,
  AttackFeedPage,
  PlatformOverview,
  ScoreRow,
  ServiceRow,
} from "@/lib/dashboard-types";
import { Badge } from "@/components/ui/badge";
import { AttackMapPanel } from "@/components/ui/attack-map-panel";
import { AttackSliceSummaryGrid } from "@/components/ui/attack-slice-summary";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { AppDialog } from "@/components/ui/app-dialog";
import { EmptyStateText } from "@/components/ui/empty-state";
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
  formatSLAState,
  ServiceSLADetail,
} from "@/components/dashboard/service-sla-detail";
import {
  IssuedRootCredentialBlock,
  type SSHSessionData,
} from "@/components/dashboard/ssh-credential-block";

export type { SSHSessionData };

type AttackFilters = {
  limit: string;
  offset: string;
  attacker: string;
  victim: string;
  service: string;
  tickFrom: string;
  tickTo: string;
};

type ScoreboardPanelProps = {
  scoreRows: ScoreRow[];
  currentTeamName?: string;
  frozen?: boolean;
  freezeAt?: string;
  unfreezeAt?: string;
};

type ServicesPanelProps = {
  rows: ServiceRow[];
  pendingAction: string | null;
  overview: PlatformOverview;
  onSelectPrimaryAction: (service: ServiceRow) => void;
  onRestart: (service: ServiceRow) => void;
  onSelectReset: (service: ServiceRow) => void;
};

type AttacksPanelProps = {
  attackFilters: AttackFilters;
  highlightedAttackIDs: string[];
  attackPage: AttackFeedPage;
  attackLiveMode: boolean;
  pendingAction: string | null;
  onAttackerChange: (value: string) => void;
  onLimitChange: (value: string) => void;
  onOffsetChange: (value: string) => void;
  onApplyFilters: () => void;
  onResetFilters: () => void;
  onServiceChange: (value: string) => void;
  onTickFromChange: (value: string) => void;
  onTickToChange: (value: string) => void;
  onVictimChange: (value: string) => void;
  onPage: (direction: "prev" | "next") => void;
};

type UnlockDialogProps = {
  actionError: string | null;
  open: boolean;
  pendingAction: string | null;
  proof: string;
  target: ServiceRow | null;
  onClose: () => void;
  onConfirm: () => void;
  onProofChange: (value: string) => void;
};

type SSHSessionDialogProps = {
  actionError: string | null;
  issuedSession: SSHSessionData | null;
  open: boolean;
  pendingAction: string | null;
  onClose: () => void;
  onConfirm: () => void;
};

type FactoryResetDialogProps = {
  actionError: string | null;
  open: boolean;
  pendingAction: string | null;
  target: ServiceRow | null;
  onClose: () => void;
  onConfirm: () => void;
};

const serviceStatusTone: Record<ServiceRow["status"], string> = {
  stable: "tone-success",
  warming: "tone-warning",
  degraded: "tone-danger",
};

export function ScoreboardPanel({
  scoreRows,
  currentTeamName,
  frozen,
  freezeAt,
  unfreezeAt,
}: ScoreboardPanelProps): ReactElement {
  return (
    <Card data-testid="participant-scoreboard-card">
      <CardHeader>
        <CardTitle>Scoreboard</CardTitle>
        <CardDescription>
          Team ranking with per-service attack, defense, SLA, and total scores.
        </CardDescription>
      </CardHeader>
      <CardContent className="grid gap-3">
        {frozen && (
          <div
            role="status"
            data-testid="scoreboard-frozen-banner"
            className="tone-warning rounded-md border px-3 py-2 text-sm"
          >
            <span className="font-semibold">Scoreboard frozen.</span> Standings
            are paused
            {freezeAt ? ` as of ${formatFreezeTimestamp(freezeAt)}` : ""}
            {unfreezeAt
              ? ` and resume at ${formatFreezeTimestamp(unfreezeAt)}`
              : " until the organizers lift the freeze"}
            .
          </div>
        )}
        <ScoreboardTable
          scoreRows={scoreRows}
          emptyMessage="No score rows are available yet."
          currentTeamName={currentTeamName}
        />
      </CardContent>
    </Card>
  );
}

function formatFreezeTimestamp(value: string): string {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return value;
  }
  return parsed.toLocaleString();
}

function formatMatchStateLabel(state?: string): string {
  if (!state) {
    return "Inactive";
  }
  const labels: Record<string, string> = {
    not_started: "Not started",
    running: "Running",
    paused: "Paused",
    stopped: "Stopped",
    finished: "Finished",
  };
  const key = state.toLowerCase();
  return labels[key] ?? state.replaceAll("_", " ");
}

function formatCountdown(seconds: number): string {
  if (seconds < 60) {
    return `${seconds}s`;
  }
  const mins = Math.floor(seconds / 60);
  const secs = seconds % 60;
  return `${mins}m ${secs.toString().padStart(2, "0")}s`;
}

function TickIntervalCard({ overview }: { overview: PlatformOverview }): ReactElement {
  const [timeLeft, setTimeLeft] = useState<number | null>(null);
  const [overdue, setOverdue] = useState(false);
  const router = useRouter();

  useEffect(() => {
    if (!overview.nextTickAt) {
      setTimeLeft(null);
      setOverdue(false);
      return;
    }
    const target = new Date(overview.nextTickAt).getTime();
    const intervalMs = (overview.tickInterval ?? 0) * 1000;

    const update = () => {
      const now = Date.now();
      // The boundary has passed but the backend hasn't published the next
      // nextTickAt yet — drive the recovery refresh below.
      setOverdue(now >= target);
      // Project forward by whole intervals so the counter rolls straight over
      // to the next interval instead of freezing at 0s while we wait for the
      // refresh to land the real nextTickAt.
      let projected = target;
      if (intervalMs > 0) {
        while (projected <= now) {
          projected += intervalMs;
        }
      }
      const diff = Math.max(0, Math.round((projected - now) / 1000));
      setTimeLeft(diff);
    };

    update();
    const timer = setInterval(update, 1000);
    return () => clearInterval(timer);
  }, [overview.nextTickAt, overview.tickInterval]);

  useEffect(() => {
    if (!overdue || !overview.nextTickAt) {
      return;
    }

    router.refresh();

    const interval = setInterval(() => {
      router.refresh();
    }, 3500);

    return () => clearInterval(interval);
  }, [overdue, overview.nextTickAt, router]);

  const matchState = overview.matchState?.toLowerCase();
  const isRunning = matchState === "running";
  const isPaused = matchState === "paused";
  const isStopped =
    matchState === "stopped" ||
    matchState === "finished" ||
    matchState === "not_started" ||
    !matchState;
  const stateLabel = formatMatchStateLabel(overview.matchState);
  const stateTone = isPaused
    ? "text-highlight"
    : isRunning
      ? "text-positive"
      : "text-muted-foreground";
  const stateDot = isPaused
    ? "bg-[var(--signal-highlight)]"
    : isRunning
      ? "bg-[var(--signal-positive)] animate-pulse"
      : "bg-muted-foreground/50";

  const countdown = (() => {
    if (isPaused) {
      return { primary: "Suspended", secondary: "Match paused" };
    }
    if (isStopped) {
      return {
        primary: "Idle",
        secondary:
          matchState === "finished"
            ? "Match finished"
            : matchState === "not_started"
              ? "Match not started"
              : "Ticks not running",
      };
    }
    if (timeLeft !== null) {
      return {
        primary: formatCountdown(timeLeft),
        secondary: overview.tickInterval
          ? `Every ${overview.tickInterval}s`
          : undefined,
      };
    }
    return { primary: "—", secondary: undefined };
  })();

  return (
    <section
      className="overflow-hidden rounded-sm border border-border bg-card"
      data-testid="match-progress"
      aria-label="Match progress"
    >
      <div className="flex items-center gap-2 border-b border-border px-3.5 py-2.5">
        <Timer className="size-4 text-muted-foreground" aria-hidden />
        <h2 className="text-sm font-semibold tracking-tight text-foreground">
          Match Progress
        </h2>
      </div>
      <dl className="grid gap-0 sm:grid-cols-2 xl:grid-cols-4">
        <div className="border-b border-border px-3.5 py-3 sm:border-r xl:border-b-0">
          <dt className="flex items-center gap-1.5 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
            <Activity className="size-3.5 shrink-0" aria-hidden />
            Current tick
          </dt>
          <dd className="mt-1.5 font-mono text-xl font-semibold tabular-nums tracking-tight text-foreground">
            {overview.currentTick !== undefined && overview.currentTick !== null
              ? `#${overview.currentTick}`
              : "—"}
          </dd>
        </div>

        <div className="border-b border-border px-3.5 py-3 sm:border-r-0 xl:border-r xl:border-b-0">
          <dt className="flex items-center gap-1.5 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
            <RefreshCw className="size-3.5 shrink-0" aria-hidden />
            Match state
          </dt>
          <dd className={`mt-1.5 flex items-center gap-2 text-sm font-semibold ${stateTone}`}>
            <span className={`inline-block size-2 shrink-0 rounded-full ${stateDot}`} />
            {stateLabel}
          </dd>
        </div>

        <div className="border-b border-border px-3.5 py-3 sm:border-r xl:border-b-0">
          <dt className="flex items-center gap-1.5 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
            <Calendar className="size-3.5 shrink-0" aria-hidden />
            Next tick at
          </dt>
          <dd className="mt-1.5 text-sm font-semibold tabular-nums text-foreground">
            {overview.nextTickAt
              ? new Date(overview.nextTickAt).toLocaleTimeString(undefined, {
                  hour: "2-digit",
                  minute: "2-digit",
                  second: "2-digit",
                })
              : "—"}
          </dd>
        </div>

        <div className="px-3.5 py-3">
          <dt className="flex items-center gap-1.5 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
            <Timer className="size-3.5 shrink-0" aria-hidden />
            Countdown
          </dt>
          <dd className="mt-1.5">
            <span
              className={
                isRunning && timeLeft !== null
                  ? "font-mono text-xl font-semibold tabular-nums tracking-tight text-foreground"
                  : "text-sm font-semibold text-muted-foreground"
              }
            >
              {countdown.primary}
            </span>
            {countdown.secondary ? (
              <span className="mt-0.5 block text-xs font-medium text-muted-foreground">
                {countdown.secondary}
              </span>
            ) : null}
          </dd>
        </div>
      </dl>
    </section>
  );
}

export function ServicesPanel({
  rows,
  pendingAction,
  overview,
  onSelectPrimaryAction,
  onRestart,
  onSelectReset,
}: ServicesPanelProps): ReactElement {
  return (
    <div className="space-y-4">
      <TickIntervalCard overview={overview} />
      
      {rows.length === 0 ? (
        <Card>
          <CardHeader>
            <CardTitle>Services</CardTitle>
            <CardDescription>
              Owned service controls for unlock, root access, restart, and
              factory reset.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <EmptyStateText message="No owned services are available for this team yet." />
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4 xl:grid-cols-2">
          {rows.map((service) => (
            <ServiceCard
              key={service.id}
              pendingAction={pendingAction}
              service={service}
              onRestart={onRestart}
              onSelectPrimaryAction={onSelectPrimaryAction}
              onSelectReset={onSelectReset}
            />
          ))}
        </div>
      )}
    </div>
  );
}

export function AttacksPanel({
  attackFilters,
  highlightedAttackIDs,
  attackPage,
  attackLiveMode,
  pendingAction,
  onAttackerChange,
  onLimitChange,
  onOffsetChange,
  onApplyFilters,
  onResetFilters,
  onServiceChange,
  onTickFromChange,
  onTickToChange,
  onVictimChange,
  onPage,
}: AttacksPanelProps): ReactElement {
  const attackRows = attackPage.items;
  const [visibleRows, setVisibleRows] = useState<AttackEvent[]>(attackRows);

  useEffect(() => {
    setVisibleRows(attackRows);
  }, [attackRows]);

  return (
    <Card>
      <CardHeader>
        <CardTitle>Accepted attacks</CardTitle>
        <CardDescription>
          Filter by team, service, and tick range. The globe and table share the
          same paginated attack slice.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex flex-col gap-3 border-b pb-4 lg:flex-row lg:items-end lg:justify-between">
          <div className="grid gap-3 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-7">
            <Field label="Attacker" htmlFor="attack-attacker">
              <Input
                id="attack-attacker"
                value={attackFilters.attacker}
                onChange={(event) => onAttackerChange(event.target.value)}
                placeholder="Team Alpha"
              />
            </Field>
            <Field label="Victim" htmlFor="attack-victim">
              <Input
                id="attack-victim"
                value={attackFilters.victim}
                onChange={(event) => onVictimChange(event.target.value)}
                placeholder="Team Delta"
              />
            </Field>
            <Field label="Service" htmlFor="attack-service">
              <Input
                id="attack-service"
                value={attackFilters.service}
                onChange={(event) => onServiceChange(event.target.value)}
                placeholder="banking"
              />
            </Field>
            <Field label="Tick From" htmlFor="attack-tick-from">
              <Input
                id="attack-tick-from"
                type="number"
                min="0"
                value={attackFilters.tickFrom}
                onChange={(event) => onTickFromChange(event.target.value)}
                placeholder="240"
              />
            </Field>
            <Field label="Tick To" htmlFor="attack-tick-to">
              <Input
                id="attack-tick-to"
                type="number"
                min="0"
                value={attackFilters.tickTo}
                onChange={(event) => onTickToChange(event.target.value)}
                placeholder="248"
              />
            </Field>
            <Field label="Limit" htmlFor="attack-limit">
              <Input
                id="attack-limit"
                type="number"
                min="1"
                max="200"
                value={attackFilters.limit}
                onChange={(event) => onLimitChange(event.target.value)}
              />
            </Field>
            <Field label="Offset" htmlFor="attack-offset">
              <Input
                id="attack-offset"
                type="number"
                min="0"
                value={attackFilters.offset}
                onChange={(event) => onOffsetChange(event.target.value)}
              />
            </Field>
          </div>
          <div className="flex flex-wrap gap-2">
            <SliceCountBadge
              totalCount={attackPage.total_count}
              visibleCount={visibleRows.length}
            />
            <LiveModeBadge
              filteredLabel="Filtered"
              liveLabel="Live"
              liveMode={attackLiveMode}
            />
          </div>
        </div>
        <PagedFilterActions
          applyLabel="Apply Filters"
          canPageNext={attackPage.has_next}
          canPagePrev={attackPage.has_prev}
          disabled={pendingAction !== null}
          liveMode={attackLiveMode}
          onApply={onApplyFilters}
          onPage={onPage}
          onReset={onResetFilters}
          resetLabel="Reset View"
          showLiveModeBadge={false}
        />
        <AttackSliceSummaryGrid rows={visibleRows} />
        <AttackMapPanel
          attackRows={attackRows}
          description="Accepted submissions rendered as directional team flow for the current paginated slice."
          highlightedAttackIDs={highlightedAttackIDs}
          title="Attack flow"
          onVisibleRowsChange={(rows) => setVisibleRows(rows)}
        />

        {visibleRows.length === 0 ? (
          <EmptyStateText message="No accepted attack events are available for this slice." />
        ) : (
          <div className="space-y-3">
            <div className="flex items-center justify-between gap-3">
              <p className="text-sm font-medium text-foreground">
                Current slice
              </p>
              <p className="text-xs text-muted-foreground">
                Table and globe show the same filtered page.
              </p>
            </div>
            <AttackFeedTable attackRows={visibleRows} />
          </div>
        )}
      </CardContent>
    </Card>
  );
}

export function UnlockServiceDialog({
  actionError,
  open,
  pendingAction,
  proof,
  target,
  onClose,
  onConfirm,
  onProofChange,
}: UnlockDialogProps): ReactElement {
  return (
    <AppDialog
      body={
        <div className="space-y-3">
          {actionError ? <ErrorBanner message={actionError} /> : null}
          <div className="space-y-2">
            <Label htmlFor="unlock-proof">Unlock proof</Label>
            <Input
              id="unlock-proof"
              value={proof}
              onChange={(event) => onProofChange(event.target.value)}
            />
          </div>
          <ServiceTargetBlock target={target} />
        </div>
      }
      description="Submit proof from your own service. Unlock stays active through a factory reset during the same match."
      footer={
        <>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button disabled={pendingAction !== null} onClick={onConfirm}>
            {pendingAction?.startsWith("unlock:") ? (
              <LoaderCircle className="h-4 w-4 animate-spin" />
            ) : null}
            Unlock Service
          </Button>
        </>
      }
      onClose={onClose}
      open={open}
      title="Unlock Service"
    />
  );
}

export function SSHSessionDialog({
  actionError,
  issuedSession,
  open,
  pendingAction,
  onClose,
  onConfirm,
}: SSHSessionDialogProps): ReactElement {
  const hasTriggered = useRef(false);

  useEffect(() => {
    if (open && !issuedSession && !pendingAction && !hasTriggered.current) {
      hasTriggered.current = true;
      onConfirm();
    }
  }, [open, issuedSession, pendingAction, onConfirm]);

  useEffect(() => {
    if (!open) {
      hasTriggered.current = false;
    }
  }, [open]);

  return (
    <AppDialog
      body={
        <div className="space-y-3">
          {actionError ? <ErrorBanner message={actionError} /> : null}
          {pendingAction?.startsWith("ssh:") && !issuedSession ? (
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <LoaderCircle className="h-4 w-4 animate-spin" />
              Loading root access…
            </div>
          ) : null}
          {issuedSession ? (
            <IssuedRootCredentialBlock issuedSession={issuedSession} />
          ) : null}
        </div>
      }
      description="Connect over WireGuard with the stable team root credential, patch live files in the owned container, and use restart or factory reset for recovery."
      footer={
        <>
          <Button variant="outline" onClick={onClose}>
            Close
          </Button>
          {issuedSession ? (
            <Button
              disabled={pendingAction !== null}
              onClick={onConfirm}
              variant="secondary"
            >
              {pendingAction?.startsWith("ssh:") ? (
                <LoaderCircle className="h-4 w-4 animate-spin" />
              ) : null}
              Reapply Credential
            </Button>
          ) : null}
        </>
      }
      onClose={onClose}
      open={open}
      title="SSH Access"
    />
  );
}

export function FactoryResetDialog({
  actionError,
  open,
  pendingAction,
  target,
  onClose,
  onConfirm,
}: FactoryResetDialogProps): ReactElement {
  return (
    <AppDialog
      body={
        <div className="space-y-3">
          {actionError ? <ErrorBanner message={actionError} /> : null}
          <ResetWarningBlock />
        </div>
      }
      description="Recreate the service from the organizer baseline image and discard the current patch state."
      footer={
        <>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button
            disabled={pendingAction !== null || target === null}
            onClick={onConfirm}
          >
            {pendingAction?.startsWith("reset:") ? (
              <LoaderCircle className="h-4 w-4 animate-spin" />
            ) : null}
            Confirm Reset
          </Button>
        </>
      }
      onClose={onClose}
      open={open}
      title="Factory Reset Service"
    />
  );
}

function ErrorBanner({ message }: { message: string }): ReactElement {
  return <StatusBanner message={message} variant="error" />;
}

function ServiceCard({
  pendingAction,
  service,
  onRestart,
  onSelectPrimaryAction,
  onSelectReset,
}: {
  pendingAction: string | null;
  service: ServiceRow;
  onRestart: (service: ServiceRow) => void;
  onSelectPrimaryAction: (service: ServiceRow) => void;
  onSelectReset: (service: ServiceRow) => void;
}): ReactElement {
  return (
    <Card data-testid={`service-card-${service.id}`}>
      <CardHeader>
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <CardTitle>{service.name}</CardTitle>
            <CardDescription>{service.endpoint}</CardDescription>
          </div>
          <div className="flex flex-wrap gap-2">
            {service.maintenance ? (
              <Badge
                className="tone-warning"
                data-testid={`service-maintenance-badge-${service.challengeId}`}
                variant="outline"
              >
                Under maintenance
              </Badge>
            ) : (
              <Badge
                className={serviceStatusTone[service.status]}
                variant="outline"
              >
                {service.status}
              </Badge>
            )}
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <ServiceDetails service={service} />
        <ServiceActionBar
          pendingAction={pendingAction}
          service={service}
          onRestart={onRestart}
          onSelectPrimaryAction={onSelectPrimaryAction}
          onSelectReset={onSelectReset}
        />
      </CardContent>
    </Card>
  );
}

function ServiceDetails({ service }: { service: ServiceRow }): ReactElement {
  return (
    <dl className="grid gap-x-6 gap-y-2 text-sm sm:grid-cols-2">
      <DetailRow label="Challenge" value={`#${service.challengeId}`} />
      <DetailRow
        label="Checker"
        value={
          service.checker === "passing" ? (
            <span className="font-semibold text-positive">passing</span>
          ) : (
            <span className="font-semibold text-negative">warning (failing)</span>
          )
        }
      />
      <DetailRow label="Service state" value={formatSLAState(service)} />
      <DetailRow label="Reset" value={service.resetCooldown} />
      <DetailRow
        label="SSH"
        value={
          service.unlocked ? (
            <span className="font-semibold text-positive">unlocked</span>
          ) : (
            <span className="text-muted-foreground">locked</span>
          )
        }
      />
      <DetailRow label="Last event" value={service.lastEvent} />
      <DetailRow
        label="SLA detail"
        value={<ServiceSLADetail service={service} />}
        wide
      />
      <DetailRow label="Access hint" value={service.sshHint} wide />
    </dl>
  );
}

function ServiceActionBar({
  pendingAction,
  service,
  onRestart,
  onSelectPrimaryAction,
  onSelectReset,
}: {
  pendingAction: string | null;
  service: ServiceRow;
  onRestart: (service: ServiceRow) => void;
  onSelectPrimaryAction: (service: ServiceRow) => void;
  onSelectReset: (service: ServiceRow) => void;
}): ReactElement {
  const isPrimaryPending =
    pendingAction === `unlock:${service.id}` ||
    pendingAction === `ssh:${service.id}`;
  const isRestartPending = pendingAction === `restart:${service.id}`;
  const isResetPending = pendingAction === `reset:${service.id}`;
  const actionsDisabled = pendingAction !== null || service.maintenance;

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap gap-3">
        <Button
          disabled={actionsDisabled}
          onClick={() => onSelectPrimaryAction(service)}
        >
          {isPrimaryPending ? (
            <LoaderCircle className="h-4 w-4 animate-spin" />
          ) : null}
          {service.unlocked ? "SSH Access" : "Unlock Service"}
        </Button>
        <Button
          disabled={actionsDisabled}
          onClick={() => onRestart(service)}
          variant="secondary"
        >
          {isRestartPending ? (
            <LoaderCircle className="h-4 w-4 animate-spin" />
          ) : null}
          Restart
        </Button>
        {service.hasSourceDownload && !service.maintenance ? (
          <Button asChild variant="outline">
            <a
              href={`/api/platform/challenges/${service.challengeId}/source`}
              download
            >
              <Download className="h-4 w-4" />
              Download Source
            </a>
          </Button>
        ) : null}
        <Button
          disabled={actionsDisabled}
          onClick={() => onSelectReset(service)}
          variant="outline"
        >
          {isResetPending ? (
            <LoaderCircle className="h-4 w-4 animate-spin" />
          ) : null}
          Factory Reset
        </Button>
      </div>
      {service.maintenance ? (
        <p className="text-sm text-muted-foreground">
          Actions unavailable while this challenge is under maintenance.
        </p>
      ) : null}
    </div>
  );
}

function ServiceTargetBlock({
  target,
}: {
  target: ServiceRow | null;
}): ReactElement {
  return (
    <InfoPanel compact>
      <InfoLine
        label="Target"
        value={target?.name ?? "unknown"}
        valueClassName="font-semibold"
      />
    </InfoPanel>
  );
}


function ResetWarningBlock(): ReactElement {
  return (
    <StatusBanner
      message="Unlock remains preserved for this team and service during the same match."
      variant="warning"
    />
  );
}

function DetailRow({
  label,
  value,
  wide = false,
}: {
  label: string;
  value: ReactNode;
  wide?: boolean;
}): ReactElement {
  return (
    <div className={wide ? "sm:col-span-2" : undefined}>
      <dt className="text-xs font-medium text-muted-foreground">{label}</dt>
      <dd className="mt-1 text-sm">{value}</dd>
    </div>
  );
}

function Field({
  label,
  htmlFor,
  children,
}: {
  label: string;
  htmlFor: string;
  children: ReactNode;
}): ReactElement {
  return (
    <div className="space-y-2">
      <Label htmlFor={htmlFor}>{label}</Label>
      {children}
    </div>
  );
}
