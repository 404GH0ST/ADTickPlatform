'use client';

import type { ReactElement, ReactNode } from 'react';
import { useEffect, useMemo, useRef, useState } from 'react';
import {
  ChevronLeft,
  ChevronRight,
  Expand,
  Minimize2,
  Pause,
  Play,
  Search,
  Zap,
} from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { EmptyStateText } from '@/components/ui/empty-state';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';

import { CyberAttackMap } from './cyber-attack-map';

export type AttackMapEvent = {
  id: string;
  attacker: string;
  victim: string;
  service: string;
  tick: number;
  verdict: string;
};

type ViewMode = 'all' | 'recent' | 'current';

const RECENT_TICK_WINDOW = 5;
const PLAYBACK_SPEEDS = [
  { label: 'Fast', value: 900 },
  { label: 'Normal', value: 1500 },
  { label: 'Slow', value: 2300 },
];

export function AttackMapPanel({
  attackRows,
  className,
  description = 'Directional attack flow across teams, services, and match ticks.',
  highlightedAttackIDs = [],
  title = 'Attack map',
}: {
  attackRows: AttackMapEvent[];
  className?: string;
  description?: string;
  highlightedAttackIDs?: string[];
  title?: string;
}): ReactElement {
  const [expanded, setExpanded] = useState(false);
  const [teamQuery, setTeamQuery] = useState('');
  const [replayAttackIDs, setReplayAttackIDs] = useState<string[]>([]);
  const [viewMode, setViewMode] = useState<ViewMode>('all');
  const [selectedTick, setSelectedTick] = useState<number | null>(null);
  const [playbackRunning, setPlaybackRunning] = useState(false);
  const [playbackSpeed, setPlaybackSpeed] = useState<number>(1500);
  const [selectedTeamId, setSelectedTeamId] = useState<string | null>(null);
  const [selectedAttackId, setSelectedAttackId] = useState<string | null>(null);
  const replayTimeoutRef = useRef<number | null>(null);
  const replayFrameRef = useRef<number | null>(null);

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

  const visibleRows = useMemo(() => {
    if (attackRows.length === 0 || selectedTickValue === 0) {
      return attackRows;
    }

    switch (viewMode) {
      case 'current':
        return attackRows.filter((row) => row.tick === selectedTickValue);
      case 'recent': {
        const windowStart = Math.max(0, selectedTickValue - (RECENT_TICK_WINDOW - 1));
        return attackRows.filter(
          (row) => row.tick >= windowStart && row.tick <= selectedTickValue,
        );
      }
      case 'all':
      default:
        return attackRows;
    }
  }, [attackRows, selectedTickValue, viewMode]);

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

  const selectedAttack = useMemo(
    () => attackRows.find((row) => row.id === selectedAttackId) ?? null,
    [attackRows, selectedAttackId],
  );
  const selectedTeamSummary = useMemo(() => {
    if (!selectedTeamId) {
      return null;
    }

    const involvedRows = visibleRows.filter(
      (row) => row.attacker === selectedTeamId || row.victim === selectedTeamId,
    );
    if (involvedRows.length === 0) {
      return {
        attacks: 0,
        inbound: 0,
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
    replayTick(selectedTickValue);
  }

  function moveTick(direction: 'prev' | 'next'): void {
    if (selectedTickValue === 0) {
      return;
    }
    setSelectedTick((current) =>
      getAdjacentTick(tickValues, current ?? latestTick, direction),
    );
  }

  function renderInspector(): ReactElement {
    if (selectedAttack) {
      return (
        <InspectorCard
          eyebrow="Attack"
          title={`${selectedAttack.attacker} -> ${selectedAttack.victim}`}
          lines={[
            { label: 'Tick', value: `#${selectedAttack.tick}` },
            { label: 'Service', value: selectedAttack.service },
            { label: 'Verdict', value: selectedAttack.verdict },
          ]}
        >
          <Button
            size="sm"
            variant="outline"
            onClick={() => setSelectedAttackId(null)}
          >
            Clear attack focus
          </Button>
        </InspectorCard>
      );
    }

    if (selectedTeamSummary) {
      return (
        <InspectorCard
          eyebrow="Team"
          title={selectedTeamSummary.team}
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

  function renderMapSurface(expandedView: boolean): ReactElement {
    return (
      <div
        className={cn(
          'grid gap-3',
          expandedView
            ? 'h-full min-h-0 grid-rows-[minmax(0,1fr)_minmax(0,20vh)] xl:grid-cols-[minmax(0,1fr)_22rem] xl:grid-rows-none'
            : 'grid-rows-[minmax(0,1fr)_auto] xl:grid-cols-[minmax(0,1fr)_20rem] xl:grid-rows-none',
        )}
      >
        <CyberAttackMap
          attacks={visibleRows}
          className={expandedView ? 'aspect-auto h-full min-h-[34rem]' : 'min-h-[24rem]'}
          focusedTeams={focusedTeams}
          highlightedAttackIDs={activeHighlightIDs}
          placementScopeTeams={allTeams}
          selectedAttackId={selectedAttackId}
          selectedTeamId={selectedTeamId}
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
        <div
          className={cn(
            'min-w-0',
            expandedView ? 'min-h-0 overflow-y-auto pr-1' : '',
          )}
        >
          {renderInspector()}
        </div>
      </div>
    );
  }

  return (
    <div
      data-testid="attack-map-panel"
      className={cn(
        'space-y-3 rounded-sm border border-border/70 bg-card p-3',
        className,
      )}
    >
      <div className="flex flex-col gap-3 border-b pb-3 xl:flex-row xl:items-start xl:justify-between">
        <div className="space-y-1">
          <p className="text-sm font-medium text-foreground">{title}</p>
          <p className="text-sm text-muted-foreground">{description}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Badge variant="secondary">{visibleRows.length} attacks</Badge>
          <Badge variant="secondary">{visibleTeamCount} teams</Badge>
          <Badge variant="secondary">{visibleServiceCount} services</Badge>
          {attackRows.length > 0 ? (
            <>
              <Button
                size="sm"
                variant="outline"
                disabled={currentTickAttackIDs.length === 0}
                onClick={replayCurrentTick}
              >
                <Zap className="h-4 w-4" />
                Replay tick #{selectedTickValue}
              </Button>
              <Button size="sm" variant="outline" onClick={() => setExpanded(true)}>
                <Expand className="h-4 w-4" />
                Maximize
              </Button>
            </>
          ) : null}
        </div>
      </div>

      {attackRows.length > 0 ? (
        <div className="grid gap-3 rounded-sm border border-border/70 bg-muted/20 p-3 xl:grid-cols-[minmax(0,1fr)_auto] xl:items-end">
          <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_auto_auto]">
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
              <p className="text-sm font-medium text-foreground">Map mode</p>
              <div className="flex flex-wrap gap-2">
                {(['all', 'recent', 'current'] as ViewMode[]).map((mode) => (
                  <Button
                    key={mode}
                    size="sm"
                    variant={viewMode === mode ? 'default' : 'outline'}
                    onClick={() => setViewMode(mode)}
                  >
                    {describeViewMode(mode)}
                  </Button>
                ))}
              </div>
            </div>
            <div className="space-y-2">
              <p className="text-sm font-medium text-foreground">Tick playback</p>
              <div className="flex flex-wrap items-center gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  disabled={tickValues.length === 0}
                  onClick={() => moveTick('prev')}
                  aria-label="Previous Tick"
                >
                  <ChevronLeft className="h-4 w-4" />
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={tickValues.length < 2}
                  onClick={() => setPlaybackRunning((current) => !current)}
                >
                  {playbackRunning ? (
                    <Pause className="h-4 w-4" />
                  ) : (
                    <Play className="h-4 w-4" />
                  )}
                  {playbackRunning ? 'Pause' : 'Play'}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={tickValues.length === 0}
                  onClick={() => moveTick('next')}
                  aria-label="Next Tick"
                >
                  <ChevronRight className="h-4 w-4" />
                </Button>
                <select
                  aria-label="Playback speed"
                  className="flex h-9 rounded-sm border border-input bg-card px-3 text-sm"
                  value={String(playbackSpeed)}
                  onChange={(event) => setPlaybackSpeed(Number(event.target.value))}
                >
                  {PLAYBACK_SPEEDS.map((speed) => (
                    <option key={speed.value} value={speed.value}>
                      {speed.label}
                    </option>
                  ))}
                </select>
              </div>
            </div>
          </div>
          <div className="space-y-2 xl:text-right">
            <p className="text-sm font-medium text-foreground">
              {selectedTickValue > 0 ? `Tick #${selectedTickValue}` : 'No ticks'}
            </p>
            <p className="text-sm text-muted-foreground">
              {normalizedTeamQuery === ''
                ? `${describeViewMode(viewMode)} view across ${formatTickWindow(visibleTicks)}.`
                : `Showing labels for ${searchedTeams.length} matching team(s).`}
            </p>
          </div>
        </div>
      ) : null}

      {attackRows.length === 0 ? (
        <EmptyStateText message="No accepted attacks in the current slice to plot." />
      ) : (
        renderMapSurface(false)
      )}

      <Dialog open={expanded} onOpenChange={setExpanded}>
        <DialogContent
          data-testid="attack-map-maximize-dialog"
          className="grid h-[98vh] w-[99vw] max-w-[99vw] grid-rows-[auto_minmax(0,1fr)] overflow-y-auto p-4 sm:p-6"
        >
          <DialogHeader>
            <DialogTitle>{title}</DialogTitle>
            <DialogDescription>{description}</DialogDescription>
          </DialogHeader>
          <div className="flex min-h-0 flex-col gap-4 overflow-y-auto">
            <div className="flex flex-wrap gap-2">
              <Badge variant="secondary">{visibleRows.length} attacks</Badge>
              <Badge variant="secondary">{visibleTeamCount} teams</Badge>
              <Badge variant="secondary">{visibleServiceCount} services</Badge>
              <Button
                size="sm"
                variant="outline"
                disabled={currentTickAttackIDs.length === 0}
                onClick={replayCurrentTick}
              >
                <Zap className="h-4 w-4" />
                Replay tick #{selectedTickValue}
              </Button>
              <Button
                size="sm"
                variant="outline"
                onClick={() => setExpanded(false)}
              >
                <Minimize2 className="h-4 w-4" />
                Back to page
              </Button>
            </div>
            <div className="min-h-0 flex-1">{renderMapSurface(true)}</div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function InspectorCard({
  children,
  eyebrow,
  lines,
  title,
}: {
  children?: ReactNode;
  eyebrow: string;
  lines: Array<{ label: string; value: string }>;
  title: string;
}): ReactElement {
  return (
    <div className="space-y-3 rounded-sm border border-border/70 bg-muted/20 p-3">
      <div className="space-y-1">
        <p className="text-xs font-medium text-muted-foreground">
          {eyebrow}
        </p>
        <p className="text-base font-semibold text-foreground">{title}</p>
      </div>
      <div className="space-y-2 text-sm">
        {lines.map((line) => (
          <div key={line.label} className="flex items-start justify-between gap-3">
            <span className="text-muted-foreground">{line.label}</span>
            <span className="text-right font-mono text-foreground">{line.value}</span>
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
