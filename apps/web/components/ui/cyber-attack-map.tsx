'use client';

import type { ReactElement } from 'react';
import { useEffect, useId, useMemo, useState } from 'react';

import { cn } from '@/lib/utils';

import { WORLD_MAP_PATH } from './world_path';

export type CyberAttackEvent = {
  id: string;
  attacker: string;
  victim: string;
  service: string;
  tick: number;
};

type TeamNode = {
  id: string;
  name: string;
  x: number;
  y: number;
  labelLineX: number;
  labelLineY: number;
  labelAnchor: 'start' | 'middle' | 'end';
  labelDx: number;
  labelDy: number;
  highlighted: boolean;
  selected: boolean;
  incoming: number;
  outgoing: number;
};

type AttackArc = {
  id: string;
  attacker: string;
  victim: string;
  service: string;
  tick: number;
  color: string;
  gradientId: string;
  highlighted: boolean;
  selected: boolean;
  active: boolean;
  path: string;
};

type TeamStats = {
  incoming: number;
  outgoing: number;
};

type LandSlot = {
  x: number;
  y: number;
};

type LandPlacementResources = {
  isLand: (x: number, y: number) => boolean;
  slots: LandSlot[];
};

type TeamNodeMarkerProps = {
  node: TeamNode;
  showLabel: boolean;
  pulseClassName: string;
  onHoverChange: (teamId: string | null) => void;
  onSelectTeam?: (teamId: string | null) => void;
};

const FALLBACK_LAND_SLOTS: LandSlot[] = [
  { x: 156, y: 128 }, { x: 236, y: 144 }, { x: 212, y: 104 }, { x: 196, y: 188 },
  { x: 304, y: 286 }, { x: 314, y: 382 }, { x: 292, y: 304 }, { x: 472, y: 126 },
  { x: 490, y: 150 }, { x: 536, y: 130 }, { x: 575, y: 124 }, { x: 596, y: 156 },
  { x: 490, y: 196 }, { x: 472, y: 244 }, { x: 546, y: 266 }, { x: 516, y: 382 },
  { x: 642, y: 184 }, { x: 684, y: 172 }, { x: 722, y: 194 }, { x: 752, y: 212 },
  { x: 792, y: 160 }, { x: 790, y: 204 }, { x: 804, y: 228 }, { x: 816, y: 250 },
  { x: 846, y: 170 }, { x: 850, y: 226 }, { x: 872, y: 184 }, { x: 850, y: 336 },
  { x: 892, y: 338 }, { x: 944, y: 362 },
];

const MAP_WIDTH = 1000;
const MAP_HEIGHT = 500;
const LAND_SLOT_LIMIT = 320;
const LAND_SCAN_START_X = 92;
const LAND_SCAN_END_X = 908;
const LAND_SCAN_START_Y = 72;
const LAND_SCAN_END_Y = 422;
const LAND_SCAN_STEP = 20;
const DEFAULT_TEAM_POINT: LandSlot = { x: 500, y: 250 };

let cachedLandPlacementResources: LandPlacementResources | null = null;
const cachedTeamPlacements = new Map<string, Record<string, LandSlot>>();

