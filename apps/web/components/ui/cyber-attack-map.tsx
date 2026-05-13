'use client';

import type { PointerEvent as ReactPointerEvent, ReactElement } from 'react';
import { useEffect, useId, useMemo, useRef, useState } from 'react';

import globeJson from '@/data/globe.json';
import { cn } from '@/lib/utils';

export type CyberAttackEvent = {
  id: string;
  attacker: string;
  victim: string;
  service: string;
  tick: number;
};

type GlobeRotation = {
  lat: number;
  lon: number;
};

type GeoPoint = {
  lat: number;
  lon: number;
};

type ProjectedPoint = GeoPoint & {
  visible: boolean;
  x: number;
  y: number;
  z: number;
};

type Landmark = GeoPoint & {
  code: string;
  name: string;
};

type GlobeFeatureCollection = {
  features: Array<{
    geometry: {
      coordinates: unknown;
      type: 'MultiPolygon' | 'Polygon';
    };
    properties?: {
      name?: string;
    };
    type: 'Feature';
  }>;
  type: 'FeatureCollection';
};

type CountryRing = {
  id: string;
  name: string;
  points: GeoPoint[];
};

type TeamNode = ProjectedPoint & {
  id: string;
  name: string;
  landmark: Landmark;
  labelX: number;
  labelY: number;
  labelAnchor: 'start' | 'middle' | 'end';
  highlighted: boolean;
  selected: boolean;
  incoming: number;
  outgoing: number;
};

type AttackArc = {
  id: string;
  domId: string;
  attacker: string;
  victim: string;
  service: string;
  tick: number;
  color: string;
  highlighted: boolean;
  selected: boolean;
  active: boolean;
  path: string;
  delay: string;
  partial: boolean;
  visible: boolean;
};

type TeamStats = {
  incoming: number;
  outgoing: number;
};

type DragState = {
  pointerId: number;
  startLat: number;
  startLon: number;
  x: number;
  y: number;
};

const MAP_WIDTH = 1000;
const MAP_HEIGHT = 560;
const CENTER_X = MAP_WIDTH / 2 - 22;
const CENTER_Y = MAP_HEIGHT / 2 + 2;
const GLOBE_RADIUS = 256;
const MAX_ROTATION_LAT = 62;
const AUTO_ROTATE_IDLE_MS = 10_000;
const AUTO_ROTATE_STEP_MS = 4_200;
const LABEL_HEIGHT = 40;
const LABEL_PADDING_X = 6;
const LABEL_SAFE_INSET_X = 24;
const LABEL_SAFE_INSET_Y = 28;
const LABEL_VERTICAL_GAP = 14;

const LANDMARKS: Landmark[] = [
  { code: 'usa-central', name: 'United States', lat: 39.6, lon: -98.5 },
  { code: 'uk-midlands', name: 'United Kingdom', lat: 52.6, lon: -1.5 },
  { code: 'java-central', name: 'Indonesia', lat: -7.4, lon: 110.0 },
  { code: 'usa-east', name: 'US East', lat: 39.4, lon: -77.8 },
  { code: 'malaysia-peninsula', name: 'Malaysia', lat: 4.2, lon: 102.0 },
  { code: 'germany-central', name: 'Germany', lat: 51.0, lon: 10.2 },
  { code: 'australia-inland', name: 'Australia', lat: -25.3, lon: 134.2 },
  { code: 'canada-ontario', name: 'Canada', lat: 50.5, lon: -86.0 },
  { code: 'arabia', name: 'Arabian Peninsula', lat: 23.7, lon: 45.3 },
  { code: 'brazil-central', name: 'Brazil', lat: -10.8, lon: -52.7 },
  { code: 'japan-honshu', name: 'Japan', lat: 37.0, lon: 138.5 },
  { code: 'east-africa', name: 'East Africa', lat: 0.4, lon: 37.8 },
  { code: 'usa-west', name: 'US West', lat: 39.2, lon: -115.0 },
  { code: 'india-central', name: 'India', lat: 22.5, lon: 79.0 },
  { code: 'south-africa', name: 'South Africa', lat: -29.0, lon: 24.0 },
  { code: 'south-china', name: 'South China', lat: 25.0, lon: 110.0 },
  { code: 'benelux', name: 'Benelux', lat: 51.5, lon: 5.2 },
  { code: 'france-central', name: 'France', lat: 46.6, lon: 2.4 },
];

