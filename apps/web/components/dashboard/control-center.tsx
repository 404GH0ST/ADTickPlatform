"use client";

import { useEffect, useRef, useState, type ReactElement } from "react";
import type { PlatformOverview } from "@/lib/dashboard-types";

import {
  AttacksPanel,
  FactoryResetDialog,
  ScoreboardPanel,
  ServicesPanel,
  SSHSessionDialog,
} from "@/components/dashboard/control-center-sections";
import { FlagSubmitPanel } from "@/components/dashboard/flag-submit-panel";
import { ThemeSlot } from "@/components/theme/theme-slot";
import { StaleDataNotice } from "@/components/ui/data-unavailable";
import type { ServicesViewProps } from "@/lib/theme-contract";
import {
  type ControlCenterOptions,
  type ControlCenterState,
  useControlCenter,
} from "@/components/dashboard/use-control-center";
import type { ServiceActionFeedback } from "@/lib/service-actions";

/**
 * During an outage the refreshed overview can't say who is signed in. The
 * person hasn't changed, so keep their identity and keep the page's controls
 * (above all the flag form and its text) mounted. The API still checks every
 * action, and live match facts stay unknown rather than showing old values.
 */
function useOverviewThroughOutage(latest: PlatformOverview): PlatformOverview {
  const [lastGood, setLastGood] = useState(latest.sessionUnavailable ? null : latest);
  if (!latest.sessionUnavailable && lastGood !== latest) {
    setLastGood(latest);
  }
  // A failed freeze check reads as "not frozen", which would strip the label
  // from a frozen board that is still on screen.
  const [lastFreeze, setLastFreeze] = useState(latest.freezeUnavailable ? null : latest);
  if (!latest.freezeUnavailable && lastFreeze !== latest) {
    setLastFreeze(latest);
  }
  const overview =
    latest.freezeUnavailable && lastFreeze
      ? {
          ...latest,
          scoreboardFrozen: lastFreeze.scoreboardFrozen,
          scoreboardFreezeAt: lastFreeze.scoreboardFreezeAt,
          scoreboardUnfreezeAt: lastFreeze.scoreboardUnfreezeAt,
        }
      : latest;
  if (!overview.sessionUnavailable || !lastGood) {
    return overview;
  }
  return {
    ...overview,
    authenticated: lastGood.authenticated,
    teamID: lastGood.teamID,
    playerID: lastGood.playerID,
    teamName: lastGood.teamName,
    teamContactEmail: lastGood.teamContactEmail,
    displayName: lastGood.displayName,
    email: lastGood.email,
    role: lastGood.role,
    acceptingSubmissions: lastGood.acceptingSubmissions,
  };
}