export function CyberAttackMap({
  attacks,
  highlightedAttackIDs = [],
  focusedTeams = [],
  placementScopeTeams,
  selectedAttackId = null,
  selectedTeamId = null,
  className,
  onSelectAttack,
  onSelectTeam,
}: {
  attacks: CyberAttackEvent[];
  highlightedAttackIDs?: string[];
  focusedTeams?: string[];
  placementScopeTeams?: string[];
  selectedAttackId?: string | null;
  selectedTeamId?: string | null;
  className?: string;
  onSelectAttack?: (attackId: string | null) => void;
  onSelectTeam?: (teamId: string | null) => void;
}): ReactElement {
  const [hoveredTeamId, setHoveredTeamId] = useState<string | null>(null);
  const [clientReady, setClientReady] = useState(false);
  const mapId = useId().replace(/:/g, '');
  const patternId = `${mapId}-dot-pattern`;
  const maskId = `${mapId}-map-mask`;
  const beamClassName = `cyber-beam-${mapId}`;
  const pulseClassName = `cyber-pulse-${mapId}`;
  const focusedTeamSet = useMemo(() => new Set(focusedTeams), [focusedTeams]);

  useEffect(() => {
    setClientReady(true);
  }, []);

  const { arcs, nodes, services } = useMemo(() => {
    const attackTeamNames = listAttackTeams(attacks);
    const placementTeamNames = listPlacementTeams(attackTeamNames, placementScopeTeams);
    const teamStats = buildTeamStats(attacks, attackTeamNames);

    const nodesByName: Record<string, TeamNode> = {};
    const pointsByTeam = clientReady
      ? getCachedTeamPlacements(placementTeamNames)
      : {};

    attackTeamNames.forEach((name) => {
      const point = pointsByTeam[name] ?? DEFAULT_TEAM_POINT;
      const labelPlacement = getLabelPlacement(point.x, point.y);
      const stats = teamStats.get(name) ?? { incoming: 0, outgoing: 0 };

      nodesByName[name] = {
        id: name,
        name,
        x: point.x,
        y: point.y,
        labelLineX: labelPlacement.lineX,
        labelLineY: labelPlacement.lineY,
        labelAnchor: labelPlacement.anchor,
        labelDx: labelPlacement.dx,
        labelDy: labelPlacement.dy,
        highlighted: false,
        selected: name === selectedTeamId,
        incoming: stats.incoming,
        outgoing: stats.outgoing,
      };
    });

    const highlightedSet = new Set(highlightedAttackIDs);
    const serviceList = listAttackServices(attacks);
    const serviceColors: Record<string, string> = {};
    serviceList.forEach((service, index) => {
      serviceColors[service] = attackPalette[index % attackPalette.length];
    });
    const serviceGradientIds = new Map(
      serviceList.map((service, index) => [service, `${mapId}-grad-${index}`]),
    );

    const arcs = attacks.map((attack) => {
      const start = nodesByName[attack.attacker];
      const end = nodesByName[attack.victim];
      const highlighted = highlightedSet.has(attack.id);
      const selected = selectedAttackId === attack.id;
      const active =
        highlighted ||
        selected ||
        attack.attacker === selectedTeamId ||
        attack.victim === selectedTeamId;

      if (active) {
        start.highlighted = true;
        end.highlighted = true;
      }

      const midX = (start.x + end.x) / 2;
      const midY = (start.y + end.y) / 2 - Math.abs(start.x - end.x) * 0.15 - 40;

      return {
        id: attack.id,
        attacker: attack.attacker,
        victim: attack.victim,
        service: attack.service,
        tick: attack.tick,
        color: serviceColors[attack.service],
        gradientId:
          serviceGradientIds.get(attack.service) ?? `${mapId}-grad-fallback`,
        highlighted,
        selected,
        active,
        path: `M ${start.x} ${start.y} Q ${midX} ${midY} ${end.x} ${end.y}`,
      } satisfies AttackArc;
    });

    return {
      arcs: arcs.sort((left, right) => Number(left.active) - Number(right.active)),
      nodes: Object.values(nodesByName),
      services: serviceList.map((service) => ({
        color: serviceColors[service],
        gradientId: serviceGradientIds.get(service) ?? `${mapId}-grad-fallback`,
        name: service,
      })),
    };
  }, [
    attacks,
    clientReady,
    highlightedAttackIDs,
    mapId,
    placementScopeTeams,
    selectedAttackId,
    selectedTeamId,
  ]);

  return (
    <div
      className={cn(
        'relative aspect-[2/1] w-full overflow-hidden rounded-xl border',
        className,
      )}
      style={{
        backgroundColor: 'var(--attack-map-background)',
        borderColor: 'var(--attack-map-border)',
      }}
    >
      <svg
        viewBox="0 0 1000 500"
        preserveAspectRatio="xMidYMid meet"
        className="pointer-events-none absolute inset-0 h-full w-full"
        xmlns="http://www.w3.org/2000/svg"
      >
        <defs>
          <pattern id={patternId} width="12" height="12" patternUnits="userSpaceOnUse">
            <circle cx="2" cy="2" r="1" fill="var(--attack-map-grid)" />
          </pattern>
          <mask id={maskId}>
            <path d={WORLD_MAP_PATH} fill="white" />
          </mask>
        </defs>
        <rect width="1000" height="500" fill={`url(#${patternId})`} opacity="0.28" />
        <rect
          width="1000"
          height="500"
          fill="var(--attack-map-land)"
          mask={`url(#${maskId})`}
          opacity="0.42"
        />
        <path
          d={WORLD_MAP_PATH}
          fill="none"
          stroke="var(--attack-map-outline)"
          strokeWidth="1"
          opacity="0.7"
        />
      </svg>

      <svg
        viewBox="0 0 1000 500"
        preserveAspectRatio="xMidYMid meet"
        className="absolute inset-0 h-full w-full"
        xmlns="http://www.w3.org/2000/svg"
      >
        <defs>
          {services.map((service) => (
            <linearGradient
              key={service.gradientId}
              id={service.gradientId}
              x1="0%"
              y1="0%"
              x2="100%"
              y2="0%"
            >
              <stop offset="0%" stopColor={service.color} stopOpacity="0" />
              <stop offset="50%" stopColor={service.color} stopOpacity="1" />
              <stop offset="100%" stopColor={service.color} stopOpacity="0" />
            </linearGradient>
          ))}
        </defs>

        {arcs.map((arc) => {
          const beamVisible = arc.highlighted || arc.selected;
          return (
            <g key={arc.id}>
              <path
                d={arc.path}
                fill="none"
                stroke={arc.color}
                strokeOpacity={arc.active ? '0.42' : '0.12'}
                strokeWidth={arc.active ? '2.2' : '1.1'}
              />
              {beamVisible ? (
                <>
                  <path
                    d={arc.path}
                    fill="none"
                    stroke={arc.color}
                    strokeWidth="2.5"
                    strokeOpacity="0.22"
                  />
                  <path
                    d={arc.path}
                    fill="none"
                    stroke={`url(#${arc.gradientId})`}
                    strokeWidth="4.5"
                    strokeDasharray="40, 960"
                    className={beamClassName}
                    style={{ filter: `drop-shadow(0 0 3px ${arc.color})` }}
                  />
                </>
              ) : null}
              <path
                d={arc.path}
                fill="none"
                stroke="transparent"
                strokeWidth="12"
                pointerEvents="stroke"
                className="cursor-pointer"
                onClick={() => onSelectAttack?.(arc.selected ? null : arc.id)}
              />
            </g>
          );
        })}

        {nodes.map((node) => (
          <TeamNodeMarker
            key={node.id}
            node={node}
            showLabel={shouldShowTeamLabel(node, hoveredTeamId, focusedTeamSet)}
            pulseClassName={pulseClassName}
            onHoverChange={setHoveredTeamId}
            onSelectTeam={onSelectTeam}
          />
        ))}
      </svg>

      <div className="pointer-events-none absolute bottom-4 left-4 flex flex-wrap gap-2">
        {services.map((service) => (
          <div
            key={service.name}
            className="flex items-center gap-1.5 rounded border px-2 py-1 backdrop-blur-sm"
            style={{
              backgroundColor: 'var(--attack-map-legend-background)',
              borderColor: 'var(--attack-map-legend-border)',
            }}
          >
            <span className="h-2 w-2 rounded-full" style={{ backgroundColor: service.color }} />
            <span
              className="text-[10px] font-bold uppercase tracking-widest"
              style={{ color: 'var(--attack-map-legend-foreground)' }}
            >
              {service.name}
            </span>
          </div>
        ))}
      </div>

      <div
        className="pointer-events-none absolute inset-0 bg-[length:100%_4px]"
        style={{
          backgroundImage:
            'linear-gradient(rgba(0, 0, 0, 0) 50%, var(--attack-map-scanline) 50%)',
        }}
      />

      <style
        dangerouslySetInnerHTML={{
          __html: `
        @keyframes ${beamClassName}-keyframes {
          0% { stroke-dashoffset: 1000; }
          100% { stroke-dashoffset: 0; }
        }
        @keyframes ${pulseClassName}-keyframes {
          0% { transform: scale(0.96); opacity: 0.2; }
          70% { transform: scale(1.8); opacity: 0; }
          100% { transform: scale(1.8); opacity: 0; }
        }
        .${beamClassName} {
          animation: ${beamClassName}-keyframes 2s linear infinite;
        }
        .${pulseClassName} {
          transform-origin: center;
          animation: ${pulseClassName}-keyframes 1.4s ease-out infinite;
        }
        @media (prefers-reduced-motion: reduce) {
          .${beamClassName},
          .${pulseClassName} {
            animation: none !important;
          }
        }
      `,
        }}
      />
    </div>
  );
}