const COUNTRY_RINGS = parseCountryRings(globeJson as GlobeFeatureCollection);

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
  const [rotation, setRotation] = useState<GlobeRotation>({ lat: -6, lon: 34 });
  const dragRef = useRef<DragState | null>(null);
  const autoIntervalRef = useRef<number | null>(null);
  const autoTargetIndexRef = useRef(0);
  const animationFrameRef = useRef<number | null>(null);
  const idleTimeoutRef = useRef<number | null>(null);
  const rotationRef = useRef<GlobeRotation>(rotation);
  const mapId = useId().replace(/:/g, '');
  const clipId = `${mapId}-globe-clip`;
  const markerId = `${mapId}-arrow`;
  const beamClassName = `globe-beam-${mapId}`;
  const haloClassName = `globe-halo-${mapId}`;
  const focusedTeamSet = useMemo(() => new Set(focusedTeams), [focusedTeams]);
  const hasFocus =
    selectedAttackId !== null ||
    selectedTeamId !== null ||
    highlightedAttackIDs.length > 0 ||
    focusedTeams.length > 0;
  const autoTargets = useMemo(() => {
    const attackTeamNames = listAttackTeams(attacks);
    const placementTeamNames = listPlacementTeams(attackTeamNames, placementScopeTeams);
    const landmarksByTeam = getLandmarkPlacements(placementTeamNames);
    return attackTeamNames.flatMap((teamName) => {
      const landmark = landmarksByTeam[teamName];
      return landmark ? [landmark] : [];
    });
  }, [attacks, placementScopeTeams]);

  const { arcs, nodes, services } = useMemo(() => {
    const attackTeamNames = listAttackTeams(attacks);
    const placementTeamNames = listPlacementTeams(attackTeamNames, placementScopeTeams);
    const teamStats = buildTeamStats(attacks, attackTeamNames);
    const landmarksByTeam = getLandmarkPlacements(placementTeamNames);
    const highlightedSet = new Set(highlightedAttackIDs);
    const serviceNames = listAttackServices(attacks);
    const serviceColors = buildServiceColors(serviceNames);

    const nodesByName: Record<string, TeamNode> = {};
    attackTeamNames.forEach((name) => {
      const landmark = landmarksByTeam[name] ?? LANDMARKS[0];
      const point = projectGlobePoint(landmark, rotation);
      const label = getLabelPlacement(point);
      const stats = teamStats.get(name) ?? { incoming: 0, outgoing: 0 };
      const selected = name === selectedTeamId;
      const highlighted = focusedTeamSet.has(name);

      nodesByName[name] = {
        ...point,
        id: name,
        name,
        landmark,
        labelX: label.x,
        labelY: label.y,
        labelAnchor: label.anchor,
        highlighted,
        selected,
        incoming: stats.incoming,
        outgoing: stats.outgoing,
      };
    });

    const arcs = attacks.map((attack, index) => {
      const start = nodesByName[attack.attacker];
      const end = nodesByName[attack.victim];
      const highlighted = highlightedSet.has(attack.id);
      const selected = selectedAttackId === attack.id;
      const active =
        highlighted ||
        selected ||
        focusedTeamSet.has(attack.attacker) ||
        focusedTeamSet.has(attack.victim) ||
        attack.attacker === selectedTeamId ||
        attack.victim === selectedTeamId;

      if (active) {
        start.highlighted = true;
        end.highlighted = true;
      }

      return {
        id: attack.id,
        domId: `${mapId}-arc-${index}`,
        attacker: attack.attacker,
        victim: attack.victim,
        service: attack.service,
        tick: attack.tick,
        color: serviceColors[attack.service],
        highlighted,
        selected,
        active,
        path: buildAttackPath(start, end, attack.id),
        delay: `${(index % 9) * 0.32}s`,
        partial: start.visible !== end.visible,
        visible: start.visible || end.visible,
      } satisfies AttackArc;
    });

    return {
      arcs: arcs.sort((left, right) => Number(left.active) - Number(right.active)),
      nodes: resolveLabelCollisions(Object.values(nodesByName)).sort((left, right) => left.z - right.z),
      services: serviceNames.map((service) => ({
        color: serviceColors[service],
        name: service,
      })),
    };
  }, [
    attacks,
    focusedTeamSet,
    highlightedAttackIDs,
    mapId,
    placementScopeTeams,
    rotation,
    selectedAttackId,
    selectedTeamId,
  ]);
  const showAllLabels = nodes.length > 0 && nodes.length <= 18;
  const motionEnabled = attacks.length <= 160;

  useEffect(() => {
    rotationRef.current = rotation;
  }, [rotation]);

  useEffect(() => {
    startAutoRotationFromIdle();
    return clearAutoRotationTimers;
  }, [autoTargets]);

  function clearAutoRotationTimers(): void {
    if (idleTimeoutRef.current !== null) {
      window.clearTimeout(idleTimeoutRef.current);
      idleTimeoutRef.current = null;
    }
    if (autoIntervalRef.current !== null) {
      window.clearInterval(autoIntervalRef.current);
      autoIntervalRef.current = null;
    }
    cancelRotationAnimation();
  }

  function cancelRotationAnimation(): void {
    if (animationFrameRef.current !== null) {
      window.cancelAnimationFrame(animationFrameRef.current);
      animationFrameRef.current = null;
    }
  }

  function markUserActivity(): void {
    clearAutoRotationTimers();
    startAutoRotationFromIdle();
  }

  function rotateToNextAutoTarget(): void {
    if (autoTargets.length === 0) {
      return;
    }
    const target = autoTargets[autoTargetIndexRef.current % autoTargets.length];
    autoTargetIndexRef.current += 1;
    animateRotationTo({
      lat: clamp(target.lat, -MAX_ROTATION_LAT, MAX_ROTATION_LAT),
      lon: target.lon,
    });
  }

  function animateRotationTo(target: GlobeRotation): void {
    cancelRotationAnimation();
    const start = rotationRef.current;
    const startTime = window.performance.now();
    const duration = 1800;
    const deltaLon = normalizeLongitude(target.lon - start.lon);
    const deltaLat = target.lat - start.lat;

    function tick(timestamp: number): void {
      const progress = clamp((timestamp - startTime) / duration, 0, 1);
      const eased = 1 - Math.pow(1 - progress, 3);
      const nextRotation = {
        lat: start.lat + deltaLat * eased,
        lon: normalizeLongitude(start.lon + deltaLon * eased),
      };
      rotationRef.current = nextRotation;
      setRotation(nextRotation);
      if (progress < 1) {
        animationFrameRef.current = window.requestAnimationFrame(tick);
      } else {
        animationFrameRef.current = null;
      }
    }

    animationFrameRef.current = window.requestAnimationFrame(tick);
  }

  function startAutoRotationFromIdle(): void {
    clearAutoRotationTimers();
    if (autoTargets.length === 0) {
      return;
    }
    idleTimeoutRef.current = window.setTimeout(() => {
      if (dragRef.current !== null) {
        startAutoRotationFromIdle();
        return;
      }
      rotateToNextAutoTarget();
      autoIntervalRef.current = window.setInterval(
        rotateToNextAutoTarget,
        AUTO_ROTATE_STEP_MS,
      );
    }, AUTO_ROTATE_IDLE_MS);
  }

  function handlePointerDown(event: ReactPointerEvent<HTMLDivElement>): void {
    markUserActivity();
    event.currentTarget.setPointerCapture(event.pointerId);
    dragRef.current = {
      pointerId: event.pointerId,
      startLat: rotation.lat,
      startLon: rotation.lon,
      x: event.clientX,
      y: event.clientY,
    };
  }

  function handlePointerMove(event: ReactPointerEvent<HTMLDivElement>): void {
    markUserActivity();
    const drag = dragRef.current;
    if (!drag || drag.pointerId !== event.pointerId) {
      return;
    }
    const nextLon = normalizeLongitude(drag.startLon - (event.clientX - drag.x) * 0.42);
    const nextLat = clamp(drag.startLat + (event.clientY - drag.y) * 0.34, -MAX_ROTATION_LAT, MAX_ROTATION_LAT);
    const nextRotation = { lat: nextLat, lon: nextLon };
    rotationRef.current = nextRotation;
    setRotation(nextRotation);
  }

  function handlePointerUp(event: ReactPointerEvent<HTMLDivElement>): void {
    markUserActivity();
    if (dragRef.current?.pointerId === event.pointerId) {
      dragRef.current = null;
    }
  }

  return (
    <div
      className={cn(
        'relative aspect-[2/1] w-full touch-none overflow-hidden rounded-sm border',
        className,
      )}
      onPointerDown={handlePointerDown}
      onPointerMove={handlePointerMove}
      onPointerUp={handlePointerUp}
      onPointerCancel={handlePointerUp}
      onMouseMove={markUserActivity}
      style={{
        background:
          'radial-gradient(circle at 48% 48%, color-mix(in oklab, var(--attack-map-node-highlight) 14%, transparent) 0%, transparent 34%), radial-gradient(circle at 48% 52%, color-mix(in oklab, var(--attack-map-land) 18%, transparent) 0%, transparent 62%), var(--attack-map-background)',
        borderColor: 'var(--attack-map-border)',
        cursor: dragRef.current ? 'grabbing' : 'grab',
      }}
    >
      <svg
        viewBox={`0 0 ${MAP_WIDTH} ${MAP_HEIGHT}`}
        preserveAspectRatio="xMidYMid meet"
        className="absolute inset-0 h-full w-full"
        xmlns="http://www.w3.org/2000/svg"
      >
        <defs>
          <clipPath id={clipId}>
            <circle cx={CENTER_X} cy={CENTER_Y} r={GLOBE_RADIUS} />
          </clipPath>
          <radialGradient id={`${mapId}-sphere`} cx="34%" cy="24%" r="76%">
            <stop offset="0%" stopColor="var(--attack-map-land)" stopOpacity="0.28" />
            <stop offset="58%" stopColor="var(--attack-map-background)" stopOpacity="0.1" />
            <stop offset="100%" stopColor="var(--foreground)" stopOpacity="0.2" />
          </radialGradient>
          <marker
            id={markerId}
            markerHeight="8"
            markerWidth="8"
            orient="auto"
            refX="7"
            refY="4"
          >
            <path d="M 0 0 L 8 4 L 0 8 z" fill="var(--attack-map-node-highlight)" opacity="0.8" />
          </marker>
        </defs>

        <GlobeSurface clipId={clipId} mapId={mapId} rotation={rotation} />

        <g clipPath={`url(#${clipId})`}>
          {arcs.map((arc) => (
            <AttackArcPath
              key={arc.id}
              arc={arc}
              beamClassName={beamClassName}
              dimmed={hasFocus && !arc.active}
              markerId={markerId}
              motionEnabled={motionEnabled}
              onSelectAttack={onSelectAttack}
            />
          ))}
        </g>

        {nodes.map((node) => {
          const showLabel = showAllLabels || shouldShowTeamLabel(node, hoveredTeamId, focusedTeamSet);
          return (
            <TeamNodeLabel
              key={`${node.id}-label`}
              node={node}
              showLabel={showLabel}
            />
          );
        })}

        {nodes.map((node) => (
          <TeamNodeDot
            key={node.id}
            node={node}
            haloClassName={haloClassName}
            showLabel={showAllLabels || shouldShowTeamLabel(node, hoveredTeamId, focusedTeamSet)}
            onHoverChange={setHoveredTeamId}
            onSelectTeam={onSelectTeam}
          />
        ))}

      </svg>

      <div className="pointer-events-none absolute bottom-3 left-3 flex max-w-[calc(100%-1.5rem)] flex-wrap gap-1.5">
        {services.map((service) => (
          <div
            key={service.name}
            className="flex items-center gap-1.5 rounded-sm border px-2 py-1"
            style={{
              backgroundColor: 'var(--attack-map-legend-background)',
              borderColor: 'var(--attack-map-legend-border)',
            }}
          >
            <span className="h-2 w-2 rounded-full" style={{ backgroundColor: service.color }} />
            <span
              className="text-[10px] font-semibold"
              style={{ color: 'var(--attack-map-legend-foreground)' }}
            >
              {service.name}
            </span>
          </div>
        ))}
      </div>

      <div className="absolute right-3 top-3 flex flex-wrap justify-end gap-1.5">
        {[
          { label: 'Americas', value: { lat: -6, lon: -82 } },
          { label: 'EMEA', value: { lat: -6, lon: 34 } },
          { label: 'APAC', value: { lat: -6, lon: 118 } },
        ].map((preset) => (
          <button
            key={preset.label}
            type="button"
            className="rounded-sm border px-2 py-1 text-[10px] font-semibold"
            onClick={(event) => {
              event.stopPropagation();
              markUserActivity();
              animateRotationTo(preset.value);
            }}
            onPointerDown={(event) => event.stopPropagation()}
            style={{
              backgroundColor: 'var(--attack-map-legend-background)',
              borderColor: 'var(--attack-map-legend-border)',
              color: 'var(--attack-map-node-label)',
            }}
          >
            {preset.label}
          </button>
        ))}
      </div>

      <style
        dangerouslySetInnerHTML={{
          __html: `
        @keyframes ${beamClassName}-keyframes {
          0% { stroke-dashoffset: 760; }
          100% { stroke-dashoffset: 0; }
        }
        @keyframes ${haloClassName}-keyframes {
          0% { opacity: 0.32; transform: scale(0.96); }
          70% { opacity: 0; transform: scale(1.55); }
          100% { opacity: 0; transform: scale(1.55); }
        }
        .${beamClassName} {
          animation: ${beamClassName}-keyframes 2s linear infinite;
        }
        .${haloClassName} {
          transform-box: fill-box;
          transform-origin: center;
          animation: ${haloClassName}-keyframes 1.7s cubic-bezier(0.22, 1, 0.36, 1) infinite;
        }
        @media (prefers-reduced-motion: reduce) {
          .${beamClassName},
          .${haloClassName} {
            animation: none !important;
          }
        }
      `,
        }}
      />
    </div>
  );
}

