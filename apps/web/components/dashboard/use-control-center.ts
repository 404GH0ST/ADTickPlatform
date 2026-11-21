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
import { browserRequestErrorMessage, PlatformAPIError, processApiResponse } from '@/lib/api-utils';
import { LiveStream } from '@/lib/live-stream';
import {
  SERVICE_ACTION_COOLDOWN_SECONDS,
  cooldownKey,
  cooldownMessage,
  factoryResetSuccessMessage,
  parseRetryAfterSeconds,
  remainingCooldownSeconds,
  restartSuccessMessage,
  slowActionMessage,
  type ServiceAction,
  type ServiceActionFeedback,
  type ServiceNotice,
} from '@/lib/service-actions';

export type ControlCenterOptions = {
  attackPage: AttackFeedPage;
  realtimeBaseUrl: string;
  scores: ScoreRow[];
  services: ServiceRow[];
  /** These requests failed on the latest refresh, so what is on screen stays. */
  scoresUnavailable?: boolean;
  servicesUnavailable?: boolean;
  attacksUnavailable?: boolean;
};

// A normal action answers well before this.
const SLOW_ACTION_MS = 10_000;

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
  /** The filters behind the rows on screen. A failed request leaves these unchanged. */
  appliedAttackFilters: AttackFilters;
  attackFeedError: string | null;
  highlightedAttackIDs: string[];
  attackLiveMode: boolean;
  /** The live streams are delivering data, whatever the last page load did. */
  scoresLive: boolean;
  attacksLive: boolean;
  attackPageState: AttackFeedPage;
  closeFactoryResetDialog: () => void;
  closeSSHSessionDialog: () => void;
  issuedSession: SSHSessionData | null;
  pageAttackFeed: (direction: 'prev' | 'next') => Promise<void>;
  pendingAction: string | null;
  /** Set once the pending action has run unusually long. */
  pendingNote: string | null;
  requestFactoryReset: () => Promise<void>;
  requestRestart: (service: ServiceRow) => Promise<void>;
  requestSSHSession: () => Promise<void>;
  resetAttackFilters: () => Promise<void>;
  resetTarget: ServiceRow | null;
  serviceFeedback: (service: ServiceRow) => ServiceActionFeedback;
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
  summary: ControlCenterSummary;
};