function TeamNodeMarker({
  node,
  showLabel,
  pulseClassName,
  onHoverChange,
  onSelectTeam,
}: TeamNodeMarkerProps): ReactElement {
  const emphasized = node.selected || node.highlighted;

  return (
    <g
      transform={`translate(${node.x}, ${node.y})`}
      className="cursor-pointer"
      onClick={() => onSelectTeam?.(node.selected ? null : node.id)}
      onMouseEnter={() => onHoverChange(node.id)}
      onMouseLeave={() => onHoverChange(null)}
    >
      <circle
        r={getNodeHaloRadius(node)}
        fill="var(--attack-map-node-highlight-halo)"
        fillOpacity={getNodeHaloOpacity(node, showLabel)}
        className={emphasized ? pulseClassName : ''}
      />
      <circle
        r={getNodeCoreRadius(node, showLabel)}
        fill={showLabel ? 'var(--attack-map-node-highlight)' : 'var(--attack-map-node)'}
        style={getNodeCoreStyle(showLabel)}
      />
      {showLabel ? <TeamNodeLabelLine node={node} emphasized={emphasized} /> : null}
      {showLabel ? <TeamNodeLabel node={node} emphasized={emphasized} /> : null}
    </g>
  );
}

function TeamNodeLabelLine({
  node,
  emphasized,
}: {
  node: TeamNode;
  emphasized: boolean;
}): ReactElement {
  return (
    <line
      x1="0"
      y1="-1"
      x2={node.labelLineX}
      y2={node.labelLineY}
      stroke="var(--attack-map-node-label)"
      strokeOpacity={emphasized ? '0.95' : '0.58'}
      strokeWidth={node.selected ? '1.35' : '0.9'}
    />
  );
}