function GlobeSurface({
  clipId,
  mapId,
  rotation,
}: {
  clipId: string;
  mapId: string;
  rotation: GlobeRotation;
}): ReactElement {
  return (
    <g pointerEvents="none">
      <circle
        cx={CENTER_X}
        cy={CENTER_Y}
        r={GLOBE_RADIUS + 20}
        fill="var(--attack-map-node-highlight-halo)"
        fillOpacity="0.12"
      />
      <circle
        cx={CENTER_X}
        cy={CENTER_Y}
        r={GLOBE_RADIUS}
        fill={`url(#${mapId}-sphere)`}
        stroke="var(--attack-map-border)"
        strokeOpacity="0.92"
        strokeWidth="1.4"
      />
      <g clipPath={`url(#${clipId})`}>
        <ProjectedGraticule rotation={rotation} />
        {COUNTRY_RINGS.map((country) => {
          const fillPath = buildProjectedCountryFillPath(country.points, rotation);
          const outlinePath = buildProjectedLinePath(country.points, rotation);
          if (fillPath === '' && outlinePath === '') {
            return null;
          }
          return (
            <g key={country.id}>
              {fillPath !== '' ? (
                <path
                  d={fillPath}
                  fill="var(--attack-map-land)"
                  fillOpacity="0.18"
                  stroke="none"
                />
              ) : null}
              <path
                d={outlinePath}
                fill="none"
                stroke="var(--attack-map-outline)"
                strokeOpacity="0.46"
                strokeWidth="0.82"
              />
            </g>
          );
        })}
        {LANDMARKS.map((landmark) => {
          const point = projectGlobePoint(landmark, rotation);
          if (!point.visible) {
            return null;
          }
          return (
            <circle
              key={landmark.code}
              cx={point.x}
              cy={point.y}
              r="1.6"
              fill="var(--attack-map-node-highlight)"
              opacity="0.34"
            />
          );
        })}
      </g>
      <circle
        cx={CENTER_X}
        cy={CENTER_Y}
        r={GLOBE_RADIUS}
        fill="none"
        stroke="var(--attack-map-node-highlight)"
        strokeOpacity="0.24"
        strokeWidth="2"
      />
    </g>
  );
}

