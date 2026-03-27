"use client";

import type { ReactElement } from "react";

import {
  AttacksPanel,
  FactoryResetDialog,
  ScoreboardPanel,
  ServicesPanel,
  SSHSessionDialog,
  UnlockServiceDialog,
} from "@/components/dashboard/control-center-sections";
import {
  type ControlCenterOptions,
  useControlCenter,
} from "@/components/dashboard/use-control-center";

export function ControlCenter({
  scores,
  services,
  attackPage,
  realtimeBaseUrl,
  attackFocusMode = "full",
  initialTab = "scoreboard",
  currentTeamName,
}: ControlCenterOptions & {
  attackFocusMode?: "full" | "map";
  initialTab?: "scoreboard" | "services" | "attacks";
  currentTeamName?: string;
}): ReactElement {
  const state = useControlCenter({
    attackPage,
    realtimeBaseUrl,
    scores,
    services,
  });

  return (
    <div className="grid gap-4">
      {renderControlCenterPanel({
        attackLiveMode: state.attackLiveMode,
        attackPage: state.attackPageState,
        attackFilters: state.attackFilters,
        attackFocusMode,
        highlightedAttackIDs: state.highlightedAttackIDs,
        initialTab,
        pendingAction: state.pendingAction,
        rows: state.rows,
        scoreRows: state.scoreRows,
        currentTeamName,
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
        onSetAttackOffset: state.setAttackOffset,
        onSetAttackService: state.setAttackService,
        onSetAttackTickFrom: state.setAttackTickFrom,
        onSetAttackTickTo: state.setAttackTickTo,
        onSetAttackVictim: state.setAttackVictim,
      })}

      <UnlockServiceDialog
        actionError={state.actionError}
        open={state.unlockTarget !== null}
        pendingAction={state.pendingAction}
        proof={state.proof}
        target={state.unlockTarget}
        onClose={state.closeUnlockDialog}
        onConfirm={() => {
          void state.requestUnlock();
        }}
        onProofChange={state.setProof}
      />
      <SSHSessionDialog
        actionError={state.actionError}
        issuedSession={state.issuedSession}
        open={state.sessionTarget !== null}
        pendingAction={state.pendingAction}
        target={state.sessionTarget}
        onClose={state.closeSSHSessionDialog}
        onConfirm={() => {
          void state.requestSSHSession();
        }}
      />
      <FactoryResetDialog
        actionError={state.actionError}
        open={state.resetTarget !== null}
        pendingAction={state.pendingAction}
        target={state.resetTarget}
        onClose={state.closeFactoryResetDialog}
        onConfirm={() => {
          void state.requestFactoryReset();
        }}
      />
    </div>
  );
}

function renderControlCenterPanel({
  attackLiveMode,
  attackPage,
  attackFilters,
  attackFocusMode,
  highlightedAttackIDs,
  initialTab,
  pendingAction,
  rows,
  scoreRows,
  onApplyAttackFilters,
  onPageAttackFeed,
  onRequestRestart,
  onResetAttackFilters,
  onSelectPrimaryAction,
  onSelectResetTarget,
  onSetAttackAttacker,
  onSetAttackLimit,
  onSetAttackOffset,
  onSetAttackService,
  onSetAttackTickFrom,
  onSetAttackTickTo,
  onSetAttackVictim,
  currentTeamName,
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
  attackFocusMode: "full" | "map";
  highlightedAttackIDs: string[];
  initialTab: "scoreboard" | "services" | "attacks";
  pendingAction: string | null;
  rows: ControlCenterOptions["services"];
  scoreRows: ControlCenterOptions["scores"];
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
  onSetAttackOffset: (value: string) => void;
  onSetAttackService: (value: string) => void;
  onSetAttackTickFrom: (value: string) => void;
  onSetAttackTickTo: (value: string) => void;
  onSetAttackVictim: (value: string) => void;
  currentTeamName?: string;
}): ReactElement {
  if (initialTab === "scoreboard") {
    return (
      <ScoreboardPanel
        scoreRows={scoreRows}
        currentTeamName={currentTeamName}
      />
    );
  }

  if (initialTab === "services") {
    return (
      <ServicesPanel
        rows={rows}
        pendingAction={pendingAction}
        onSelectPrimaryAction={onSelectPrimaryAction}
        onRestart={onRequestRestart}
        onSelectReset={onSelectResetTarget}
      />
    );
  }

  return (
    <AttacksPanel
      attackFilters={attackFilters}
      highlightedAttackIDs={highlightedAttackIDs}
      attackPage={attackPage}
      attackLiveMode={attackLiveMode}
      focusMode={attackFocusMode}
      pendingAction={pendingAction}
      onAttackerChange={onSetAttackAttacker}
      onLimitChange={onSetAttackLimit}
      onOffsetChange={onSetAttackOffset}
      onApplyFilters={onApplyAttackFilters}
      onResetFilters={onResetAttackFilters}
      onServiceChange={onSetAttackService}
      onTickFromChange={onSetAttackTickFrom}
      onTickToChange={onSetAttackTickTo}
      onVictimChange={onSetAttackVictim}
      onPage={onPageAttackFeed}
    />
  );
}
