"use client";
import type { ReactElement, ReactNode } from "react";
import { useRouter } from "next/navigation";

import { nextClockOffset, overdueRefreshDelayMs } from "@/lib/match-clock";
import { formatMatchStateLabel } from "@/lib/match-state";
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
import {
  buildAttackFilterOptions,
  hasActiveAttackFilters,
  isSameAttackView,
} from "@/lib/dashboard-utils";
import {
  cooldownMessage,
  type ServiceActionFeedback,
} from "@/lib/service-actions";
import {
  describeService,
  serviceCooldownLabel,
  serviceLockBadgeLabel,
} from "@/lib/service-view-model";
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
import { formatIndonesianDate, formatWibTime } from "@/lib/date-format";
import { ChallengeCategoryIcon } from "@/components/ui/challenge-category-icon";
import { ServiceConnectionLine } from "@/components/ui/service-connection";
import {
  ATTACKS_UNAVAILABLE,
  DataUnavailable,
  SCORES_UNAVAILABLE,
} from "@/components/ui/data-unavailable";
import { EmptyStateText } from "@/components/ui/empty-state";
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

export type AttackFilters = {
  limit: string;
  offset: string;
  attacker: string;
  victim: string;
  service: string;
  tickFrom: string;
  tickTo: string;
};

export type ScoreboardPanelProps = {
  scoreRows: ScoreRow[];
  currentTeamName?: string;
  frozen?: boolean;
  freezeAt?: string;
  unfreezeAt?: string;
  /** The scores request failed and nothing has arrived since. */
  unavailable?: boolean;
};

export type ServicesPanelProps = {
  rows: ServiceRow[];
  pendingAction: string | null;
  overview: PlatformOverview;
  serviceFeedback: (service: ServiceRow) => ServiceActionFeedback;
  onSelectPrimaryAction: (service: ServiceRow) => void;
  onRestart: (service: ServiceRow) => void;
  onSelectReset: (service: ServiceRow) => void;
};

export type AttacksPanelProps = {
  /** What is selected in the form, which may not be applied yet. */
  attackFilters: AttackFilters;
  /** What the rows on screen were loaded with. Defaults to attackFilters. */
  appliedFilters?: AttackFilters;
  /** Why the last Apply, Reset or page change failed. The rows did not change. */
  feedError?: string | null;
  highlightedAttackIDs: string[];
  attackPage: AttackFeedPage;
  attackLiveMode: boolean;
  /** The attack feed request failed and nothing has arrived since. */
  unavailable?: boolean;
  pendingAction: string | null;
  currentTeamName?: string;
  serviceOptions?: string[];
  teamOptions?: string[];
  onAttackerChange: (value: string) => void;
  onLimitChange: (value: string) => void;
  onApplyFilters: () => void;
  onResetFilters: () => void;
  onServiceChange: (value: string) => void;
  onTickFromChange: (value: string) => void;
  onTickToChange: (value: string) => void;
  onVictimChange: (value: string) => void;
  onPage: (direction: "prev" | "next") => void;
};

const selectClassName =
  "flex h-10 w-full rounded-sm border border-input bg-card px-3 text-sm outline-none transition focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background";

export type SSHSessionDialogProps = {
  actionError: string | null;
  issuedSession: SSHSessionData | null;
  open: boolean;
  pendingAction: string | null;
  /** Set once the pending action has run unusually long. */
  pendingNote?: string | null;
  onClose: () => void;
  onConfirm: () => void;
};

export type FactoryResetDialogProps = {
  actionError: string | null;
  cooldownSeconds: number;
  open: boolean;
  pendingAction: string | null;
  /** Set once the pending action has run unusually long. */
  pendingNote?: string | null;
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
  unavailable,
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
            <span className="font-semibold">Scoreboard frozen.</span>
            {freezeAt ? ` Snapshot as of ${formatIndonesianDate(freezeAt)}.` : ""}
          </div>
        )}
        {unavailable ? (
          <DataUnavailable {...SCORES_UNAVAILABLE} />
        ) : (
          <ScoreboardTable
            scoreRows={scoreRows}
            emptyMessage="No score rows are available yet."
            currentTeamName={currentTeamName}
          />
        )}
      </CardContent>
    </Card>
  );
}