function ProjectedGraticule({ rotation }: { rotation: GlobeRotation }): ReactElement {
  return (
    <g>
      {[-60, -30, 0, 30, 60].map((lat) => (
        <path
          key={`lat-${lat}`}
          d={buildProjectedLinePath(
            Array.from({ length: 73 }, (_, index) => ({
              lat,
              lon: -180 + index * 5,
            })),
            rotation,
          )}
          fill="none"
          stroke="var(--attack-map-grid)"
          strokeOpacity={lat === 0 ? '0.42' : '0.22'}
        />
      ))}
      {[-150, -120, -90, -60, -30, 0, 30, 60, 90, 120, 150, 180].map((lon) => (
        <path
          key={`lon-${lon}`}
          d={buildProjectedLinePath(
            Array.from({ length: 37 }, (_, index) => ({
              lat: -90 + index * 5,
              lon,
            })),
            rotation,
          )}
          fill="none"
          stroke="var(--attack-map-grid)"
          strokeOpacity="0.18"
        />
      ))}
    </g>
  );
}

function AttackArcPath({
  arc,
  beamClassName,
  dimmed,
  markerId,
  motionEnabled,
  onSelectAttack,
}: {
  arc: AttackArc;
  beamClassName: string;
  dimmed: boolean;
  markerId: string;
  motionEnabled: boolean;
  onSelectAttack?: (attackId: string | null) => void;
}): ReactElement | null {
  if (!arc.visible) {
    return null;
  }
  const emphasized = arc.selected || arc.highlighted || arc.active;
  const opacity = dimmed ? '0.06' : arc.partial ? '0.24' : emphasized ? '0.86' : '0.42';
  const strokeWidth = arc.selected ? '3.6' : emphasized ? '2.35' : '1.45';
  const packetEnabled = motionEnabled && !dimmed && !arc.partial;

  return (
    <g>
      <path
        id={arc.domId}
        d={arc.path}
        fill="none"
        markerEnd={emphasized && !arc.partial ? `url(#${markerId})` : undefined}
        stroke={arc.color}
        strokeLinecap="round"
        strokeDasharray={arc.partial ? '7 9' : undefined}
        strokeOpacity={opacity}
        strokeWidth={strokeWidth}
      />
      {packetEnabled ? (
        <>
          <path
            d={arc.path}
            fill="none"
            stroke={arc.color}
            strokeDasharray={emphasized ? '38 722' : '20 740'}
            strokeLinecap="round"
            strokeOpacity={emphasized ? '0.95' : '0.48'}
            strokeWidth={emphasized ? '3.4' : '2.2'}
            className={beamClassName}
          />
          <circle r={emphasized ? '4.5' : '3.1'} fill={arc.color} opacity={emphasized ? '0.95' : '0.72'}>
            <animateMotion dur={emphasized ? '2.4s' : '3.6s'} begin={arc.delay} repeatCount="indefinite">
              <mpath href={`#${arc.domId}`} />
            </animateMotion>
          </circle>
        </>
      ) : null}
      <path
        d={arc.path}
        fill="none"
        stroke="transparent"
        strokeWidth="14"
        pointerEvents="stroke"
        className="cursor-pointer"
        onClick={() => onSelectAttack?.(arc.selected ? null : arc.id)}
      />
    </g>
  );
}