export function ControlCenter({
  scores,
  services,
  attackPage,
  realtimeBaseUrl,
  initialTab = "scoreboard",
  currentTeamName,
  overview: latestOverview,
}: ControlCenterOptions & {
  initialTab?: "scoreboard" | "services" | "attacks";
  currentTeamName?: string;
  overview: PlatformOverview;
}): ReactElement {
  const overview = useOverviewThroughOutage(latestOverview);
  const state = useControlCenter({
    attackPage,
    realtimeBaseUrl,
    scores,
    services,
    scoresUnavailable: overview.scoresUnavailable,
    servicesUnavailable: overview.servicesUnavailable,
    attacksUnavailable: overview.attacksUnavailable,
  });
  const showingLastData =
    (initialTab === "scoreboard" && overview.scoresUnavailable && !state.scoresLive && state.scoreRows.length > 0) ||
    (initialTab === "services" && overview.servicesUnavailable && state.rows.length > 0) ||
    (initialTab === "attacks" && overview.attacksUnavailable && !state.attacksLive && state.attackPageState.items.length > 0);
  useAutoRequestSSHSession({
    open: state.sessionTarget !== null,
    issued: state.issuedSession !== null,
    pending: state.pendingAction !== null,
    request: state.requestSSHSession,
  });

  return (
    <div className="grid grid-cols-1 gap-4">
      {showingLastData ? <StaleDataNotice /> : null}
      {renderControlCenterPanel({
        attackLiveMode: state.attackLiveMode,
        attackPage: state.attackPageState,
        attackFilters: state.attackFilters,
        appliedAttackFilters: state.appliedAttackFilters,
        attackFeedError: state.attackFeedError,
        highlightedAttackIDs: state.highlightedAttackIDs,
        initialTab,
        pendingAction: state.pendingAction,
        rows: state.rows,
        scoreRows: state.scoreRows,
        serviceFeedback: state.serviceFeedback,
        currentTeamName,
        overview,
        onApplyAttackFilters: () => {
          void state.applyAttackFilters();
        },
        onPageAttackFeed: (direction) => {
          void state.pageAttackFeed(direction);
        },
        onRequestRestart: (service) => {
          void state.requestRestart(service);
        },
        onResetAttackFilters: () => {
          void state.resetAttackFilters();
        },
        onSelectPrimaryAction: state.selectPrimaryAction,
        onSelectResetTarget: state.selectResetTarget,
        onSetAttackAttacker: state.setAttackAttacker,
        onSetAttackLimit: state.setAttackLimit,
        onSetAttackService: state.setAttackService,
        onSetAttackTickFrom: state.setAttackTickFrom,
        onSetAttackTickTo: state.setAttackTickTo,
        onSetAttackVictim: state.setAttackVictim,
      })}

      <ThemeSlot
        name="SSHSessionDialog"
        fallback={SSHSessionDialog}
        props={{
          actionError: state.actionError,
          issuedSession: state.issuedSession,
          open: state.sessionTarget !== null,
          pendingAction: state.pendingAction,
          pendingNote: state.pendingNote,
          onClose: state.closeSSHSessionDialog,
          onConfirm: () => {
            void state.requestSSHSession();
          },
        }}
      />
      <ThemeSlot
        name="FactoryResetDialog"
        fallback={FactoryResetDialog}
        props={{
          actionError: state.actionError,
          cooldownSeconds: state.resetTarget
            ? state.serviceFeedback(state.resetTarget).resetCooldown
            : 0,
          open: state.resetTarget !== null,
          pendingAction: state.pendingAction,
          pendingNote: state.pendingNote,
          target: state.resetTarget,
          onClose: state.closeFactoryResetDialog,
          onConfirm: () => {
            void state.requestFactoryReset();
          },
        }}
      />
    </div>
  );
}

