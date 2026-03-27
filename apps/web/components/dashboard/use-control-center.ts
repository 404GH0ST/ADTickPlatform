'use client';

import { useEffect, useMemo, useRef, useState } from 'react';

import type { AttackFeedPage, ScoreRow, ServiceRow } from '@/lib/dashboard-types';
import type { SSHSessionData } from '@/components/dashboard/control-center-sections';

export type ControlCenterOptions = {
  attackPage: AttackFeedPage;
  realtimeBaseUrl: string;
  scores: ScoreRow[];
  services: ServiceRow[];
};

type ActionEnvelope<T> =
  | {
      status: 'success';
      data: T;
    }
  | {
      status: 'failed' | 'forbidden' | 'too many request';
      message: string;
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

type AttackFilters = typeof baseAttackFilters;

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
  const [attackPageState, setAttackPageState] = useState<AttackFeedPage>(attackPage);
  const [proof, setProof] = useState('unlock-proof-from-own-service');
  const [unlockTarget, setUnlockTarget] = useState<ServiceRow | null>(null);
  const [sessionTarget, setSessionTarget] = useState<ServiceRow | null>(null);
  const [resetTarget, setResetTarget] = useState<ServiceRow | null>(null);
  const [issuedSession, setIssuedSession] = useState<SSHSessionData | null>(null);
  const [pendingAction, setPendingAction] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [attackFilters, setAttackFilters] = useState<AttackFilters>(initialAttackFilters);
  const [highlightedAttackIDs, setHighlightedAttackIDs] = useState<string[]>([]);
  const attackPageRef = useRef<AttackFeedPage>(attackPage);
  const attackHighlightTimeoutRef = useRef<number | null>(null);
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
    return () => {
      if (attackHighlightTimeoutRef.current !== null) {
        window.clearTimeout(attackHighlightTimeoutRef.current);
      }
    };
  }, []);

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

    attacksSource.onmessage = (event) => {
      if (!attackLiveMode) {
        return;
      }

      try {
        const nextPage = JSON.parse(event.data) as AttackFeedPage;
        const mergedPage = mergeRealtimeAttackPage(
          attackPageRef.current,
          nextPage,
          attackFilters.limit,
        );
        const nextHighlights = collectNewAttackIDs(attackPageRef.current, mergedPage);
        setAttackPageState(mergedPage);
        if (nextHighlights.length > 0) {
          scheduleAttackHighlights(nextHighlights);
        }
      } catch {
        // Keep the last good snapshot if one event is malformed.
      }
    };

    return () => {
      scoreboardSource.close();
      attacksSource.close();
    };
  }, [attackFilters.limit, attackLiveMode, realtimeBaseUrl]);

  async function refreshAttackFeed(
    silent = false,
    filters: AttackFilters = attackFilters,
  ): Promise<void> {
    if (!silent) {
      setPendingAction('attacks:refresh');
      setActionError(null);
    }

    try {
      const params = new URLSearchParams();
      const limit = parsePositiveInteger(filters.limit, 12, 200);
      if (limit !== undefined) {
        params.set('limit', String(limit));
      }

      const offset = parseNonNegativeInteger(filters.offset);
      if (offset > 0) {
        params.set('offset', String(offset));
      }
      if (filters.attacker.trim() !== '') {
        params.set('attacker', filters.attacker.trim());
      }
      if (filters.victim.trim() !== '') {
        params.set('victim', filters.victim.trim());
      }
      if (filters.service.trim() !== '') {
        params.set('service', filters.service.trim());
      }
      const tickFrom = parseNonNegativeInteger(filters.tickFrom);
      if (tickFrom > 0) {
        params.set('tick_from', String(tickFrom));
      }
      const tickTo = parseNonNegativeInteger(filters.tickTo);
      if (tickTo > 0) {
        params.set('tick_to', String(tickTo));
      }

      const query = params.toString();
      const response = await fetch(`/api/platform/attacks${query ? `?${query}` : ''}`);
      const payload = (await response.json()) as ActionEnvelope<AttackFeedPage>;
      if (!response.ok || payload.status !== 'success') {
        throw new Error('message' in payload ? payload.message : 'attack feed fetch failed');
      }

      clearAttackHighlights();
      setAttackPageState(payload.data);
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
    const limit = parsePositiveInteger(attackFilters.limit, 12, 200) ?? 12;
    const currentOffset = parseNonNegativeInteger(attackFilters.offset);
    const nextOffset =
      direction === 'prev' ? Math.max(0, currentOffset - limit) : currentOffset + limit;
    const next = { ...attackFilters, offset: String(nextOffset) };
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
      const payload = (await response.json()) as ActionEnvelope<{ unlocked: boolean }>;
      if (!response.ok || payload.status !== 'success') {
        throw new Error('message' in payload ? payload.message : 'unlock request failed');
      }

      setRows((current) =>
        current.map((row) =>
          row.id === unlockTarget.id
            ? {
                ...row,
                unlocked: true,
                status: row.status === 'degraded' ? 'warming' : row.status,
                sshHint:
                  'unlock accepted; request a one-time root password to get the current credential',
                lastEvent: 'unlock granted via participant API',
              }
            : row,
        ),
      );
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
      const payload = (await response.json()) as ActionEnvelope<SSHSessionData>;
      if (!response.ok || payload.status !== 'success') {
        throw new Error('message' in payload ? payload.message : 'ssh session request failed');
      }

      setRows((current) =>
        current.map((row) =>
          row.id === sessionTarget.id
            ? {
                ...row,
                unlocked: true,
                sshHint: payload.data.connection_hint,
                lastEvent: 'ssh access active',
              }
            : row,
        ),
      );
      setIssuedSession(payload.data);
      setSessionTarget((current) =>
        current ? { ...current, sshHint: payload.data.connection_hint } : current,
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
      const payload = (await response.json()) as ActionEnvelope<{ unlock_preserved: boolean }>;
      if (!response.ok || payload.status !== 'success') {
        throw new Error('message' in payload ? payload.message : 'factory reset request failed');
      }

      setRows((current) =>
        current.map((row) =>
          row.id === resetTarget.id
            ? {
                ...row,
                status: 'warming',
                checker: 'warning',
                unlocked: payload.data.unlock_preserved ? row.unlocked : false,
                sshHint: payload.data.unlock_preserved
                  ? 'unlock preserved; request a fresh one-time root password to rotate the credential'
                  : row.sshHint,
                lastEvent: 'factory reset triggered via participant API',
                resetCooldown: 'cooldown: 90s',
              }
            : row,
        ),
      );
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
      const payload = (await response.json()) as ActionEnvelope<{ action: 'restart' }>;
      if (!response.ok || payload.status !== 'success') {
        throw new Error('message' in payload ? payload.message : 'restart request failed');
      }

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

function collectNewAttackIDs(currentPage: AttackFeedPage, nextPage: AttackFeedPage): string[] {
  const currentIDs = new Set(currentPage.items.map((item) => item.id));
  return nextPage.items
    .filter((item) => !currentIDs.has(item.id))
    .map((item) => item.id)
    .slice(0, 4);
}

function mergeRealtimeAttackPage(
  currentPage: AttackFeedPage,
  streamedPage: AttackFeedPage,
  requestedLimit: string,
): AttackFeedPage {
  const limit = parsePositiveInteger(requestedLimit, streamedPage.limit) ?? streamedPage.limit;
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

function parsePositiveInteger(raw: string, fallback?: number, max = 0): number | undefined {
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