function TeamNodeLabel({
  node,
  showLabel,
}: {
  node: TeamNode;
  showLabel: boolean;
}): ReactElement | null {
  if (!node.visible || !showLabel) {
    return null;
  }

  return (
    <g pointerEvents="none">
      <line
        x1={node.x}
        x2={node.labelX}
        y1={node.y}
        y2={node.labelY}
        stroke="var(--attack-map-node-label)"
        strokeOpacity={node.selected ? '0.9' : '0.52'}
      />
      <TeamLabelPlate node={node} />
      <text
        x={node.labelX}
        y={node.labelY - 7}
        textAnchor={node.labelAnchor}
        fill={node.selected ? 'var(--foreground)' : 'var(--attack-map-node-label)'}
        stroke="var(--attack-map-node-label-halo)"
        strokeLinejoin="round"
        strokeWidth="2.4"
        paintOrder="stroke"
        fontSize={node.selected ? '12.5' : '11'}
        fontWeight={node.selected ? '700' : '650'}
        className="select-none font-sans"
      >
        {node.name}
      </text>
      <text
        x={node.labelX}
        y={node.labelY + 7}
        textAnchor={node.labelAnchor}
        fill="var(--attack-map-node-label)"
        fontSize="9.5"
        fontWeight="500"
        className="select-none font-sans"
      >
        {node.landmark.name} / {node.outgoing} out
      </text>
    </g>
  );
}

