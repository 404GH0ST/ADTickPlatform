'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import { useRouter } from 'next/navigation';

import type { AttackFeedPage, ScoreRow, ServiceRow } from '@/lib/dashboard-types';
import type { SSHSessionData } from '@/components/dashboard/control-center-sections';
import type { TeamServiceState } from '@/lib/platform-api';
import { useAttackHighlights } from '@/components/hooks/use-attack-highlights';
import { useAttackSfx } from '@/components/hooks/use-attack-sfx';
import {
  computePageOffset,
  handleRealtimeAttackMessage,
  parsePositiveInteger,
  parseNonNegativeInteger,
  isDefaultAttackFilters,
  type AttackFilters,
} from '@/lib/dashboard-utils';
import { processApiResponse } from '@/lib/api-utils';

export type ControlCenterOptions = {
  attackPage: AttackFeedPage;
  realtimeBaseUrl: string;
  scores: ScoreRow[];
  services: ServiceRow[];
};

const baseAttackFilters = {
  limit: '12',
  offset: '0',
  attacker: '',
  victim: '',
  service: '',
  tickFrom: '',
  tickTo: '',
};


type ControlCenterSummary = {
  acceptedFlags: number;
  currentTick: number;
  stableServices: number;
  unlockedServices: number;
};

export type ControlCenterState = {
  actionError: string | null;
  applyAttackFilters: () => Promise<void>;
  attackFilters: AttackFilters;
  highlightedAttackIDs: string[];
  attackLiveMode: boolean;
  attackPageState: AttackFeedPage;
  closeFactoryResetDialog: () => void;
  closeSSHSessionDialog: () => void;
  closeUnlockDialog: () => void;
  issuedSession: SSHSessionData | null;
  pageAttackFeed: (direction: 'prev' | 'next') => Promise<void>;
  pendingAction: string | null;
  proof: string;
  requestFactoryReset: () => Promise<void>;
  requestRestart: (service: ServiceRow) => Promise<void>;
  requestSSHSession: () => Promise<void>;
  requestUnlock: () => Promise<void>;
  resetAttackFilters: () => Promise<void>;
  resetTarget: ServiceRow | null;
  rows: ServiceRow[];
  scoreRows: ScoreRow[];
  selectPrimaryAction: (service: ServiceRow) => void;
  selectResetTarget: (service: ServiceRow) => void;
  sessionTarget: ServiceRow | null;
  setAttackLimit: (value: string) => void;
  setAttackOffset: (value: string) => void;
  setAttackAttacker: (value: string) => void;
  setAttackVictim: (value: string) => void;
  setAttackService: (value: string) => void;
  setAttackTickFrom: (value: string) => void;
  setAttackTickTo: (value: string) => void;
  setProof: (value: string) => void;
  summary: ControlCenterSummary;
  unlockTarget: ServiceRow | null;
};

