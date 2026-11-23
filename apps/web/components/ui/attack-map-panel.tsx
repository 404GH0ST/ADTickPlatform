'use client';

import type { ReactElement, ReactNode } from 'react';
import { lazy, useCallback, useEffect, useId, useMemo, useRef, useState } from 'react';
import {
  Check,
  ChevronLeft,
  ChevronRight,
  Expand,
  Minimize2,
  Pause,
  Play,
  Search,
  Volume2,
  VolumeX,
  Zap,
} from 'lucide-react';

import {
  useAttackSfxPreferences,
  useAttackSfxPreview,
} from '@/components/hooks/use-attack-sfx';
import { HydrationSuspense } from '@/components/ui/hydration-suspense';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { ScrollableRegion } from '@/components/ui/scrollable-region';
import { cn } from '@/lib/utils';

const CyberAttackMap = lazy(async () => {
  const { CyberAttackMap } = await import('./cyber-attack-map');
  return { default: CyberAttackMap };
});

export type AttackMapEvent = {
  first_blood?: boolean;
  id: string;
  attacker: string;
  attacker_institution?: string;
  victim_institution?: string;
  attackerLocation?: {
    lat: number;
    lon: number;
  };
  victim: string;
  victimLocation?: {
    lat: number;
    lon: number;
  };
  service: string;
  tick: number;
  verdict: string;
};

type ViewMode = 'all' | 'recent' | 'current';

const RECENT_TICK_WINDOW = 5;
const PAUSED_CAMERA_KEY = 'ad-platform-attack-camera-paused';
const PLAYBACK_SPEEDS = [
  { label: 'Fast', value: 900 },
  { label: 'Normal', value: 1500 },
  { label: 'Slow', value: 2300 },
];

function filterRowsForView<T extends { tick: number }>(rows: T[], viewMode: ViewMode, selectedTick: number): T[] {
  if (rows.length === 0 || selectedTick === 0) {
    return rows;
  }

  switch (viewMode) {
    case 'current':
      return rows.filter((row) => row.tick === selectedTick);
    case 'recent': {
      const windowStart = Math.max(0, selectedTick - (RECENT_TICK_WINDOW - 1));
      return rows.filter((row) => row.tick >= windowStart && row.tick <= selectedTick);
    }
    case 'all':
    default:
      return rows;
  }
}

const NO_FIRST_BLOODS: AttackMapEvent[] = [];