function TeamNodeDot({
  node,
  haloClassName,
  showLabel,
  onHoverChange,
  onSelectTeam,
}: {
  node: TeamNode;
  haloClassName: string;
  showLabel: boolean;
  onHoverChange: (teamId: string | null) => void;
  onSelectTeam?: (teamId: string | null) => void;
}): ReactElement | null {
  if (!node.visible) {
    return null;
  }
  const emphasized = node.selected || node.highlighted || showLabel;
  const totalActivity = node.incoming + node.outgoing;
  const coreRadius = clamp(4.5 + Math.sqrt(totalActivity) * 1.05, 5, 13);
  const ringRadius = coreRadius + 6;

  return (
    <g
      className="cursor-pointer"
      onClick={() => onSelectTeam?.(node.selected ? null : node.id)}
      onMouseEnter={() => onHoverChange(node.id)}
      onMouseLeave={() => onHoverChange(null)}
    >
      <circle
        cx={node.x}
        cy={node.y}
        r={ringRadius}
        fill="var(--attack-map-node-highlight-halo)"
        fillOpacity={emphasized ? '0.3' : '0.08'}
        className={emphasized ? haloClassName : undefined}
      />
      <circle
        cx={node.x}
        cy={node.y}
        r={ringRadius}
        fill="none"
        stroke="var(--attack-map-node-highlight)"
        strokeOpacity={emphasized ? '0.5' : '0.16'}
      />
      <circle
        cx={node.x}
        cy={node.y}
        r={coreRadius}
        fill={emphasized ? 'var(--attack-map-node-highlight)' : 'var(--attack-map-node)'}
        stroke="var(--attack-map-background)"
        strokeWidth="2"
      />
    </g>
  );
}

function TeamLabelPlate({ node }: { node: TeamNode }): ReactElement {
  const width = getTeamLabelWidth(node);
  const box = getLabelBox(node);

  return (
    <rect
      x={box.left}
      y={node.labelY - 23}
      width={width}
      height="40"
      rx="3"
      fill="var(--attack-map-legend-background)"
      stroke="var(--attack-map-legend-border)"
      strokeOpacity={node.selected ? '0.95' : '0.72'}
    />
  );
}

function getTeamLabelWidth(node: TeamNode): number {
  return getRawLabelWidth(
    node.name.length > node.landmark.name.length ? node.name : node.landmark.name,
  );
}