export function useControlCenter({
  attackPage,
  realtimeBaseUrl,
  scores,
  services,
  scoresUnavailable,
  servicesUnavailable,
  attacksUnavailable,
}: ControlCenterOptions): ControlCenterState {
  const initialAttackFilters = useMemo<AttackFilters>(
    () => ({
      ...baseAttackFilters,
      limit: String(attackPage.limit ?? 12),
      offset: String(attackPage.offset ?? 0),
    }),
    [attackPage.limit, attackPage.offset],
  );
  const router = useRouter();
  const [rows, setRows] = useState<ServiceRow[]>(services);
  const [scoreRows, setScoreRows] = useState<ScoreRow[]>(scores);

  useEffect(() => {
    const refresh = () => router.refresh();
    window.addEventListener('platform:flag-accepted', refresh);
    return () => window.removeEventListener('platform:flag-accepted', refresh);
  }, [router]);

  // A failed refresh arrives as an empty list. It says nothing about the match,
  // so the last good rows stay until a real answer replaces them.
  useEffect(() => {
    if (!servicesUnavailable) setRows(services);
  }, [services, servicesUnavailable]);

  useEffect(() => {
    if (!scoresUnavailable) setScoreRows(scores);
  }, [scores, scoresUnavailable]);


  useEffect(() => {
    const interval = setInterval(() => {
      router.refresh();
    }, 20000);

    return () => clearInterval(interval);
  }, [router]);
  const [attackPageState, setAttackPageState] = useState<AttackFeedPage>(attackPage);
  const [sessionTarget, setSessionTarget] = useState<ServiceRow | null>(null);
  const [resetTarget, setResetTarget] = useState<ServiceRow | null>(null);
  const [issuedSession, setIssuedSession] = useState<SSHSessionData | null>(null);
  const [pendingAction, setPendingAction] = useState<string | null>(null);
  const [slowAction, setSlowAction] = useState<string | null>(null);
  const pendingNote =
    pendingAction && slowAction === pendingAction ? slowActionMessage(pendingAction) : null;
  const [actionError, setActionError] = useState<string | null>(null);
  const [serviceNotices, setServiceNotices] = useState<Record<string, ServiceNotice>>({});
  const [cooldownUntil, setCooldownUntil] = useState<Record<string, number>>({});
  const [now, setNow] = useState(() => Date.now());
  const cooldownActive = Object.values(cooldownUntil).some((until) => until > now);

  useEffect(() => {
    if (!pendingAction) return;
    const timer = setTimeout(() => setSlowAction(pendingAction), SLOW_ACTION_MS);
    return () => {
      clearTimeout(timer);
      setSlowAction(null);
    };
  }, [pendingAction]);

  useEffect(() => {
    if (!cooldownActive) {
      return;
    }
    const interval = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(interval);
  }, [cooldownActive]);

  function startCooldown(action: ServiceAction, serviceId: string, seconds: number): void {
    const current = Date.now();
    setNow(current);
    setCooldownUntil((existing) => ({
      ...existing,
      [cooldownKey(action, serviceId)]: current + seconds * 1000,
    }));
  }

  function setServiceNotice(serviceId: string, notice: ServiceNotice): void {
    setServiceNotices((existing) => ({ ...existing, [serviceId]: notice }));
  }

  /**
   * Turn a failed lifecycle request into a participant-facing message. A 429
   * is the per-service cooldown: sync the local countdown to Retry-After.
   */
  function describeActionFailure(
    action: ServiceAction,
    serviceId: string,
    error: unknown,
    fallback: string,
  ): ServiceNotice {
    if (error instanceof PlatformAPIError && error.status === 429) {
      const seconds = parseRetryAfterSeconds(error.retryAfter);
      if (seconds !== null) {
        startCooldown(action, serviceId, seconds);
      }
      const message =
        error.message && !error.message.startsWith('Too many requests')
          ? error.message
          : cooldownMessage(action, seconds ?? SERVICE_ACTION_COOLDOWN_SECONDS[action]);
      return { action, variant: 'warning', message };
    }
    return {
      action,
      variant: 'error',
      message: browserRequestErrorMessage(error, fallback),
    };
  }

  function slowNotice(serviceId: string): ServiceNotice | null {
    if (!pendingNote || !pendingAction?.endsWith(`:${serviceId}`)) return null;
    if (pendingAction.startsWith('restart:')) return { action: 'restart', variant: 'warning', message: pendingNote };
    if (pendingAction.startsWith('reset:')) return { action: 'reset', variant: 'warning', message: pendingNote };
    return null;
  }

  function serviceFeedback(service: ServiceRow): ServiceActionFeedback {
    return {
      notice: slowNotice(service.id) ?? serviceNotices[service.id] ?? null,
      restartCooldown: remainingCooldownSeconds(cooldownUntil[cooldownKey('restart', service.id)], now),
      resetCooldown: remainingCooldownSeconds(cooldownUntil[cooldownKey('reset', service.id)], now),
    };
  }
  const [attackFilters, setAttackFilters] = useState<AttackFilters>(initialAttackFilters);
  const [appliedAttackFilters, setAppliedAttackFilters] = useState<AttackFilters>(initialAttackFilters);
  const [attackFeedError, setAttackFeedError] = useState<string | null>(null);
  const [scoresLive, setScoresLive] = useState(false);
  const [attacksLive, setAttacksLive] = useState(false);
  const attackPageRef = useRef<AttackFeedPage>(attackPage);
  const { highlightedAttackIDs, scheduleAttackHighlights, clearAttackHighlights } = useAttackHighlights();
  const playAttackSfx = useAttackSfx();
  const attackRealtimeEnabled = initialAttackFilters.offset === '0';
  const attackLiveMode = useMemo(
    () => attackRealtimeEnabled && isDefaultAttackFilters(appliedAttackFilters, initialAttackFilters),
    [appliedAttackFilters, attackRealtimeEnabled, initialAttackFilters],
  );
  const attackRows = attackPageState.items;

  // A page that opened during an outage has no feed to keep: take the first good one.
  useEffect(() => {
    if (attacksUnavailable || !attackLiveMode || attackPage.items.length === 0) return;
    setAttackPageState((current) => (current.items.length === 0 ? attackPage : current));
  }, [attackPage, attacksUnavailable, attackLiveMode]);

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

    const scoreboardSource = new LiveStream(`${realtimeBaseUrl}/scoreboard/stream`);
    const attacksSource = new LiveStream(`${realtimeBaseUrl}/attacks/stream`);

    scoreboardSource.onmessage = (event) => {
      try {
        setScoreRows(JSON.parse(event.data) as ScoreRow[]);
        setScoresLive(true);
      } catch {
        // Keep the last good snapshot if one event is malformed.
      }
    };
    scoreboardSource.onerror = () => setScoresLive(false);

    const handleAttackFrame = handleRealtimeAttackMessage({
      isLive: () => attackLiveMode,
      currentPage: () => attackPageRef.current,
      limitStr: appliedAttackFilters.limit,
      setPage: setAttackPageState,
      onInvalidate: () => { void refreshAttackFeed(true, initialAttackFilters, true); },
      scheduleHighlights: (ids, firstBloodIds) => {
        scheduleAttackHighlights([...new Set([...ids, ...firstBloodIds])]);
        playAttackSfx(ids, firstBloodIds);
      },
    });
    attacksSource.onmessage = (event) => {
      setAttacksLive(attackLiveMode);
      handleAttackFrame(event);
    };
    attacksSource.onerror = () => setAttacksLive(false);

    return () => {
      scoreboardSource.close();
      attacksSource.close();
      setScoresLive(false);
      setAttacksLive(false);
    };
  }, [appliedAttackFilters.limit, attackLiveMode, initialAttackFilters, playAttackSfx, realtimeBaseUrl, scheduleAttackHighlights]);

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
    preserveHighlights = false,
  ): Promise<void> {
    if (!silent) {
      setPendingAction('attacks:refresh');
      setActionError(null);
      setAttackFeedError(null);
    }

    try {
      const response = await fetch(`/api/platform/attacks${buildAttackQuery(filters)}`);
      const payload = await processApiResponse<AttackFeedPage>(response, '/api/platform/attacks');

      if (!preserveHighlights) clearAttackHighlights();
      setAttackPageState(payload);
      setAppliedAttackFilters(filters);
    } catch (error) {
      if (!silent) {
        setAttackFeedError(browserRequestErrorMessage(error, "The attack feed couldn't be loaded."));
        // The rows didn't change, so neither did their place in the feed.
        setAttackFilters((current) => ({ ...current, offset: appliedAttackFilters.offset }));
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
      setActionError(browserRequestErrorMessage(error, 'ssh session request failed'));
    } finally {
      setPendingAction(null);
    }
  }

  async function requestFactoryReset(): Promise<void> {
    if (!resetTarget) {
      return;
    }

    const target = resetTarget;
    setPendingAction(`reset:${target.id}`);
    setActionError(null);

    try {
      const response = await fetch(`/api/platform/services/${target.challengeId}/reset/factory`, {
        method: 'POST',
      });
      const payload = await processApiResponse<{ unlock_preserved: boolean; penalty_points?: number }>(response, 'factory reset request');

      setRows((current) =>
        current.map((row) =>
          row.id === target.id
            ? {
                ...row,
                status: 'warming',
                checker: 'warning',
                unlocked: payload.unlock_preserved ? row.unlocked : false,
                sshHint: payload.unlock_preserved
                  ? 'unlock preserved; open SSH Access to reapply the team credential'
                  : row.sshHint,
                lastEvent: payload.penalty_points
                  ? `factory reset discarded stored flag; tick SLA forfeited and ${payload.penalty_points}-point defense penalty`
                  : 'factory reset triggered via participant API',
                resetCooldown: 'cooldown: 90s',
              }
            : row,
        ),
      );
      startCooldown('reset', target.id, SERVICE_ACTION_COOLDOWN_SECONDS.reset);
      setServiceNotice(target.id, {
        action: 'reset',
        variant: payload.penalty_points ? 'warning' : 'success',
        message: factoryResetSuccessMessage(payload),
      });
      setIssuedSession(null);
      setResetTarget(null);
      await refreshServiceRow(target.challengeId).catch(() => undefined);
    } catch (error) {
      const notice = describeActionFailure('reset', target.id, error, 'factory reset request failed');
      setActionError(notice.message);
      setServiceNotice(target.id, notice);
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
      startCooldown('restart', service.id, SERVICE_ACTION_COOLDOWN_SECONDS.restart);
      setServiceNotice(service.id, {
        action: 'restart',
        variant: 'success',
        message: restartSuccessMessage(),
      });

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
      await refreshServiceRow(service.challengeId).catch(() => undefined);
    } catch (error) {
      setServiceNotice(
        service.id,
        describeActionFailure('restart', service.id, error, 'restart request failed'),
      );
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
    }
  }

  function selectResetTarget(service: ServiceRow): void {
    setActionError(null);
    setResetTarget(service);
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
    appliedAttackFilters,
    attackFeedError,
    highlightedAttackIDs,
    attackLiveMode,
    scoresLive,
    attacksLive,
    attackPageState,
    closeFactoryResetDialog,
    closeSSHSessionDialog,
    issuedSession,
    pageAttackFeed,
    pendingAction,
    pendingNote,
    requestFactoryReset,
    requestRestart,
    requestSSHSession,
    resetAttackFilters,
    resetTarget,
    rows,
    serviceFeedback,
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
    summary,
  };
}