export function AttackMapPanel({
  attackRows,
  firstBloods,
  className,
  description = 'Directional attack flow across teams, services, and match ticks.',
  highlightedAttackIDs = [],
  title = 'Attack map',
  teamRoster = [],
  onVisibleRowsChange,
}: {
  attackRows: AttackMapEvent[];
  /** Every first blood in capture order, including any outside `attackRows`. */
  firstBloods?: AttackMapEvent[] | null;
  /**
   * Every team in the match (the scoreboard). Teams are placed on the globe
   * from this full roster up front, so a team appearing in or dropping out of
   * the loaded attack page never shifts anyone else's location.
   */
  teamRoster?: string[];
  className?: string;
  description?: string;
  highlightedAttackIDs?: string[];
  title?: string;
  onVisibleRowsChange?: (rows: AttackMapEvent[]) => void;
}): ReactElement {
  const [expanded, setExpanded] = useState(false);
  const [expandedLayout, setExpandedLayout] = useState<HTMLDivElement | null>(null);
  const [compactViewer, setCompactViewer] = useState(false);
  useEffect(() => {
    if (!expandedLayout) return;
    const observer = new ResizeObserver(([entry]) => {
      const rem = Number.parseFloat(getComputedStyle(document.documentElement).fontSize);
      const { width, height } = entry.contentRect;
      const comfortableHeight = width < 66 * rem ? 50 * rem : 34 * rem;
      setCompactViewer(width < 25 * rem || height < comfortableHeight);
    });
    observer.observe(expandedLayout);
    return () => observer.disconnect();
  }, [expandedLayout]);
  const maximizeTrigger = useRef<HTMLButtonElement>(null);
  const viewLabelId = useId();
  const [autoCameraPreference, setAutoCameraPreference] = useState<boolean | null>(null);
  const [teamQuery, setTeamQuery] = useState('');
  const [replayAttackIDs, setReplayAttackIDs] = useState<string[]>([]);
  const [viewMode, setViewMode] = useState<ViewMode>('all');
  const [selectedTick, setSelectedTick] = useState<number | null>(null);
  const [playbackRunning, setPlaybackRunning] = useState(false);
  const [playbackSpeed, setPlaybackSpeed] = useState<number>(1500);
  const [selectedTeamId, setSelectedTeamId] = useState<string | null>(null);
  const [selectedAttackId, setSelectedAttackId] = useState<string | null>(null);
  const [featuredAttackId, setFeaturedAttackId] = useState<string | null>(null);
  const { preferences: attackSfx, setVolume, toggleEnabled } =
    useAttackSfxPreferences();
  const previewAttackSfx = useAttackSfxPreview();
  const replayTimeoutRef = useRef<number | null>(null);
  const replayFrameRef = useRef<number | null>(null);

  useEffect(() => {
    try {
      if (window.localStorage.getItem(PAUSED_CAMERA_KEY) === 'true') {
        setAutoCameraPreference(false);
      }
    } catch {
      // The control still works when this browser blocks local storage.
    }
  }, []);

  const changeAutoCamera = useCallback((enabled: boolean) => {
    const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    setAutoCameraPreference(enabled ? (reducedMotion ? true : null) : false);
    try {
      if (enabled) {
        window.localStorage.removeItem(PAUSED_CAMERA_KEY);
      } else {
        window.localStorage.setItem(PAUSED_CAMERA_KEY, 'true');
      }
    } catch {
      // Keep the in-memory choice when it cannot be saved.
    }
  }, []);

  const tickValues = useMemo(
    () => Array.from(new Set(attackRows.map((row) => row.tick))).sort((left, right) => left - right),
    [attackRows],
  );
  const latestTick = tickValues.at(-1) ?? 0;
  const selectedTickValue = selectedTick ?? latestTick;

  useEffect(() => {
    if (tickValues.length === 0) {
      setSelectedTick(null);
      setPlaybackRunning(false);
      return;
    }
    if (selectedTick === null || !tickValues.includes(selectedTick)) {
      setSelectedTick(latestTick);
    }
  }, [latestTick, selectedTick, tickValues]);

  useEffect(() => {
    return () => {
      if (replayTimeoutRef.current !== null) {
        window.clearTimeout(replayTimeoutRef.current);
      }
      if (replayFrameRef.current !== null) {
        window.cancelAnimationFrame(replayFrameRef.current);
      }
    };
  }, []);

  useEffect(() => {
    if (!playbackRunning || tickValues.length < 2 || selectedTickValue === 0) {
      return;
    }

    const interval = window.setInterval(() => {
      setSelectedTick((current) => {
        if (current === null) {
          return latestTick;
        }
        return getAdjacentTick(tickValues, current, 'next');
      });
    }, playbackSpeed);

    return () => {
      window.clearInterval(interval);
    };
  }, [latestTick, playbackRunning, playbackSpeed, selectedTickValue, tickValues]);

  const visibleRows = useMemo(
    () => filterRowsForView(attackRows, viewMode, selectedTickValue),
    [attackRows, selectedTickValue, viewMode],
  );
  const visibleFirstBloods = useMemo(
    () => filterRowsForView(firstBloods ?? NO_FIRST_BLOODS, viewMode, selectedTickValue),
    [firstBloods, selectedTickValue, viewMode],
  );

  useEffect(() => {
    onVisibleRowsChange?.(visibleRows);
  }, [visibleRows, onVisibleRowsChange]);

  const visibleTeamCount = useMemo(
    () =>
      new Set([
        ...visibleRows.map((row) => row.attacker),
        ...visibleRows.map((row) => row.victim),
      ]).size,
    [visibleRows],
  );
  const visibleServiceCount = useMemo(
    () => new Set(visibleRows.map((row) => row.service)).size,
    [visibleRows],
  );
  const visibleTicks = useMemo(
    () => Array.from(new Set(visibleRows.map((row) => row.tick))).sort((left, right) => left - right),
    [visibleRows],
  );
  const allTeams = useMemo(
    () =>
      Array.from(
        new Set([
          ...attackRows.map((row) => row.attacker),
          ...attackRows.map((row) => row.victim),
        ]),
      ).sort(),
    [attackRows],
  );
  // Key on content, not identity: callers rebuild the roster array each
  // render, and the globe restarts auto-rotation when this list changes.
  const firstBloodTeams = (firstBloods ?? NO_FIRST_BLOODS).flatMap((row) => [row.attacker, row.victim]);
  const placementKey = Array.from(new Set([...teamRoster, ...allTeams, ...firstBloodTeams])).sort().join('\u0000');
  const placementTeams = useMemo(
    () => (placementKey === '' ? [] : placementKey.split('\u0000')),
    [placementKey],
  );
  const normalizedTeamQuery = teamQuery.trim().toLowerCase();
  const searchedTeams = useMemo(() => {
    if (normalizedTeamQuery === '') {
      return [];
    }
    return allTeams.filter((team) =>
      team.toLowerCase().includes(normalizedTeamQuery),
    );
  }, [allTeams, normalizedTeamQuery]);
  const focusedTeams = useMemo(() => {
    const values = new Set(searchedTeams);
    if (selectedTeamId) {
      values.add(selectedTeamId);
    }
    return Array.from(values);
  }, [searchedTeams, selectedTeamId]);
  const currentTickAttackIDs = useMemo(
    () =>
      selectedTickValue > 0
        ? attackRows
            .filter((row) => row.tick === selectedTickValue)
            .map((row) => row.id)
        : [],
    [attackRows, selectedTickValue],
  );
  const activeHighlightIDs = useMemo(
    () =>
      Array.from(
        new Set([
          ...highlightedAttackIDs,
          ...replayAttackIDs,
          ...(selectedAttackId ? [selectedAttackId] : []),
        ]),
      ),
    [highlightedAttackIDs, replayAttackIDs, selectedAttackId],
  );
  const teamLocations = useMemo(() => {
    const locations: Record<string, { lat: number; lon: number }> = {};
    attackRows.forEach((row) => {
      if (row.attackerLocation) {
        locations[row.attacker] = row.attackerLocation;
      }
      if (row.victimLocation) {
        locations[row.victim] = row.victimLocation;
      }
    });
    return locations;
  }, [attackRows]);

  const selectedAttack = useMemo(
    () => attackRows.find((row) => row.id === selectedAttackId) ?? null,
    [attackRows, selectedAttackId],
  );
  const featuredAttack = useMemo(
    () => attackRows.find((row) => row.id === featuredAttackId) ?? null,
    [attackRows, featuredAttackId],
  );
  const inspectedAttack = selectedAttack ?? featuredAttack;
  const selectedTeamSummary = useMemo(() => {
    if (!selectedTeamId) {
      return null;
    }

    const involvedRows = visibleRows.filter(
      (row) => row.attacker === selectedTeamId || row.victim === selectedTeamId,
    );
    const institution =
      involvedRows
        .map((row) =>
          row.attacker === selectedTeamId ? row.attacker_institution : row.victim_institution,
        )
        .find((value) => Boolean(value)) ?? '';
    if (involvedRows.length === 0) {
      return {
        attacks: 0,
        inbound: 0,
        institution,
        outbound: 0,
        latestTick: null,
        opponents: [],
        services: [],
        team: selectedTeamId,
      };
    }

    const outbound = involvedRows.filter((row) => row.attacker === selectedTeamId);
    const inbound = involvedRows.filter((row) => row.victim === selectedTeamId);
    const opponentCounts = new Map<string, number>();
    involvedRows.forEach((row) => {
      const opponent = row.attacker === selectedTeamId ? row.victim : row.attacker;
      opponentCounts.set(opponent, (opponentCounts.get(opponent) ?? 0) + 1);
    });

    return {
      attacks: involvedRows.length,
      inbound: inbound.length,
      institution,
      outbound: outbound.length,
      latestTick: involvedRows.reduce(
        (current, row) => Math.max(current, row.tick),
        0,
      ),
      opponents: Array.from(opponentCounts.entries())
        .sort((left, right) => right[1] - left[1] || left[0].localeCompare(right[0]))
        .slice(0, 6),
      services: Array.from(new Set(involvedRows.map((row) => row.service))).sort(),
      team: selectedTeamId,
    };
  }, [selectedTeamId, visibleRows]);

  useEffect(() => {
    if (selectedAttackId && !attackRows.some((row) => row.id === selectedAttackId)) {
      setSelectedAttackId(null);
    }
  }, [attackRows, selectedAttackId]);

  useEffect(() => {
    if (featuredAttackId && !attackRows.some((row) => row.id === featuredAttackId)) {
      setFeaturedAttackId(null);
    }
  }, [attackRows, featuredAttackId]);

  const handleFeaturedAttackChange = useCallback((attackId: string | null) => {
    setFeaturedAttackId(attackId);
  }, []);

  useEffect(() => {
    if (selectedTeamId && !allTeams.includes(selectedTeamId)) {
      setSelectedTeamId(null);
    }
  }, [allTeams, selectedTeamId]);

  function replayTick(tick: number): void {
    const tickAttackIDs = attackRows
      .filter((row) => row.tick === tick)
      .map((row) => row.id);

    if (tickAttackIDs.length === 0) {
      return;
    }

    if (replayTimeoutRef.current !== null) {
      window.clearTimeout(replayTimeoutRef.current);
    }
    if (replayFrameRef.current !== null) {
      window.cancelAnimationFrame(replayFrameRef.current);
    }

    setReplayAttackIDs([]);
    replayFrameRef.current = window.requestAnimationFrame(() => {
      setReplayAttackIDs(tickAttackIDs);
      replayTimeoutRef.current = window.setTimeout(() => {
        setReplayAttackIDs([]);
        replayTimeoutRef.current = null;
      }, 3200);
    });
  }

  function replayCurrentTick(): void {
    if (selectedTickValue === 0) {
      return;
    }
    if (viewMode === 'all') {
      setViewMode('current');
    }
    replayTick(selectedTickValue);
  }

  function moveTick(direction: 'prev' | 'next'): void {
    if (selectedTickValue === 0) {
      return;
    }
    if (viewMode === 'all') {
      setViewMode('current');
    }
    setSelectedTick((current) =>
      getAdjacentTick(tickValues, current ?? latestTick, direction),
    );
  }

  const togglePlayback = useCallback(() => {
    setPlaybackRunning((current) => {
      const next = !current;
      if (next && viewMode === 'all') {
        setViewMode('current');
      }
      return next;
    });
  }, [viewMode]);

  function renderInspector(): ReactElement {
    if (inspectedAttack) {
      const inspectingAutoFeature = selectedAttack === null;
      return (
        <InspectorCard
          eyebrow={inspectingAutoFeature ? 'Featured transmission' : 'Attack'}
          title={`${inspectedAttack.attacker} -> ${inspectedAttack.victim}`}
          lines={[
            { label: 'Tick', value: `#${inspectedAttack.tick}` },
            { label: 'Service', value: inspectedAttack.service },
            { label: 'Verdict', value: inspectedAttack.first_blood ? 'First blood' : inspectedAttack.verdict },
          ]}
        >
          {inspectingAutoFeature ? (
            <p className="text-sm text-muted-foreground">
              Select a path to inspect it.
            </p>
          ) : (
            <Button
              size="sm"
              variant="outline"
              onClick={() => setSelectedAttackId(null)}
            >
              Clear attack focus
            </Button>
          )}
        </InspectorCard>
      );
    }

    if (selectedTeamSummary) {
      return (
        <InspectorCard
          eyebrow="Team"
          title={selectedTeamSummary.team}
          subtitle={selectedTeamSummary.institution || undefined}
          lines={[
            { label: 'Outbound', value: String(selectedTeamSummary.outbound) },
            { label: 'Inbound', value: String(selectedTeamSummary.inbound) },
            { label: 'Total involved', value: String(selectedTeamSummary.attacks) },
            {
              label: 'Latest tick',
              value:
                selectedTeamSummary.latestTick === null
                  ? 'not visible'
                  : `#${selectedTeamSummary.latestTick}`,
            },
          ]}
        >
          {selectedTeamSummary.services.length > 0 ? (
            <div className="space-y-2">
              <p className="text-xs font-medium text-muted-foreground">Services</p>
              <div className="flex flex-wrap gap-2">
                {selectedTeamSummary.services.map((service) => (
                  <Badge key={service} variant="secondary">
                    {service}
                  </Badge>
                ))}
              </div>
            </div>
          ) : null}
          {selectedTeamSummary.opponents.length > 0 ? (
            <div className="space-y-2">
              <p className="text-xs font-medium text-muted-foreground">Frequent opponents</p>
              <div className="space-y-1 text-sm text-muted-foreground">
                {selectedTeamSummary.opponents.map(([opponent, count]) => (
                  <div
                    key={opponent}
                    className="flex items-center justify-between gap-3"
                  >
                    <span className="truncate">{opponent}</span>
                    <span className="font-mono text-foreground">{count}</span>
                  </div>
                ))}
              </div>
            </div>
          ) : null}
          <Button
            size="sm"
            variant="outline"
            onClick={() => setSelectedTeamId(null)}
          >
            Clear team focus
          </Button>
        </InspectorCard>
      );
    }

    return (
      <InspectorCard
        eyebrow="Inspector"
        title="Map overview"
        lines={[
          { label: 'Visible attacks', value: String(visibleRows.length) },
          { label: 'Visible teams', value: String(visibleTeamCount) },
          { label: 'Visible services', value: String(visibleServiceCount) },
          { label: 'Tick window', value: formatTickWindow(visibleTicks) },
        ]}
      >
        <p className="text-sm text-muted-foreground">
          Click a team or attack path to inspect it. Search focuses labels while
          the rest of the globe stays readable.
        </p>
      </InspectorCard>
    );
  }

  const expandedActions = (
    <div className="flex min-w-0 flex-wrap items-center gap-2">
      <Badge data-testid="attack-map-visible-attacks" variant="secondary">{visibleRows.length} attacks</Badge>
      <Badge data-testid="attack-map-visible-teams" variant="secondary">{visibleTeamCount} teams</Badge>
      <Badge data-testid="attack-map-visible-services" variant="secondary">{visibleServiceCount} services</Badge>
      <Button
        data-testid="attack-map-highlight-current-tick"
        size="sm"
        variant="outline"
        className="any-pointer-coarse:min-h-11"
        disabled={currentTickAttackIDs.length === 0}
        onClick={replayCurrentTick}
      >
        <Zap className="size-4" aria-hidden />
        Highlight tick #{selectedTickValue}
      </Button>
    </div>
  );

  function renderMapSurface(expandedView: boolean): ReactElement {
    const compact = expandedView && compactViewer;
    const mapSize = expandedView
      ? cn('aspect-auto flex-1', compact ? 'min-h-0' : 'min-h-[min(16rem,65dvh)]')
      : 'min-h-[27rem]';

    return (
      <div
        className={cn(
          'gap-3',
          expandedView
            ? 'flex h-full flex-col @[60rem]/expanded-map:grid @[60rem]/expanded-map:grid-cols-[minmax(0,1fr)_16rem] @[60rem]/expanded-map:grid-rows-[minmax(0,1fr)]'
            : 'grid grid-rows-[minmax(0,1fr)_auto] xl:grid-cols-[minmax(0,1fr)_18rem] 2xl:grid-cols-[minmax(0,1.18fr)_17rem] xl:grid-rows-none',
        )}
      >
        <HydrationSuspense
          fallback={
            <div data-slot="well"
              role="status"
              className={cn('grid place-items-center rounded-sm border bg-card p-4 text-sm text-muted-foreground', mapSize)}
            >
              Loading attack globe…
            </div>
          }
        >
          <CyberAttackMap
            attacks={visibleRows}
            firstBloods={visibleFirstBloods}
            autoCameraPreference={autoCameraPreference}
            className={mapSize}
            expanded={expandedView}
            compactActions={compact ? expandedActions : undefined}
            focusedTeams={focusedTeams}
            highlightedAttackIDs={activeHighlightIDs}
            liveAttackIDs={highlightedAttackIDs}
            placementScopeTeams={placementTeams}
            selectedAttackId={selectedAttackId}
            selectedTeamId={selectedTeamId}
            teamLocations={teamLocations}
            onAutoCameraChange={changeAutoCamera}
            onFeaturedAttackChange={handleFeaturedAttackChange}
            onSelectAttack={(attackId) => {
              setSelectedAttackId(attackId);
              if (attackId) {
                setSelectedTeamId(null);
              }
            }}
            onSelectTeam={(teamId) => {
              setSelectedTeamId(teamId);
              if (teamId) {
                setSelectedAttackId(null);
              }
            }}
          />
        </HydrationSuspense>
        <div
          className={cn(
            'min-w-0',
            expandedView && 'shrink-0 @[60rem]/expanded-map:min-h-0 @[60rem]/expanded-map:overflow-y-auto @[60rem]/expanded-map:pr-1',
          )}
        >
          {renderInspector()}
        </div>
      </div>
    );
  }

  return (
    <div data-slot="well"
      data-testid="attack-map-panel"
      className={cn(
        'space-y-3 rounded-sm border border-border/70 bg-card px-1 py-3 sm:px-3 any-pointer-coarse:[&_[data-size=touch]]:min-h-11 any-pointer-coarse:[&_[data-size=touch]]:min-w-11',
        className,
      )}
    >
      <div className="flex flex-col gap-3 border-b pb-3 xl:flex-row xl:items-start xl:justify-between">
        <div className="space-y-1">
          <p className="text-sm font-medium text-foreground">{title}</p>
          <p className="text-sm text-muted-foreground">{description}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Badge data-testid="attack-map-visible-attacks" variant="secondary">
            {visibleRows.length} attacks
          </Badge>
          <Badge data-testid="attack-map-visible-teams" variant="secondary">
            {visibleTeamCount} teams
          </Badge>
          <Badge data-testid="attack-map-visible-services" variant="secondary">
            {visibleServiceCount} services
          </Badge>
          <div data-slot="well"
            className="flex min-h-9 items-center gap-2 rounded-sm border border-border/70 bg-muted/20 px-2"
            data-testid="attack-sfx-control"
          >
            <Button
              aria-label="SFX: Attack sound"
              aria-pressed={attackSfx.enabled}
              className="h-7 px-2 text-xs sm:h-7"
              size="touch"
              title={attackSfx.enabled ? 'Mute attack sound' : 'Enable attack sound'}
              variant="ghost"
              onClick={toggleEnabled}
            >
              {attackSfx.enabled ? (
                <Volume2 className="h-4 w-4" />
              ) : (
                <VolumeX className="h-4 w-4" />
              )}
              SFX
            </Button>
            <input
              aria-label="Attack sound volume"
              className="hidden h-7 w-20 accent-primary disabled:opacity-40 sm:block"
              disabled={!attackSfx.enabled}
              max="1"
              min="0"
              step="0.05"
              type="range"
              value={attackSfx.volume}
              onChange={(event) => setVolume(Number(event.target.value))}
            />
            <Button
              aria-label="Test attack sound"
              className="hidden h-7 px-2 text-xs sm:inline-flex sm:h-7"
              disabled={!attackSfx.enabled}
              size="touch"
              title="Test attack sound"
              variant="ghost"
              onClick={previewAttackSfx}
            >
              Test
            </Button>
          </div>
          <Button
            size="touch"
            variant="outline"
            onClick={(event) => {
              maximizeTrigger.current = event.currentTarget;
              setExpanded(true);
            }}
          >
            <Expand className="h-4 w-4" />
            Maximize
          </Button>
        </div>
      </div>

      {attackRows.length > 0 ? (
        <div data-slot="well" className="@container grid grid-cols-1 gap-3 rounded-sm border border-border/70 bg-muted/20 px-1 py-3 sm:px-3">
          <div className="grid grid-cols-1 gap-3 @[48rem]:grid-cols-[minmax(0,1fr)_15rem] @[48rem]:items-start">
            <div className="grid min-w-0 grid-cols-1 gap-3 @[38rem]:grid-cols-[minmax(0,1fr)_auto] @[38rem]:items-end">
              <div className="space-y-2">
                <label
                  htmlFor={`${title}-team-search`}
                  className="text-sm font-medium text-foreground"
                >
                  Focus team
                </label>
                <div className="relative">
                  <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                  <Input
                    id={`${title}-team-search`}
                    value={teamQuery}
                    onChange={(event) => setTeamQuery(event.target.value)}
                    placeholder="Search team name to reveal labels"
                    className="pl-9"
                  />
                </div>
              </div>
              <div className="space-y-2">
                <p id={viewLabelId} className="text-sm font-medium text-foreground">View</p>
                <div role="group" aria-labelledby={viewLabelId} className="flex flex-wrap gap-2">
                  {(['all', 'recent', 'current'] as ViewMode[]).map((mode) => (
                    <Button
                      key={mode}
                      type="button"
                      size="touch"
                      aria-pressed={viewMode === mode}
                      className="gap-1 px-1 sm:gap-2 sm:px-3"
                      variant={viewMode === mode ? 'default' : 'outline'}
                      onClick={() => setViewMode(mode)}
                    >
                      <Check
                        aria-hidden="true"
                        className={cn('size-3 shrink-0 sm:size-4', viewMode !== mode && 'invisible')}
                      />
                      <span className="min-w-0 wrap-anywhere">{describeViewMode(mode)}</span>
                    </Button>
                  ))}
                </div>
              </div>
            </div>
            <div className="min-w-0 space-y-1 wrap-anywhere @[48rem]:text-right">
              <p
                className="text-sm font-medium text-foreground"
                data-testid="attack-map-selected-tick"
              >
                {selectedTickValue > 0 ? `Tick #${selectedTickValue}` : 'No ticks'}
              </p>
              <p className="max-w-[34rem] text-sm text-muted-foreground">
                {normalizedTeamQuery === ''
                  ? `${describeViewMode(viewMode)} view across ${formatTickWindow(visibleTicks)}.`
                  : `Showing labels for ${searchedTeams.length} matching team(s).`}
              </p>
            </div>
          </div>
          <div className="flex flex-col gap-3 border-t border-border/60 pt-3 lg:flex-row lg:items-center lg:justify-between">
            <div className="space-y-1">
              <label
                htmlFor={`${title}-playback-speed`}
                className="text-xs font-medium uppercase text-muted-foreground"
              >
                Playback
              </label>
              <p className="text-sm text-muted-foreground">
                Review ticks or highlight the current burst.
              </p>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <Button
                size="touch"
                variant="outline"
                disabled={tickValues.length === 0}
                onClick={() => moveTick('prev')}
                aria-label="Previous Tick"
              >
                <ChevronLeft className="h-4 w-4" />
              </Button>
              <Button
                size="touch"
                variant="outline"
                disabled={tickValues.length < 2}
                onClick={togglePlayback}
              >
                {playbackRunning ? (
                  <Pause className="h-4 w-4" />
                ) : (
                  <Play className="h-4 w-4" />
                )}
                {playbackRunning ? 'Pause' : 'Play'}
              </Button>
              <Button
                size="touch"
                variant="outline"
                disabled={tickValues.length === 0}
                onClick={() => moveTick('next')}
                aria-label="Next Tick"
              >
                <ChevronRight className="h-4 w-4" />
              </Button>
              <select
                id={`${title}-playback-speed`}
                aria-label="Playback speed"
                className="flex h-11 rounded-sm border border-input bg-card px-3 text-sm any-pointer-coarse:min-h-11"
                value={String(playbackSpeed)}
                onChange={(event) => setPlaybackSpeed(Number(event.target.value))}
              >
                {PLAYBACK_SPEEDS.map((speed) => (
                  <option key={speed.value} value={speed.value}>
                    {speed.label}
                  </option>
                ))}
              </select>
              <Button
                data-testid="attack-map-highlight-current-tick"
                size="touch"
                variant="outline"
                disabled={currentTickAttackIDs.length === 0}
                onClick={replayCurrentTick}
              >
                <Zap className="h-4 w-4" />
                Highlight tick #{selectedTickValue}
              </Button>
            </div>
          </div>
        </div>
      ) : null}

      {expanded ? null : renderMapSurface(false)}

      <Dialog open={expanded} onOpenChange={setExpanded}>
        <DialogContent
          data-testid="attack-map-maximize-dialog"
          showCloseButton={false}
          className="h-[98dvh] w-[99vw] max-w-[99vw] grid-rows-[minmax(0,1fr)] p-0"
          onCloseAutoFocus={(event) => {
            event.preventDefault();
            maximizeTrigger.current?.focus();
          }}
          onEscapeKeyDown={(event) => {
            const controls = expandedLayout?.querySelector<HTMLDetailsElement>('[data-globe-controls][open]');
            if (!controls) return;
            event.preventDefault();
            controls.open = false;
            controls.querySelector('summary')?.focus();
          }}
        >
          {/* Chromium can leave a fixed scroll container blank when the dense globe opens from a scrolled page. */}
          <div
            ref={setExpandedLayout}
            className="grid min-w-0 grid-rows-[auto_minmax(min(10rem,35dvh),1fr)] gap-2 overflow-y-auto p-2 sm:p-3"
          >
            <DialogHeader className="min-w-0 flex-row flex-wrap items-center gap-x-4 gap-y-2">
              <div className="min-w-0 flex-1 basis-40">
                <DialogTitle className="wrap-anywhere">{title}</DialogTitle>
                <DialogDescription className="sr-only">{description}</DialogDescription>
              </div>
              {compactViewer ? null : expandedActions}
              <Button
                size="sm"
                variant="outline"
                className="any-pointer-coarse:min-h-11"
                onClick={() => setExpanded(false)}
              >
                <Minimize2 className="size-4" aria-hidden />
                Back to page
              </Button>
            </DialogHeader>
            <ScrollableRegion axis="vertical" label="Maximized attack map" className="@container/expanded-map min-h-0 overflow-y-auto">
              {renderMapSurface(true)}
            </ScrollableRegion>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function InspectorCard({
  children,
  className,
  eyebrow,
  lines,
  subtitle,
  title,
  ...props
}: {
  children?: ReactNode;
  className?: string;
  eyebrow: string;
  lines: Array<{ label: string; value: string }>;
  subtitle?: string;
  title: string;
} & React.HTMLAttributes<HTMLDivElement>): ReactElement {
  return (
    <div data-slot="well"
      className={cn("min-w-0 space-y-3 rounded-sm border border-border/70 bg-muted/20 p-3", className)}
      {...props}
    >
      <div className="space-y-1">
        <p className="text-xs font-medium text-muted-foreground">
          {eyebrow}
        </p>
        <p className="min-w-0 break-words text-base font-semibold text-foreground">{title}</p>
        {subtitle ? (
          <p
            data-testid="attack-inspector-institution"
            className="min-w-0 break-words text-sm text-muted-foreground"
          >
            {subtitle}
          </p>
        ) : null}
      </div>
      <div className="space-y-2 text-sm">
        {lines.map((line) => (
          <div key={line.label} className="grid grid-cols-[5rem_minmax(0,1fr)] items-start gap-3">
            <span className="text-muted-foreground">{line.label}</span>
            <span className="min-w-0 break-words text-right font-mono text-foreground">{line.value}</span>
          </div>
        ))}
      </div>
      {children ? <div className="space-y-3">{children}</div> : null}
    </div>
  );
}

function describeViewMode(mode: ViewMode): string {
  switch (mode) {
    case 'current':
      return 'Current Tick';
    case 'recent':
      return 'Recent';
    case 'all':
    default:
      return 'All Feed';
  }
}

function formatTickWindow(values: number[]): string {
  if (values.length === 0) {
    return 'no ticks';
  }
  const first = values[0];
  const last = values.at(-1) ?? first;
  return first === last ? `tick #${first}` : `ticks #${first}-#${last}`;
}

function getAdjacentTick(
  tickValues: number[],
  current: number,
  direction: 'prev' | 'next',
): number {
  const currentIndex = tickValues.indexOf(current);
  if (currentIndex === -1) {
    return tickValues.at(-1) ?? 0;
  }
  if (direction === 'prev') {
    return tickValues[(currentIndex - 1 + tickValues.length) % tickValues.length];
  }
  return tickValues[(currentIndex + 1) % tickValues.length];
}