function TeamNodeLabel({
  node,
  emphasized,
}: {
  node: TeamNode;
  emphasized: boolean;
}): ReactElement {
  return (
    <text
      x={node.labelDx}
      y={emphasized ? node.labelDy - 2 : node.labelDy}
      textAnchor={node.labelAnchor}
      fill={emphasized ? 'var(--foreground)' : 'var(--attack-map-node-label)'}
      stroke="var(--attack-map-node-label-halo)"
      strokeWidth={emphasized ? '3' : '2.25'}
      paintOrder="stroke"
      strokeLinejoin="round"
      fontSize={getNodeLabelFontSize(node)}
      fontWeight={emphasized ? '600' : '500'}
      className="pointer-events-none select-none drop-shadow-lg font-sans"
    >
      {node.name}
    </text>
  );
}

function shouldShowTeamLabel(
  node: TeamNode,
  hoveredTeamId: string | null,
  focusedTeamSet: Set<string>,
): boolean {
  return (
    node.highlighted ||
    node.selected ||
    hoveredTeamId === node.id ||
    focusedTeamSet.has(node.id)
  );
}

function getNodeHaloRadius(node: TeamNode): number {
  if (node.selected) {
    return 12;
  }
  return node.highlighted ? 10 : 4;
}

function getNodeHaloOpacity(node: TeamNode, showLabel: boolean): number {
  if (showLabel) {
    return 0.2;
  }
  return node.highlighted ? 0.12 : 0.04;
}