function formatCountdown(seconds: number): string {
  if (seconds < 60) {
    return `${seconds}s`;
  }
  const mins = Math.floor(seconds / 60);
  const secs = seconds % 60;
  return `${mins}m ${secs.toString().padStart(2, "0")}s`;
}

export type MatchClock = {
  currentTickLabel: string;
  stateLabel: string;
  stateKind: "running" | "paused" | "idle";
  nextTickLabel: string;
  countdown: { primary: string; secondary?: string };
  /** True while a live countdown is ticking (running match, known next tick). */
  counting: boolean;
  timeLeft: number | null;
};

/**
 * Live tick clock for the participant views: countdown to the next tick,
 * rolled forward by whole intervals, plus a router refresh while the backend
 * catches up after a boundary. Shared by the stock card and themed clocks.
 */
export function useMatchClock(overview: PlatformOverview): MatchClock {
  const [timeLeft, setTimeLeft] = useState<number | null>(null);
  const [overdue, setOverdue] = useState(false);
  const router = useRouter();
  // Participants' laptops are often minutes off, so count against server time.
  const clockOffset = useRef<number | null>(null);

  useEffect(() => {
    if (overview.serverTime !== undefined) {
      clockOffset.current = nextClockOffset(clockOffset.current, overview.serverTime - Date.now());
    }
  }, [overview.serverTime]);

  useEffect(() => {
    if (!overview.nextTickAt) {
      setTimeLeft(null);
      setOverdue(false);
      return;
    }
    const target = new Date(overview.nextTickAt).getTime();
    const intervalMs = (overview.tickInterval ?? 0) * 1000;

    const update = () => {
      const now = Date.now() + (clockOffset.current ?? 0);
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

    // A stalled scheduler can stay late for a long time, so ease off.
    let attempt = 0;
    let timer: ReturnType<typeof setTimeout>;
    const scheduleRefresh = () => {
      timer = setTimeout(() => {
        router.refresh();
        attempt += 1;
        scheduleRefresh();
      }, overdueRefreshDelayMs(attempt));
    };
    scheduleRefresh();

    return () => clearTimeout(timer);
  }, [overdue, overview.nextTickAt, router]);

  const matchState = overview.matchState?.toLowerCase();
  const isRunning = matchState === "running";
  const isPaused = matchState === "paused";
  const isStopped =
    matchState === "stopped" ||
    matchState === "finished" ||
    matchState === "not_started" ||
    !matchState;
  // No state because the request failed is different from a match that is idle.
  const stateUnknown = !matchState && overview.source === "degraded";
  const stateLabel = stateUnknown ? "Unavailable" : formatMatchStateLabel(overview.matchState);

  const countdown = (() => {
    if (stateUnknown) {
      return { primary: "—", secondary: "Match status unavailable" };
    }
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

  return {
    currentTickLabel:
      overview.currentTick !== undefined && overview.currentTick !== null
        ? `#${overview.currentTick}`
        : "—",
    stateLabel,
    stateKind: isRunning ? "running" : isPaused ? "paused" : "idle",
    nextTickLabel: overview.nextTickAt
      ? formatWibTime(overview.nextTickAt)
      : "—",
    countdown,
    counting: isRunning && timeLeft !== null,
    timeLeft,
  };
}

export function TickIntervalCard({ overview }: { overview: PlatformOverview }): ReactElement {
  const clock = useMatchClock(overview);
  const isPaused = clock.stateKind === "paused";
  const isRunning = clock.stateKind === "running";
  const stateTone = isPaused
    ? "text-highlight"
    : isRunning
      ? "text-positive"
      : "text-muted-foreground";
  const stateDot = isPaused
    ? "bg-highlight"
    : isRunning
      ? "bg-positive animate-pulse"
      : "bg-muted-foreground/50";
  const { stateLabel, countdown } = clock;
  return (
    <section
      data-slot="card"
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
            {clock.currentTickLabel}
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
            {clock.nextTickLabel}
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
                clock.counting
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
  serviceFeedback,
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
              Owned service controls for SSH access, restart, and
              factory reset.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <EmptyStateText
              message={
                overview.servicesUnavailable
                  ? "Your services can't be loaded right now. This doesn't mean they were removed. They'll return once the platform API responds."
                  : "No services are assigned to this team yet. This page refreshes automatically after an organizer deploys a challenge; contact an organizer if the match has already started."
              }
            />
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4 xl:grid-cols-2">
          {rows.map((service) => (
            <ServiceCard
              key={service.id}
              feedback={serviceFeedback(service)}
              matchState={overview.matchState}
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
  appliedFilters = attackFilters,
  feedError,
  highlightedAttackIDs,
  attackPage,
  attackLiveMode,
  unavailable,
  pendingAction,
  currentTeamName,
  serviceOptions = [],
  teamOptions = [],
  onAttackerChange,
  onLimitChange,
  onApplyFilters,
  onResetFilters,
  onServiceChange,
  onTickFromChange,
  onTickToChange,
  onVictimChange,
  onPage,
}: AttacksPanelProps): ReactElement {
  const attackRows = attackPage.items;
  const latestTick = attackRows.reduce((value, row) => Math.max(value, row.tick), 0);
  const filtersApplied = hasActiveAttackFilters(appliedFilters);
  const selectionPending = !isSameAttackView(attackFilters, appliedFilters);
  const [visibleRows, setVisibleRows] = useState<AttackEvent[]>(attackRows);
  const attackerChoices = buildAttackFilterOptions(
    [
      ...teamOptions,
      ...(currentTeamName ? [currentTeamName] : []),
      ...attackRows.map((row) => row.attacker),
    ],
    attackFilters.attacker,
  );
  const victimChoices = buildAttackFilterOptions(
    [
      ...teamOptions,
      ...(currentTeamName ? [currentTeamName] : []),
      ...attackRows.map((row) => row.victim),
    ],
    attackFilters.victim,
  );
  const serviceChoices = buildAttackFilterOptions(
    [...serviceOptions, ...attackRows.map((row) => row.service)],
    attackFilters.service,
  );

  useEffect(() => {
    setVisibleRows(attackRows);
  }, [attackRows]);

  // Filters and the globe have nothing to act on until the feed loads.
  if (unavailable) {
    return (
      <Card>
        <CardHeader>
          <CardTitle as="h2">Accepted attacks</CardTitle>
        </CardHeader>
        <CardContent>
          <DataUnavailable {...ATTACKS_UNAVAILABLE} />
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle as="h2">Accepted attacks</CardTitle>
        <CardDescription>
          Filter by team, service, and tick range. The globe and table share the
          same paginated attack slice.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <section className="grid gap-3 border-b pb-4" aria-labelledby="attack-filter-heading">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <h3 id="attack-filter-heading" className="text-sm font-medium text-foreground">Tactical view</h3>
              <p className="text-xs text-muted-foreground">Start with the question you need to answer, then refine only if necessary.</p>
            </div>
            <div className="flex flex-wrap gap-2">
              {currentTeamName ? (
                <>
                  <Button size="sm" variant="outline" onClick={() => onVictimChange(currentTeamName)}>Against us</Button>
                  <Button size="sm" variant="outline" onClick={() => onAttackerChange(currentTeamName)}>By us</Button>
                </>
              ) : null}
              {latestTick > 0 ? (
                <Button size="sm" variant="outline" onClick={() => {
                  onTickFromChange(String(latestTick));
                  onTickToChange(String(latestTick));
                }}>Current tick</Button>
              ) : null}
            </div>
          </div>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            <Field label="Attacker" htmlFor="attack-attacker">
              <select
                id="attack-attacker"
                className={selectClassName}
                value={attackFilters.attacker}
                onChange={(event) => onAttackerChange(event.target.value)}
              >
                <option value="">Any attacker</option>
                {attackerChoices.map((name) => (
                  <option key={name} value={name}>
                    {name}
                  </option>
                ))}
              </select>
            </Field>
            <Field label="Victim" htmlFor="attack-victim">
              <select
                id="attack-victim"
                className={selectClassName}
                value={attackFilters.victim}
                onChange={(event) => onVictimChange(event.target.value)}
              >
                <option value="">Any victim</option>
                {victimChoices.map((name) => (
                  <option key={name} value={name}>
                    {name}
                  </option>
                ))}
              </select>
            </Field>
            <Field label="Service" htmlFor="attack-service">
              <select
                id="attack-service"
                className={selectClassName}
                value={attackFilters.service}
                onChange={(event) => onServiceChange(event.target.value)}
              >
                <option value="">Any service</option>
                {serviceChoices.map((name) => (
                  <option key={name} value={name}>
                    {name}
                  </option>
                ))}
              </select>
            </Field>
          </div>
          {selectionPending ? (
            <p className="text-xs text-muted-foreground" role="status">
              Selected view: {formatAttackFilterSummary(attackFilters) || "all attacks"}. Choose Apply view to load it.
            </p>
          ) : filtersApplied ? (
            <p className="text-xs text-muted-foreground" role="status">
              Showing {formatAttackFilterSummary(appliedFilters)}.
            </p>
          ) : null}
          <details data-slot="well" className="rounded-sm border border-border/70 bg-muted/10 p-3">
            <summary className="cursor-pointer text-sm font-medium text-foreground">Advanced filters</summary>
            <div className="mt-3 grid gap-3 sm:grid-cols-3">
              <Field label="Tick from" htmlFor="attack-tick-from"><Input id="attack-tick-from" type="number" min="0" value={attackFilters.tickFrom} onChange={(event) => onTickFromChange(event.target.value)} placeholder="240" /></Field>
              <Field label="Tick to" htmlFor="attack-tick-to"><Input id="attack-tick-to" type="number" min="0" value={attackFilters.tickTo} onChange={(event) => onTickToChange(event.target.value)} placeholder="248" /></Field>
              <Field label="Rows per page" htmlFor="attack-limit"><Input id="attack-limit" type="number" min="1" max="200" value={attackFilters.limit} onChange={(event) => onLimitChange(event.target.value)} /></Field>
            </div>
          </details>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <PagedFilterActions
              applyLabel="Apply view"
              canPageNext={attackPage.has_next}
              canPagePrev={attackPage.has_prev}
              disabled={pendingAction !== null}
              liveMode={attackLiveMode}
              onApply={onApplyFilters}
              onPage={onPage}
              onReset={onResetFilters}
              resetLabel="Reset view"
              showLiveModeBadge={false}
            />
            <div className="flex flex-wrap gap-2">
            <SliceCountBadge
              totalCount={attackPage.total_count}
              visibleCount={visibleRows.length}
              offset={attackPage.offset ?? 0}
            />
            <LiveModeBadge
              filteredLabel="Filtered"
              liveLabel="Live"
              liveMode={attackLiveMode}
            />
            </div>
          </div>
          {feedError ? (
            <div data-testid="attack-feed-error">
              <StatusBanner
                message={`That view couldn't be loaded. ${feedError.replace(/[.!?]?$/, ".")} The results below are from the last view that loaded.`}
                variant="error"
              />
            </div>
          ) : null}
        </section>
        <AttackSliceSummaryGrid rows={visibleRows} />
        <AttackMapPanel
          attackRows={attackRows}
          firstBloods={attackPage.first_bloods}
          description="Accepted submissions rendered as directional team flow for the current paginated slice."
          highlightedAttackIDs={highlightedAttackIDs}
          teamRoster={teamOptions}
          title="Attack flow"
          onVisibleRowsChange={(rows) => setVisibleRows(rows)}
        />

        {visibleRows.length === 0 ? (
          <div data-slot="well" className="grid justify-items-start gap-2 rounded-sm border border-border/70 bg-muted/10 p-4">
            <p className="text-sm font-medium text-foreground">No accepted attacks in this view</p>
            <EmptyStateText message={filtersApplied ? 'The active team, service, or tick filters exclude every accepted attack. Reset the view to return to the live feed.' : 'Accepted attacks will appear here automatically after a valid flag is submitted.'} />
            {filtersApplied ? <Button size="sm" variant="outline" onClick={onResetFilters}>Reset view</Button> : null}
          </div>
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

function formatAttackFilterSummary(filters: AttackFilters): string {
  const parts = [
    filters.attacker.trim() ? `attacker ${filters.attacker.trim()}` : null,
    filters.victim.trim() ? `victim ${filters.victim.trim()}` : null,
    filters.service.trim() ? `service ${filters.service.trim()}` : null,
    filters.tickFrom.trim() || filters.tickTo.trim()
      ? `ticks ${filters.tickFrom.trim() || 'start'}–${filters.tickTo.trim() || 'latest'}`
      : null,
  ].filter((value): value is string => value !== null);

  return parts.join(', ');
}

export function SSHSessionDialog({
  actionError,
  issuedSession,
  open,
  pendingAction,
  pendingNote,
  onClose,
  onConfirm,
}: SSHSessionDialogProps): ReactElement {
  // The session request on open is issued by ControlCenter
  // (useAutoRequestSSHSession), so themed dialogs get it for free.
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
          {pendingNote && pendingAction?.startsWith("ssh:") ? (
            <StatusBanner message={pendingNote} variant="warning" />
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
  cooldownSeconds,
  open,
  pendingAction,
  pendingNote,
  target,
  onClose,
  onConfirm,
}: FactoryResetDialogProps): ReactElement {
  return (
    <AppDialog
      body={
        <div className="space-y-3">
          {actionError ? (
            <ErrorBanner message={actionError} />
          ) : cooldownSeconds > 0 ? (
            <StatusBanner
              message={cooldownMessage("reset", cooldownSeconds)}
              variant="warning"
            />
          ) : null}
          {pendingNote && pendingAction?.startsWith("reset:") ? (
            <StatusBanner message={pendingNote} variant="warning" />
          ) : null}
          <ResetWarningBlock />
        </div>
      }
      description={
        <>
          {target ? (
            <span className="mb-2 block wrap-anywhere">
              <strong className="block text-foreground">{target.name}</strong>
              <span className="block font-mono text-sm">{target.endpoint}</span>
            </span>
          ) : null}
          Recreate the service from the organizer baseline image and discard the current patch state.
        </>
      }
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

/** Resolve lock badge from API lock_reason, match state, or service text fields. */
function ServiceCard({
  feedback,
  matchState,
  pendingAction,
  service,
  onRestart,
  onSelectPrimaryAction,
  onSelectReset,
}: {
  feedback: ServiceActionFeedback;
  matchState?: string | null;
  pendingAction: string | null;
  service: ServiceRow;
  onRestart: (service: ServiceRow) => void;
  onSelectPrimaryAction: (service: ServiceRow) => void;
  onSelectReset: (service: ServiceRow) => void;
}): ReactElement {
  const view = describeService(service, { feedback, pendingAction, matchState });
  return (
    <Card data-testid={`service-card-${service.id}`}>
      <CardHeader>
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="flex min-w-0 flex-[1_1_12rem] items-start gap-3">
            <span
              data-slot="category-icon"
              className="grid size-10 shrink-0 place-items-center rounded-sm border border-border bg-muted/40 text-foreground"
            >
              <ChallengeCategoryIcon category={service.category} className="size-5" />
            </span>
            <div className="min-w-0">
              <CardTitle className="wrap-anywhere">{service.name}</CardTitle>
              <CardDescription data-testid={`service-category-${service.id}`}>
                {view.categoryLabel}
              </CardDescription>
            </div>
          </div>
          <div className="flex flex-wrap gap-2">
            {service.maintenance ? (
              <Badge
                className="tone-warning"
                data-testid={`service-maintenance-badge-${service.challengeId}`}
                variant="outline"
              >
                {serviceLockBadgeLabel(service, matchState)}
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
        <ServiceConnectionLine
          connection={view.connection}
          endpoint={service.endpoint}
          testId={`service-connection-${service.id}`}
        />
        <ServiceDetails feedback={feedback} service={service} />
        <ServiceActionBar
          feedback={feedback}
          matchState={matchState}
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

function ServiceDetails({
  feedback,
  service,
}: {
  feedback: ServiceActionFeedback;
  service: ServiceRow;
}): ReactElement {
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
      <DetailRow label="Reset" value={serviceCooldownLabel(service, feedback)} />
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
  feedback,
  matchState,
  pendingAction,
  service,
  onRestart,
  onSelectPrimaryAction,
  onSelectReset,
}: {
  feedback: ServiceActionFeedback;
  matchState?: string | null;
  pendingAction: string | null;
  service: ServiceRow;
  onRestart: (service: ServiceRow) => void;
  onSelectPrimaryAction: (service: ServiceRow) => void;
  onSelectReset: (service: ServiceRow) => void;
}): ReactElement {
  const view = describeService(service, { feedback, pendingAction, matchState });

  return (
    <div className="space-y-3">
      {view.notice ? (
        <div data-testid={`service-action-notice-${service.id}`}>
          <StatusBanner
            message={view.notice.message}
            variant={view.notice.variant}
          />
        </div>
      ) : null}
      {view.unlockHint ? <p className="text-sm text-muted-foreground">{view.unlockHint}</p> : null}
      <div className="flex flex-wrap gap-3">
        <Button
          disabled={view.ssh.disabled}
          holdFocus={view.ssh.pending}
          onClick={() => onSelectPrimaryAction(service)}
        >
          {view.ssh.pending ? (
            <LoaderCircle className="h-4 w-4 animate-spin" />
          ) : null}
          {view.ssh.label}
        </Button>
        <Button
          disabled={view.restart.disabled}
          holdFocus={view.restart.pending}
          onClick={() => onRestart(service)}
          variant="secondary"
        >
          {view.restart.pending ? (
            <LoaderCircle className="h-4 w-4 animate-spin" />
          ) : null}
          Restart
          {view.restart.cooldownSeconds > 0 ? (
            <span className="tabular-nums text-muted-foreground">
              {` ${view.restart.cooldownSeconds}s`}
            </span>
          ) : null}
        </Button>
        {view.sourceHref ? (
          <Button asChild variant="outline">
            <a href={view.sourceHref} download>
              <Download className="h-4 w-4" />
              Download Source
            </a>
          </Button>
        ) : null}
        <Button
          disabled={view.reset.disabled}
          holdFocus={view.reset.pending}
          onClick={() => onSelectReset(service)}
          variant="outline"
        >
          {view.reset.pending ? (
            <LoaderCircle className="h-4 w-4 animate-spin" />
          ) : null}
          Factory Reset
          {view.reset.cooldownSeconds > 0 ? (
            <span className="tabular-nums text-muted-foreground">
              {` ${view.reset.cooldownSeconds}s`}
            </span>
          ) : null}
        </Button>
      </div>
      {view.lock ? (
        <p className="text-sm text-muted-foreground">{view.lock.help}</p>
      ) : null}
    </div>
  );
}

export function ResetWarningBlock(): ReactElement {
  return (
    <StatusBanner
      message="Factory reset deletes stored flags. If a flag is outstanding when state removal begins, its tick SLA is forfeited and a severe defense penalty applies even if recovery fails. SSH unlock remains preserved."
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