export function useControlCenter({
  attackPage,
  realtimeBaseUrl,
  scores,
  services,
}: ControlCenterOptions): ControlCenterState {
  const initialAttackFilters = useMemo<AttackFilters>(
    () => ({
      ...baseAttackFilters,
      limit: String(attackPage.limit ?? 12),
      offset: String(attackPage.offset ?? 0),
    }),
    [attackPage.limit, attackPage.offset],
  );
  const [rows, setRows] = useState<ServiceRow[]>(services);
  const [scoreRows, setScoreRows] = useState<ScoreRow[]>(scores);

  useEffect(() => {
    setRows(services);
  }, [services]);

  useEffect(() => {
    setScoreRows(scores);
  }, [scores]);

  const router = useRouter();

  useEffect(() => {
    const interval = setInterval(() => {
      router.refresh();
    }, 20000);

    return () => clearInterval(interval);
  }, [router]);
  const [attackPageState, setAttackPageState] = useState<AttackFeedPage>(attackPage);
  const [proof, setProof] = useState('unlock-proof-from-own-service');
  const [unlockTarget, setUnlockTarget] = useState<ServiceRow | null>(null);
  const [sessionTarget, setSessionTarget] = useState<ServiceRow | null>(null);
  const [resetTarget, setResetTarget] = useState<ServiceRow | null>(null);
  const [issuedSession, setIssuedSession] = useState<SSHSessionData | null>(null);
  const [pendingAction, setPendingAction] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [attackFilters, setAttackFilters] = useState<AttackFilters>(initialAttackFilters);
  const attackPageRef = useRef<AttackFeedPage>(attackPage);
  const { highlightedAttackIDs, scheduleAttackHighlights, clearAttackHighlights } = useAttackHighlights();
  const playAttackSfx = useAttackSfx();
  const attackRealtimeEnabled = initialAttackFilters.offset === '0';
  const attackLiveMode = useMemo(
    () => attackRealtimeEnabled && isDefaultAttackFilters(attackFilters, initialAttackFilters),
    [attackFilters, attackRealtimeEnabled, initialAttackFilters],
  );
  const attackRows = attackPageState.items;

  const summary = useMemo(() => {
    const stableServices = rows.filter((row) => row.status === 'stable').length;
    const unlockedServices = rows.filter((row) => row.unlocked).length;
    const currentTick = attackRows.reduce((current, attack) => Math.max(current, attack.tick), 0);

    return {
      acceptedFlags: attackPageState.total_count,
      currentTick,
      stableServices,
      unlockedServices,
    };
  }, [attackPageState.total_count, attackRows, rows]);

  useEffect(() => {
    attackPageRef.current = attackPageState;
  }, [attackPageState]);


  useEffect(() => {
    if (!realtimeBaseUrl) {
      return;
    }

    const scoreboardSource = new EventSource(`${realtimeBaseUrl}/scoreboard/stream`);
    const attacksSource = new EventSource(`${realtimeBaseUrl}/attacks/stream`);

    scoreboardSource.onmessage = (event) => {
      try {
        setScoreRows(JSON.parse(event.data) as ScoreRow[]);
      } catch {
        // Keep the last good snapshot if one event is malformed.
      }
    };

    attacksSource.onmessage = handleRealtimeAttackMessage({
      isLive: () => attackLiveMode,
      currentPage: () => attackPageRef.current,
      limitStr: attackFilters.limit,
      setPage: setAttackPageState,
      scheduleHighlights: (ids) => {
        scheduleAttackHighlights(ids);
        playAttackSfx(ids);
      },
    });

    return () => {
      scoreboardSource.close();
      attacksSource.close();
    };
  }, [attackFilters.limit, attackLiveMode, playAttackSfx, realtimeBaseUrl, scheduleAttackHighlights]);

  function buildAttackQuery(filters: AttackFilters): string {
    const params = new URLSearchParams();
    const limit = parsePositiveInteger(filters.limit, 12, 200);
    if (limit !== undefined) params.set('limit', String(limit));
    const offset = parseNonNegativeInteger(filters.offset) ?? 0;
    if (offset > 0) params.set('offset', String(offset));
    if (filters.attacker.trim() !== '') params.set('attacker', filters.attacker.trim());
    if (filters.victim.trim() !== '') params.set('victim', filters.victim.trim());
    if (filters.service.trim() !== '') params.set('service', filters.service.trim());
    const tickFrom = parseNonNegativeInteger(filters.tickFrom) ?? 0;
    if (tickFrom > 0) params.set('tick_from', String(tickFrom));
    const tickTo = parseNonNegativeInteger(filters.tickTo) ?? 0;
    if (tickTo > 0) params.set('tick_to', String(tickTo));
    const query = params.toString();
    return query ? `?${query}` : '';
  }

  function mergeServiceState(row: ServiceRow, state: TeamServiceState): ServiceRow {
    return {
      ...row,
      endpoint: state.endpoint,
      port: Number(state.endpoint.split(':').at(-1) ?? row.port),
      status: state.status,
      checker: state.checker,
      unlocked: state.unlocked,
      sshHint: state.ssh_hint,
      lastEvent: state.last_event,
      resetCooldown: state.reset_cooldown,
      maintenance: state.maintenance ?? row.maintenance,
      lockReason: state.lock_reason ?? row.lockReason,
      slaStatus: state.sla_status ?? row.slaStatus,
      slaPhase: state.sla_phase ?? row.slaPhase,
      slaTickId: state.sla_tick_id ?? row.slaTickId,
      slaMessage: state.sla_message ?? row.slaMessage,
    };
  }

  async function refreshServiceRow(challengeId: number): Promise<void> {
    const response = await fetch('/api/platform/team/services');
    const states = await processApiResponse<TeamServiceState[]>(
      response,
      '/api/platform/team/services',
    );
    const nextState = states.find((entry) => entry.challenge_id === challengeId);
    if (!nextState) {
      return;
    }
    setRows((current) =>
      current.map((row) =>
        row.challengeId === challengeId ? mergeServiceState(row, nextState) : row,
      ),
    );
  }

  async function refreshAttackFeed(
    silent = false,
    filters: AttackFilters = attackFilters,
  ): Promise<void> {
    if (!silent) {
      setPendingAction('attacks:refresh');
      setActionError(null);
    }

    try {
      const response = await fetch(`/api/platform/attacks${buildAttackQuery(filters)}`);
      const payload = await processApiResponse<AttackFeedPage>(response, '/api/platform/attacks');

      clearAttackHighlights();
      setAttackPageState(payload);
    } catch (error) {
      if (!silent) {
        setActionError(error instanceof Error ? error.message : 'attack feed fetch failed');
      }
    } finally {
      if (!silent) {
        setPendingAction(null);
      }
    }
  }

  async function applyAttackFilters(): Promise<void> {
    const next = { ...attackFilters, offset: '0' };
    setAttackFilters(next);
    await refreshAttackFeed(false, next);
  }

  async function resetAttackFilters(): Promise<void> {
    setAttackFilters(initialAttackFilters);
    await refreshAttackFeed(false, initialAttackFilters);
  }

  async function pageAttackFeed(direction: 'prev' | 'next'): Promise<void> {
    const next = computePageOffset(attackFilters, direction);
    setAttackFilters(next);
    await refreshAttackFeed(false, next);
  }

  async function requestUnlock(): Promise<void> {
    if (!unlockTarget) {
      return;
    }

    setPendingAction(`unlock:${unlockTarget.id}`);
    setActionError(null);

    try {
      const response = await fetch(`/api/platform/services/${unlockTarget.challengeId}/unlock`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ proof }),
      });
      await processApiResponse<{ unlocked: boolean }>(response, 'unlock request');

      setRows((current) =>
        current.map((row) =>
          row.id === unlockTarget.id
            ? {
                ...row,
                unlocked: true,
                status: row.status === 'degraded' ? 'warming' : row.status,
                sshHint: 'unlock accepted; use SSH Access to view the team credential',
                lastEvent: 'unlock granted via participant API',
              }
            : row,
        ),
      );
      await refreshServiceRow(unlockTarget.challengeId);
      setUnlockTarget(null);
    } catch (error) {
      setActionError(error instanceof Error ? error.message : 'unlock request failed');
    } finally {
      setPendingAction(null);
    }
  }

  async function requestSSHSession(): Promise<void> {
    if (!sessionTarget) {
      return;
    }

    setPendingAction(`ssh:${sessionTarget.id}`);
    setActionError(null);

    try {
      const response = await fetch(`/api/platform/services/${sessionTarget.challengeId}/ssh-session`, {
        method: 'POST',
      });
      const payload = await processApiResponse<SSHSessionData>(response, 'ssh session request');

      setRows((current) =>
        current.map((row) =>
          row.id === sessionTarget.id
            ? {
                ...row,
                unlocked: true,
                sshHint: payload.connection_hint,
                lastEvent: 'team ssh credential retrieved',
              }
            : row,
        ),
      );
      await refreshServiceRow(sessionTarget.challengeId);
      setIssuedSession(payload);
      setSessionTarget((current) =>
        current ? { ...current, sshHint: payload.connection_hint } : current,
      );
    } catch (error) {
      setActionError(error instanceof Error ? error.message : 'ssh session request failed');
    } finally {
      setPendingAction(null);
    }
  }

  async function requestFactoryReset(): Promise<void> {
    if (!resetTarget) {
      return;
    }

    setPendingAction(`reset:${resetTarget.id}`);
    setActionError(null);

    try {
      const response = await fetch(`/api/platform/services/${resetTarget.challengeId}/reset/factory`, {
        method: 'POST',
      });
      const payload = await processApiResponse<{ unlock_preserved: boolean }>(response, 'factory reset request');

      setRows((current) =>
        current.map((row) =>
          row.id === resetTarget.id
            ? {
                ...row,
                status: 'warming',
                checker: 'warning',
                unlocked: payload.unlock_preserved ? row.unlocked : false,
                sshHint: payload.unlock_preserved
                  ? 'unlock preserved; open SSH Access to reapply the team credential'
                  : row.sshHint,
                lastEvent: 'factory reset triggered via participant API',
                resetCooldown: 'cooldown: 90s',
              }
            : row,
        ),
      );
      await refreshServiceRow(resetTarget.challengeId);
      setIssuedSession(null);
      setResetTarget(null);
    } catch (error) {
      setActionError(error instanceof Error ? error.message : 'factory reset request failed');
    } finally {
      setPendingAction(null);
    }
  }

  async function requestRestart(service: ServiceRow): Promise<void> {
    setPendingAction(`restart:${service.id}`);
    setActionError(null);

    try {
      const response = await fetch(`/api/platform/services/${service.challengeId}/reset/restart`, {
        method: 'POST',
      });
      await processApiResponse<{ action: 'restart' }>(response, 'restart request');

      setRows((current) =>
        current.map((row) =>
          row.id === service.id
            ? {
                ...row,
                status: 'warming',
                checker: 'warning',
                lastEvent: 'service restart triggered via participant API',
                resetCooldown: 'restart requested',
              }
            : row,
        ),
      );
      await refreshServiceRow(service.challengeId);
    } catch (error) {
      setActionError(error instanceof Error ? error.message : 'restart request failed');
    } finally {
      setPendingAction(null);
    }
  }

  function setAttackLimit(value: string): void {
    setAttackFilters((current) => ({ ...current, limit: value }));
  }

  function setAttackOffset(value: string): void {
    setAttackFilters((current) => ({ ...current, offset: value }));
  }

  function setAttackAttacker(value: string): void {
    setAttackFilters((current) => ({ ...current, attacker: value }));
  }

  function setAttackVictim(value: string): void {
    setAttackFilters((current) => ({ ...current, victim: value }));
  }

  function setAttackService(value: string): void {
    setAttackFilters((current) => ({ ...current, service: value }));
  }

  function setAttackTickFrom(value: string): void {
    setAttackFilters((current) => ({ ...current, tickFrom: value }));
  }

  function setAttackTickTo(value: string): void {
    setAttackFilters((current) => ({ ...current, tickTo: value }));
  }

  function selectPrimaryAction(service: ServiceRow): void {
    if (service.unlocked) {
      setSessionTarget(service);
      return;
    }

    setUnlockTarget(service);
  }

  function selectResetTarget(service: ServiceRow): void {
    setResetTarget(service);
  }

  function closeUnlockDialog(): void {
    setUnlockTarget(null);
  }

  function closeSSHSessionDialog(): void {
    setSessionTarget(null);
    setIssuedSession(null);
  }

  function closeFactoryResetDialog(): void {
    setResetTarget(null);
  }



  return {
    actionError,
    applyAttackFilters,
    attackFilters,
    highlightedAttackIDs,
    attackLiveMode,
    attackPageState,
    closeFactoryResetDialog,
    closeSSHSessionDialog,
    closeUnlockDialog,
    issuedSession,
    pageAttackFeed,
    pendingAction,
    proof,
    requestFactoryReset,
    requestRestart,
    requestSSHSession,
    requestUnlock,
    resetAttackFilters,
    resetTarget,
    rows,
    scoreRows,
    selectPrimaryAction,
    selectResetTarget,
    sessionTarget,
    setAttackLimit,
    setAttackOffset,
    setAttackAttacker,
    setAttackVictim,
    setAttackService,
    setAttackTickFrom,
    setAttackTickTo,
    setProof,
    summary,
    unlockTarget,
  };
}