function renderControlCenterPanel({
  attackLiveMode,
  attackPage,
  attackFilters,
  appliedAttackFilters,
  attackFeedError,
  highlightedAttackIDs,
  initialTab,
  pendingAction,
  rows,
  scoreRows,
  serviceFeedback,
  onApplyAttackFilters,
  onPageAttackFeed,
  onRequestRestart,
  onResetAttackFilters,
  onSelectPrimaryAction,
  onSelectResetTarget,
  onSetAttackAttacker,
  onSetAttackLimit,
  onSetAttackService,
  onSetAttackTickFrom,
  onSetAttackTickTo,
  onSetAttackVictim,
  currentTeamName,
  overview,
}: {
  attackLiveMode: boolean;
  attackPage: ControlCenterOptions["attackPage"];
  attackFilters: {
    limit: string;
    offset: string;
    attacker: string;
    victim: string;
    service: string;
    tickFrom: string;
    tickTo: string;
  };
  appliedAttackFilters: ControlCenterState["appliedAttackFilters"];
  attackFeedError: string | null;
  highlightedAttackIDs: string[];
  initialTab: "scoreboard" | "services" | "attacks";
  pendingAction: string | null;
  rows: ControlCenterOptions["services"];
  scoreRows: ControlCenterOptions["scores"];
  serviceFeedback: (
    service: ControlCenterOptions["services"][number],
  ) => ServiceActionFeedback;
  onApplyAttackFilters: () => void;
  onPageAttackFeed: (direction: "prev" | "next") => void;
  onRequestRestart: (service: ControlCenterOptions["services"][number]) => void;
  onResetAttackFilters: () => void;
  onSelectPrimaryAction: (
    service: ControlCenterOptions["services"][number],
  ) => void;
  onSelectResetTarget: (
    service: ControlCenterOptions["services"][number],
  ) => void;
  onSetAttackAttacker: (value: string) => void;
  onSetAttackLimit: (value: string) => void;
  onSetAttackService: (value: string) => void;
  onSetAttackTickFrom: (value: string) => void;
  onSetAttackTickTo: (value: string) => void;
  onSetAttackVictim: (value: string) => void;
  currentTeamName?: string;
  overview: PlatformOverview;
}): ReactElement {
  if (initialTab === "scoreboard") {
    return (
      <ThemeSlot
        name="Scoreboard"
        fallback={ScoreboardPanel}
        props={{
          scoreRows,
          currentTeamName,
          frozen: overview.scoreboardFrozen,
          freezeAt: overview.scoreboardFreezeAt,
          unfreezeAt: overview.scoreboardUnfreezeAt,
          // Live updates can fill the board after a failed first load.
          unavailable: Boolean(overview.scoresUnavailable) && scoreRows.length === 0,
        }}
      />
    );
  }

  if (initialTab === "services") {
    const showFlagSubmit =
      overview.authenticated &&
      overview.role !== "organizer" &&
      (overview.teamID ?? 0) > 0;
    // Keyed so a different account gets a fresh form, never the last one's text.
    const flagDraftOwner = `${overview.teamID}:${overview.playerID}`;
    return (
      <ThemeSlot
        name="Services"
        fallback={DefaultServicesView}
        props={{
          flagSubmit: showFlagSubmit ? (
            <FlagSubmitPanel
              key={flagDraftOwner}
              acceptingSubmissions={overview.acceptingSubmissions}
              draftOwner={flagDraftOwner}
            />
          ) : null,
          rows,
          pendingAction,
          overview,
          serviceFeedback,
          onSelectPrimaryAction,
          onRestart: onRequestRestart,
          onSelectReset: onSelectResetTarget,
        }}
      />
    );
  }

  const teamOptions = scoreRows.map((row) => row.team);
  const serviceOptions = rows.map((row) => row.name);

  return (
    <ThemeSlot
      name="Attacks"
      fallback={AttacksPanel}
      props={{
        attackFilters,
        appliedFilters: appliedAttackFilters,
        feedError: attackFeedError,
        highlightedAttackIDs,
        attackPage,
        attackLiveMode,
        unavailable: Boolean(overview.attacksUnavailable) && attackPage.items.length === 0,
        pendingAction,
        currentTeamName,
        serviceOptions,
        teamOptions,
        onAttackerChange: onSetAttackAttacker,
        onLimitChange: onSetAttackLimit,
        onApplyFilters: onApplyAttackFilters,
        onResetFilters: onResetAttackFilters,
        onServiceChange: onSetAttackService,
        onTickFromChange: onSetAttackTickFrom,
        onTickToChange: onSetAttackTickTo,
        onVictimChange: onSetAttackVictim,
        onPage: onPageAttackFeed,
      }}
    />
  );
}

/**
 * Opening the SSH dialog requests the session once (credentials are fetched
 * on open, not on a second click). Kept out of the dialog view so a themed
 * dialog only has to render.
 */
function useAutoRequestSSHSession({
  open,
  issued,
  pending,
  request,
}: {
  open: boolean;
  issued: boolean;
  pending: boolean;
  request: () => Promise<unknown> | void;
}) {
  const triggered = useRef(false);
  const requestRef = useRef(request);
  requestRef.current = request;

  useEffect(() => {
    if (!open) {
      triggered.current = false;
      return;
    }
    if (!issued && !pending && !triggered.current) {
      triggered.current = true;
      void requestRef.current();
    }
  }, [open, issued, pending]);
}

/** Stock services page body: flag form on top, then service cards. */
export function DefaultServicesView({ flagSubmit, ...panel }: ServicesViewProps): ReactElement {
  return (
    <div className="grid gap-4">
      {flagSubmit}
      <ServicesPanel {...panel} />
    </div>
  );
}