function getNodeCoreRadius(node: TeamNode, showLabel: boolean): number {
  if (showLabel) {
    return 3.8;
  }
  return node.highlighted || node.selected ? 4 : 2.5;
}

function getNodeCoreStyle(showLabel: boolean): { filter: string } | undefined {
  if (!showLabel) {
    return undefined;
  }
  return { filter: 'drop-shadow(0 0 8px var(--attack-map-node-highlight))' };
}

function getNodeLabelFontSize(node: TeamNode): string {
  if (node.selected) {
    return '12.5';
  }
  return node.highlighted ? '12' : '10';
}

const attackPalette = [
  '#2dd4bf',
  '#3b82f6',
  '#f59e0b',
  '#ef4444',
  '#a855f7',
  '#10b981',
  '#f97316',
  '#ec4899',
];

function hashTeamName(name: string): number {
  let hash = 5381;
  for (let index = 0; index < name.length; index += 1) {
    hash = ((hash << 5) + hash + name.charCodeAt(index)) >>> 0;
  }
  return hash;
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

function getLabelPlacement(
  x: number,
  y: number,
): {
  anchor: 'start' | 'middle' | 'end';
  dx: number;
  dy: number;
  lineX: number;
  lineY: number;
} {
  if (x < 120) {
    return { anchor: 'start', dx: 10, dy: -10, lineX: 6, lineY: -6 };
  }
  if (x > 880) {
    return { anchor: 'end', dx: -10, dy: -10, lineX: -6, lineY: -6 };
  }
  if (y > 360) {
    return { anchor: 'middle', dx: 0, dy: -16, lineX: 0, lineY: -8 };
  }
  return { anchor: 'middle', dx: 0, dy: -14, lineX: 0, lineY: -8 };
}

function createFallbackPlacementResources(): LandPlacementResources {
  return {
    slots: FALLBACK_LAND_SLOTS,
    isLand: () => true,
  };
}

function listAttackServices(attacks: CyberAttackEvent[]): string[] {
  return Array.from(new Set(attacks.map((attack) => attack.service))).sort();
}

function listAttackTeams(attacks: CyberAttackEvent[]): string[] {
  return Array.from(
    new Set([
      ...attacks.map((attack) => attack.attacker),
      ...attacks.map((attack) => attack.victim),
    ]),
  ).sort();
}

function listPlacementTeams(
  attackTeamNames: string[],
  placementScopeTeams?: string[],
): string[] {
  if (!placementScopeTeams || placementScopeTeams.length === 0) {
    return attackTeamNames;
  }

  return Array.from(
    new Set([...placementScopeTeams, ...attackTeamNames]),
  ).sort();
}

function buildTeamStats(
  attacks: CyberAttackEvent[],
  attackTeamNames: string[],
): Map<string, TeamStats> {
  const stats = new Map<string, TeamStats>();
  attackTeamNames.forEach((name) => {
    stats.set(name, { incoming: 0, outgoing: 0 });
  });

  attacks.forEach((attack) => {
    const attackerStats = stats.get(attack.attacker);
    const victimStats = stats.get(attack.victim);
    if (attackerStats) {
      attackerStats.outgoing += 1;
    }
    if (victimStats) {
      victimStats.incoming += 1;
    }
  });

  return stats;
}

function getLandPlacementResources(): LandPlacementResources {
  if (cachedLandPlacementResources) {
    return cachedLandPlacementResources;
  }

  if (typeof document === 'undefined' || typeof Path2D === 'undefined') {
    cachedLandPlacementResources = createFallbackPlacementResources();
    return cachedLandPlacementResources;
  }

  const canvas = document.createElement('canvas');
  canvas.width = MAP_WIDTH;
  canvas.height = MAP_HEIGHT;
  const context = canvas.getContext('2d');
  if (!context) {
    cachedLandPlacementResources = createFallbackPlacementResources();
    return cachedLandPlacementResources;
  }

  const landPath = new Path2D(WORLD_MAP_PATH);
  const isRawLand = (x: number, y: number) =>
    context.isPointInPath(landPath, x, y);
  const isDeepLand = (x: number, y: number) => {
    const checks = [
      [0, 0],
      [8, 0],
      [-8, 0],
      [0, 8],
      [0, -8],
      [6, 6],
      [-6, 6],
      [6, -6],
      [-6, -6],
    ];

    return checks.every(([dx, dy]) => {
      const px = clamp(x + dx, 1, MAP_WIDTH - 1);
      const py = clamp(y + dy, 1, MAP_HEIGHT - 1);
      return isRawLand(px, py);
    });
  };

  const generatedSlots: LandSlot[] = [];
  for (let y = LAND_SCAN_START_Y; y <= LAND_SCAN_END_Y; y += LAND_SCAN_STEP) {
    const rowIndex = Math.floor((y - LAND_SCAN_START_Y) / LAND_SCAN_STEP);
    const rowOffset = rowIndex % 2 === 0 ? 0 : LAND_SCAN_STEP / 2;
    for (
      let x = LAND_SCAN_START_X + rowOffset;
      x <= LAND_SCAN_END_X;
      x += LAND_SCAN_STEP
    ) {
      if (isDeepLand(x, y)) {
        generatedSlots.push({ x, y });
      }
    }
  }

  const slots =
    generatedSlots.length >= FALLBACK_LAND_SLOTS.length
      ? interleaveLandSlots(generatedSlots).slice(0, LAND_SLOT_LIMIT)
      : FALLBACK_LAND_SLOTS;

  cachedLandPlacementResources = {
    slots,
    isLand: isDeepLand,
  };
  return cachedLandPlacementResources;
}

function getCachedTeamPlacements(teamNames: string[]): Record<string, LandSlot> {
  const cacheKey = teamNames.join('\u001f');
  const cachedPlacements = cachedTeamPlacements.get(cacheKey);
  if (cachedPlacements) {
    return cachedPlacements;
  }

  const placement = getLandPlacementResources();
  const pointsByTeam: Record<string, LandSlot> = {};
  const usedSlots = new Set<number>();
  const overflowCounts = new Map<number, number>();
  const sortedPlacementTeams = sortTeamsForPlacement(teamNames);

  sortedPlacementTeams.forEach((name) => {
    const hash = hashTeamName(name);
    const slotIndex = pickSlotIndex(hash, placement.slots.length, usedSlots);
    const baseSlot = placement.slots[slotIndex];
    let repeatIndex = 0;
    if (!usedSlots.has(slotIndex)) {
      usedSlots.add(slotIndex);
    } else {
      repeatIndex = (overflowCounts.get(slotIndex) ?? 0) + 1;
      overflowCounts.set(slotIndex, repeatIndex);
    }

    pointsByTeam[name] = ensureLandPoint(
      getSlotPoint(baseSlot, repeatIndex, hash, placement.isLand),
      placement.slots,
      placement.isLand,
    );
  });

  cachedTeamPlacements.set(cacheKey, pointsByTeam);
  return pointsByTeam;
}

function sortTeamsForPlacement(teamNames: string[]): string[] {
  return [...teamNames].sort((left, right) => {
    const leftHash = hashTeamName(left);
    const rightHash = hashTeamName(right);
    return leftHash - rightHash || left.localeCompare(right);
  });
}

function interleaveLandSlots(slots: LandSlot[]): LandSlot[] {
  const cellWidth = (LAND_SCAN_END_X - LAND_SCAN_START_X) / 8;
  const cellHeight = (LAND_SCAN_END_Y - LAND_SCAN_START_Y) / 4;
  const buckets = new Map<string, LandSlot[]>();

  for (const slot of slots) {
    const xIndex = clamp(
      Math.floor((slot.x - LAND_SCAN_START_X) / cellWidth),
      0,
      7,
    );
    const yIndex = clamp(
      Math.floor((slot.y - LAND_SCAN_START_Y) / cellHeight),
      0,
      3,
    );
    const key = `${xIndex}:${yIndex}`;
    const values = buckets.get(key) ?? [];
    values.push(slot);
    buckets.set(key, values);
  }

  const keyOrder = Array.from(buckets.keys()).sort((left, right) => {
    const [lx, ly] = left.split(':').map((value) => Number(value));
    const [rx, ry] = right.split(':').map((value) => Number(value));
    const leftDistance = Math.abs(lx - 3.5) + Math.abs(ly - 1.5);
    const rightDistance = Math.abs(rx - 3.5) + Math.abs(ry - 1.5);
    return leftDistance - rightDistance || ly - ry || lx - rx;
  });

  keyOrder.forEach((key) => {
    const values = buckets.get(key);
    if (!values) {
      return;
    }
    values.sort((left, right) => left.y - right.y || left.x - right.x);
  });

  const result: LandSlot[] = [];
  let remaining = true;
  while (remaining) {
    remaining = false;
    for (const key of keyOrder) {
      const values = buckets.get(key);
      if (!values || values.length === 0) {
        continue;
      }
      remaining = true;
      result.push(values.shift() as LandSlot);
    }
  }

  return result;
}

function pickSlotIndex(
  hash: number,
  slotCount: number,
  usedSlots: Set<number>,
): number {
  if (slotCount <= 0) {
    return 0;
  }

  const start = hash % slotCount;
  const stride = pickStride(slotCount, hash);

  for (let attempt = 0; attempt < slotCount; attempt += 1) {
    const index = (start + attempt * stride) % slotCount;
    if (!usedSlots.has(index)) {
      return index;
    }
  }

  return start;
}

function pickStride(slotCount: number, hash: number): number {
  const candidates = [97, 89, 83, 79, 73, 71, 67, 61, 59, 53, 47];
  for (const candidate of candidates) {
    if (candidate >= slotCount) {
      continue;
    }
    if (gcd(candidate, slotCount) === 1) {
      return candidate;
    }
  }
  const fallback = (hash % Math.max(2, slotCount - 1)) + 1;
  return gcd(fallback, slotCount) === 1 ? fallback : 1;
}

function gcd(left: number, right: number): number {
  let a = Math.abs(left);
  let b = Math.abs(right);
  while (b !== 0) {
    const next = a % b;
    a = b;
    b = next;
  }
  return a;
}

function getSlotPoint(
  slot: LandSlot,
  repeatIndex: number,
  hash: number,
  isLand: (x: number, y: number) => boolean,
): LandSlot {
  if (repeatIndex === 0) {
    return { x: slot.x, y: slot.y };
  }

  const goldenAngle = 2.399963229728653;
  for (let attempt = 0; attempt < 28; attempt += 1) {
    const radius = 3 + repeatIndex * 1.7 + attempt * 0.35;
    const angle = hash * 0.011 + repeatIndex * goldenAngle + attempt * 0.32;
    const candidate = {
      x: clamp(slot.x + Math.cos(angle) * radius, 34, 966),
      y: clamp(slot.y + Math.sin(angle) * radius, 34, 456),
    };
    if (isLand(candidate.x, candidate.y)) {
      return candidate;
    }
  }

  return { x: slot.x, y: slot.y };
}

function ensureLandPoint(
  point: LandSlot,
  slots: LandSlot[],
  isLand: (x: number, y: number) => boolean,
): LandSlot {
  if (isLand(point.x, point.y)) {
    return point;
  }
  if (slots.length === 0) {
    return point;
  }

  let nearest = slots[0];
  let bestDistance = Number.POSITIVE_INFINITY;
  for (const slot of slots) {
    const dx = slot.x - point.x;
    const dy = slot.y - point.y;
    const distance = dx * dx + dy * dy;
    if (distance < bestDistance) {
      bestDistance = distance;
      nearest = slot;
    }
  }

  return { x: nearest.x, y: nearest.y };
}