function getRawLabelWidth(value: string): number {
  return clamp(value.length * 7.2 + 38, 104, 220);
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

function buildAttackPath(start: TeamNode, end: TeamNode, attackId: string): string {
  if (start.id === end.id) {
    const radius = 24 + (hashString(attackId) % 18);
    return `M ${start.x} ${start.y} C ${start.x - radius} ${start.y - radius * 2}, ${start.x + radius} ${start.y - radius * 2}, ${start.x} ${start.y}`;
  }

  const midX = (start.x + end.x) / 2;
  const midY = (start.y + end.y) / 2;
  const centerVector = normalize(midX - CENTER_X, midY - CENTER_Y);
  const distance = Math.hypot(end.x - start.x, end.y - start.y);
  const lift = clamp(distance * 0.34, 46, 148) + (hashString(attackId) % 30);
  const controlX = midX + centerVector.x * lift;
  const controlY = midY + centerVector.y * lift - 18;

  return `M ${start.x} ${start.y} Q ${controlX} ${controlY} ${end.x} ${end.y}`;
}

function buildProjectedCountryFillPath(points: GeoPoint[], rotation: GlobeRotation): string {
  const projected = points.map((point) => projectGlobePoint(point, rotation));
  if (projected.length < 3 || !projected.every((point) => point.visible)) {
    return '';
  }
  return `${projected
    .map((point, index) => `${index === 0 ? 'M' : 'L'} ${point.x} ${point.y}`)
    .join(' ')} Z`;
}

function buildProjectedLinePath(points: GeoPoint[], rotation: GlobeRotation): string {
  let path = '';
  let drawing = false;
  points.forEach((point) => {
    const projected = projectGlobePoint(point, rotation);
    if (!projected.visible) {
      drawing = false;
      return;
    }
    path += `${drawing ? ' L' : ' M'} ${projected.x} ${projected.y}`;
    drawing = true;
  });
  return path;
}

function parseCountryRings(collection: GlobeFeatureCollection): CountryRing[] {
  return collection.features.flatMap((feature, featureIndex) => {
    const name = feature.properties?.name ?? `country-${featureIndex}`;
    const polygons = getFeaturePolygons(feature.geometry);
    return polygons.flatMap((polygon, polygonIndex) => {
      const outerRing = polygon[0];
      if (!outerRing || outerRing.length < 3) {
        return [];
      }
      return [
        {
          id: `${featureIndex}-${polygonIndex}`,
          name,
          points: outerRing.map(([lon, lat]) => ({ lat, lon })),
        },
      ];
    });
  });
}

function getFeaturePolygons(
  geometry: GlobeFeatureCollection['features'][number]['geometry'],
): Array<Array<Array<[number, number]>>> {
  if (geometry.type === 'Polygon') {
    return parsePolygonCoordinates(geometry.coordinates);
  }
  return parseMultiPolygonCoordinates(geometry.coordinates);
}

function parsePolygonCoordinates(
  coordinates: unknown,
): Array<Array<Array<[number, number]>>> {
  if (!Array.isArray(coordinates)) {
    return [];
  }
  return [
    coordinates
      .map((ring) => parseLinearRing(ring))
      .filter((ring) => ring.length >= 3),
  ].filter((polygon) => polygon.length > 0);
}

function parseMultiPolygonCoordinates(
  coordinates: unknown,
): Array<Array<Array<[number, number]>>> {
  if (!Array.isArray(coordinates)) {
    return [];
  }
  return coordinates.flatMap((polygon) => parsePolygonCoordinates(polygon));
}

function parseLinearRing(ring: unknown): Array<[number, number]> {
  if (!Array.isArray(ring)) {
    return [];
  }
  return ring.flatMap((coordinate) => {
    if (
      Array.isArray(coordinate) &&
      typeof coordinate[0] === 'number' &&
      typeof coordinate[1] === 'number'
    ) {
      return [[coordinate[0], coordinate[1]] satisfies [number, number]];
    }
    return [];
  });
}

function getLandmarkPlacements(teamNames: string[]): Record<string, Landmark> {
  const orderedTeams = [...teamNames].sort();
  const placements: Record<string, Landmark> = {};

  orderedTeams.forEach((team, index) => {
    placements[team] = LANDMARKS[index % LANDMARKS.length];
  });

  return placements;
}

function projectGlobePoint(point: GeoPoint, rotation: GlobeRotation): ProjectedPoint {
  const lat = toRadians(point.lat);
  const lon = toRadians(normalizeLongitude(point.lon - rotation.lon));
  const tilt = toRadians(rotation.lat);
  const cosLat = Math.cos(lat);
  const x = GLOBE_RADIUS * cosLat * Math.sin(lon);
  const rawY = GLOBE_RADIUS * (Math.sin(lat) * Math.cos(tilt) - cosLat * Math.cos(lon) * Math.sin(tilt));
  const z = Math.sin(lat) * Math.sin(tilt) + cosLat * Math.cos(lon) * Math.cos(tilt);

  return {
    ...point,
    visible: z > -0.04,
    x: CENTER_X + x,
    y: CENTER_Y - rawY,
    z,
  };
}

function getLabelPlacement(point: ProjectedPoint): {
  anchor: 'start' | 'middle' | 'end';
  x: number;
  y: number;
} {
  const vector = normalize(point.x - CENTER_X, point.y - CENTER_Y);
  const anchor = vector.x < -0.22 ? 'end' : vector.x > 0.22 ? 'start' : 'middle';
  const width = getRawLabelWidth('');
  const preferredX = point.x + vector.x * 44;
  const x = clampLabelAnchorX(preferredX, width, anchor);
  const y = clamp(point.y + vector.y * 34, LABEL_SAFE_INSET_Y + LABEL_HEIGHT / 2, MAP_HEIGHT - LABEL_SAFE_INSET_Y - LABEL_HEIGHT / 2);
  return { anchor, x, y };
}

function resolveLabelCollisions(nodes: TeamNode[]): TeamNode[] {
  const adjustedNodes = nodes.map((node) => ({ ...node }));
  const placedNodes: TeamNode[] = [];
  const visibleNodes = adjustedNodes
    .filter((node) => node.visible)
    .sort((left, right) => left.labelY - right.labelY || left.labelX - right.labelX);

  visibleNodes.forEach((node) => {
    let nextY = node.labelY;
    placedNodes.forEach((placedNode) => {
      if (
        labelsOverlap(
          { ...node, labelY: nextY },
          placedNode,
        )
      ) {
        nextY = placedNode.labelY + LABEL_HEIGHT + LABEL_VERTICAL_GAP;
      }
    });
    node.labelY = clamp(nextY, LABEL_SAFE_INSET_Y + LABEL_HEIGHT / 2, MAP_HEIGHT - LABEL_SAFE_INSET_Y - LABEL_HEIGHT / 2);
    node.labelX = clampLabelAnchorX(node.labelX, getTeamLabelWidth(node), node.labelAnchor);
    placedNodes.push(node);
  });

  const overflow = Math.max(
    0,
    ...placedNodes.map((node) => node.labelY + LABEL_HEIGHT / 2 - (MAP_HEIGHT - LABEL_SAFE_INSET_Y)),
  );
  if (overflow > 0) {
    placedNodes.forEach((node) => {
      node.labelY = clamp(node.labelY - overflow, LABEL_SAFE_INSET_Y + LABEL_HEIGHT / 2, MAP_HEIGHT - LABEL_SAFE_INSET_Y - LABEL_HEIGHT / 2);
    });
  }

  return adjustedNodes;
}

function labelsOverlap(left: TeamNode, right: TeamNode): boolean {
  const leftBox = getLabelBox(left);
  const rightBox = getLabelBox(right);
  return !(
    leftBox.right < rightBox.left ||
    leftBox.left > rightBox.right ||
    leftBox.bottom < rightBox.top ||
    leftBox.top > rightBox.bottom
  );
}

function getLabelBox(node: TeamNode): {
  bottom: number;
  left: number;
  right: number;
  top: number;
} {
  const width = getTeamLabelWidth(node);
  const left =
    node.labelAnchor === 'end'
      ? node.labelX + LABEL_PADDING_X - width
      : node.labelAnchor === 'middle'
        ? node.labelX - width / 2
        : node.labelX - LABEL_PADDING_X;

  return {
    bottom: node.labelY + LABEL_HEIGHT / 2,
    left,
    right: left + width,
    top: node.labelY - LABEL_HEIGHT / 2,
  };
}

function clampLabelAnchorX(
  preferredX: number,
  width: number,
  anchor: 'start' | 'middle' | 'end',
): number {
  if (anchor === 'end') {
    return clamp(
      preferredX,
      LABEL_SAFE_INSET_X + width - LABEL_PADDING_X,
      MAP_WIDTH - LABEL_SAFE_INSET_X - LABEL_PADDING_X,
    );
  }
  if (anchor === 'middle') {
    return clamp(
      preferredX,
      LABEL_SAFE_INSET_X + width / 2,
      MAP_WIDTH - LABEL_SAFE_INSET_X - width / 2,
    );
  }
  return clamp(
    preferredX,
    LABEL_SAFE_INSET_X + LABEL_PADDING_X,
    MAP_WIDTH - LABEL_SAFE_INSET_X - width + LABEL_PADDING_X,
  );
}

function normalize(x: number, y: number): { x: number; y: number } {
  const length = Math.sqrt(x * x + y * y);
  if (length === 0) {
    return { x: 0, y: -1 };
  }
  return { x: x / length, y: y / length };
}

function normalizeLongitude(value: number): number {
  return ((((value + 180) % 360) + 360) % 360) - 180;
}

function toRadians(value: number): number {
  return (value * Math.PI) / 180;
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

  return Array.from(new Set([...placementScopeTeams, ...attackTeamNames])).sort();
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

function buildServiceColors(services: string[]): Record<string, string> {
  const colors: Record<string, string> = {};
  services.forEach((service, index) => {
    colors[service] = attackPalette[index % attackPalette.length];
  });
  return colors;
}

function hashString(value: string): number {
  let hash = 5381;
  for (let index = 0; index < value.length; index += 1) {
    hash = ((hash << 5) + hash + value.charCodeAt(index)) >>> 0;
  }
  return hash;
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

const attackPalette = [
  'oklch(0.76 0.13 52)',
  'oklch(0.72 0.1 145)',
  'oklch(0.74 0.09 225)',
  'oklch(0.72 0.12 28)',
  'oklch(0.78 0.1 92)',
  'oklch(0.7 0.1 178)',
  'oklch(0.74 0.1 305)',
  'oklch(0.75 0.1 15)',
];
