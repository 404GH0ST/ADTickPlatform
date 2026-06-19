'use client';

import type {
  KeyboardEvent as ReactKeyboardEvent,
  PointerEvent as ReactPointerEvent,
  ReactElement,
} from 'react';
import { useEffect, useId, useMemo, useRef, useState } from 'react';

import globeJson from '@/data/globe.json';
import { cn } from '@/lib/utils';

export type CyberAttackEvent = {
  id: string;
  attacker: string;
  victim: string;
  service: string;
  tick: number;
  attackerLocation?: GeoPoint;
  victimLocation?: GeoPoint;
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

type ArcPoint = ProjectedPoint & {
  id: string;
};

type AttackArc = {
  attackIds: string[];
  id: string;
  domId: string;
  attacker: string;
  victim: string;
  service: string;
  tick: number;
  count: number;
  color: string;
  endX: number;
  endY: number;
  featured: boolean;
  highlighted: boolean;
  fresh: boolean;
  selected: boolean;
  recency: number;
  active: boolean;
  path: string;
  delay: string;
  partial: boolean;
  startX: number;
  startY: number;
  visible: boolean;
  visibility: number;
};

type AttackRoute = CyberAttackEvent & {
  attackIds: string[];
  count: number;
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
const FEATURED_ATTACK_STEP_MS = 6_400;
const DEFAULT_LABEL_LIMIT = 10;
const DENSE_KEYBOARD_ITEM_LIMIT = 28;
const PACKET_ANIMATION_ROUTE_LIMIT = 80;
const LABEL_HEIGHT = 40;
const LABEL_PADDING_X = 6;
const LABEL_SAFE_INSET_X = 24;
const LABEL_SAFE_INSET_Y = 28;
const LABEL_VERTICAL_GAP = 14;
const FEATURED_ROUTE_SAFE_RADIUS = GLOBE_RADIUS * 0.68;
const FEATURED_ROUTE_HARD_RIM_RADIUS = GLOBE_RADIUS * 0.84;

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
  onFeaturedAttackChange,
  presentation = false,
  teamLocations,
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
  onFeaturedAttackChange?: (attackId: string | null) => void;
  presentation?: boolean;
  teamLocations?: Record<string, GeoPoint>;
}): ReactElement {
  const [hoveredTeamId, setHoveredTeamId] = useState<string | null>(null);
  const [featuredAttackId, setFeaturedAttackId] = useState<string | null>(null);
  const [dragging, setDragging] = useState(false);
  const [rotation, setRotation] = useState<GlobeRotation>({ lat: -6, lon: 34 });
  const mapRootRef = useRef<HTMLDivElement | null>(null);
  const dragRef = useRef<DragState | null>(null);
  const animationFrameRef = useRef<number | null>(null);
  const featuredAttackIndexRef = useRef(0);
  const featuredAttackIntervalRef = useRef<number | null>(null);
  const featuredAttackIdRef = useRef<string | null>(featuredAttackId);
  const featuredAttackPausedRef = useRef(false);
  const previousAttackIdsRef = useRef<Set<string> | null>(null);
  const freshAttackTimeoutRef = useRef<number | null>(null);
  const [freshAttackIds, setFreshAttackIds] = useState<Set<string>>(() => new Set());
  const idleTimeoutRef = useRef<number | null>(null);
  const lastActivityRef = useRef(0);
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
    featuredAttackId !== null ||
    highlightedAttackIDs.length > 0 ||
    focusedTeams.length > 0;
  const featuredQueue = useMemo(() => buildFeaturedAttackQueue(attacks), [attacks]);
  const { arcs, featuredAttack, nodes, services } = useMemo(() => {
    const attackTeamNames = listAttackTeams(attacks);
    const placementTeamNames = listPlacementTeams(attackTeamNames, placementScopeTeams);
    const teamStats = buildTeamStats(attacks, attackTeamNames);
    const landmarksByTeam = getLandmarkPlacements(placementTeamNames, attacks, teamLocations);
    const highlightedSet = new Set(highlightedAttackIDs);
    const serviceNames = listAttackServices(attacks);
    const serviceColors = buildServiceColors(serviceNames);
    const tickRange = getAttackTickRange(attacks);
    const routeBundles = bundleAttackRoutes(attacks);

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

    const arcs = routeBundles.map((attack, index) => {
      const start = nodesByName[attack.attacker];
      const end = nodesByName[attack.victim];
      const highlighted = attack.attackIds.some((id) => highlightedSet.has(id));
      const selected = selectedAttackId !== null && attack.attackIds.includes(selectedAttackId);
      const featured = featuredAttackId !== null && attack.attackIds.includes(featuredAttackId);
      const fresh = attack.attackIds.some((id) => freshAttackIds.has(id));
      const active =
        highlighted ||
        selected ||
        featured ||
        focusedTeamSet.has(attack.attacker) ||
        focusedTeamSet.has(attack.victim) ||
        attack.attacker === selectedTeamId ||
        attack.victim === selectedTeamId;

      if (active) {
        start.highlighted = true;
        end.highlighted = true;
      }
      const arcEndpoints = getArcEndpoints(start, end, rotation);

      return {
        id: attack.id,
        attackIds: attack.attackIds,
        domId: `${mapId}-arc-${index}`,
        attacker: attack.attacker,
        victim: attack.victim,
        service: attack.service,
        tick: attack.tick,
        count: attack.count,
        color: serviceColors[attack.service],
        endX: arcEndpoints.end.x,
        endY: arcEndpoints.end.y,
        highlighted,
        fresh,
        selected,
        featured,
        recency: getAttackRecency(attack.tick, tickRange),
        active,
        path: buildAttackPath(arcEndpoints.start, arcEndpoints.end, attack.id),
        delay: `${(index % 9) * 0.28}s`,
        partial: start.visible !== end.visible,
        startX: arcEndpoints.start.x,
        startY: arcEndpoints.start.y,
        visible: start.visible || end.visible,
        visibility: getArcVisibility(start, end),
      } satisfies AttackArc;
    });

    return {
      arcs: arcs.sort((left, right) => Number(left.active) - Number(right.active)),
      featuredAttack: attacks.find((attack) => attack.id === featuredAttackId) ?? null,
      nodes: resolveLabelCollisions(Object.values(nodesByName)).sort((left, right) => left.z - right.z),
      services: serviceNames.map((service) => ({
        color: serviceColors[service],
        name: service,
      })),
    };
  }, [
    attacks,
    focusedTeamSet,
    featuredAttackId,
    freshAttackIds,
    highlightedAttackIDs,
    mapId,
    placementScopeTeams,
    rotation,
    selectedAttackId,
    selectedTeamId,
    teamLocations,
  ]);
  const showDefaultLabels = !hasFocus && nodes.length > 0 && nodes.length <= DEFAULT_LABEL_LIMIT;
  const denseKeyboardMode = arcs.length + nodes.length > DENSE_KEYBOARD_ITEM_LIMIT;
  const motionEnabled = attacks.length <= 160 && arcs.length <= PACKET_ANIMATION_ROUTE_LIMIT;

  useEffect(() => {
    rotationRef.current = rotation;
  }, [rotation]);

  useEffect(() => {
    featuredAttackIdRef.current = featuredAttackId;
    onFeaturedAttackChange?.(featuredAttackId);
  }, [featuredAttackId, onFeaturedAttackChange]);

  useEffect(() => {
    startAutoRotationFromIdle();
    return clearAutoRotationTimers;
  }, [attacks, featuredQueue, placementScopeTeams]);

  useEffect(() => {
    const nextAttackIds = new Set(attacks.map((attack) => attack.id));
    const previousAttackIds = previousAttackIdsRef.current;
    previousAttackIdsRef.current = nextAttackIds;
    if (previousAttackIds === null) {
      return undefined;
    }

    const freshIds = attacks
      .map((attack) => attack.id)
      .filter((id) => !previousAttackIds.has(id));
    if (freshIds.length === 0) {
      return undefined;
    }

    setFreshAttackIds(new Set(freshIds));
    if (freshAttackTimeoutRef.current !== null) {
      window.clearTimeout(freshAttackTimeoutRef.current);
    }
    freshAttackTimeoutRef.current = window.setTimeout(() => {
      setFreshAttackIds(new Set());
      freshAttackTimeoutRef.current = null;
    }, 4_200);

    return undefined;
  }, [attacks]);

  useEffect(() => {
    const root = mapRootRef.current;
    if (!root) {
      return undefined;
    }

    function handleFeatureAttack(event: Event): void {
      const detail = (event as CustomEvent<{
        animate?: boolean;
        attackId?: string;
        rotation?: GlobeRotation;
      }>).detail;
      if (!detail?.attackId) {
        return;
      }
      const attack = attacks.find((item) => item.id === detail.attackId);
      if (!attack) {
        return;
      }
      clearAutoRotationTimers(false);
      if (detail.rotation) {
        const nextRotation = {
          lat: clamp(detail.rotation.lat, -MAX_ROTATION_LAT, MAX_ROTATION_LAT),
          lon: normalizeLongitude(detail.rotation.lon),
        };
        rotationRef.current = nextRotation;
        setRotation(nextRotation);
      }
      featureAttack(attack, detail.rotation ? false : detail.animate !== false, !detail.rotation);
    }

    root.addEventListener('ad-platform:feature-attack', handleFeatureAttack);
    return () => {
      root.removeEventListener('ad-platform:feature-attack', handleFeatureAttack);
    };
  }, [attacks, placementScopeTeams]);

  useEffect(() => {
    function handlePauseFeaturedAttacks(): void {
      featuredAttackPausedRef.current = true;
      clearAutoRotationTimers(false);
    }

    function handleResumeFeaturedAttacks(): void {
      featuredAttackPausedRef.current = false;
      startAutoRotationFromIdle();
    }

    function handleFeatureNextAttack(): void {
      featuredAttackPausedRef.current = false;
      clearAutoRotationTimers(false);
      featureNextAttack();
      featuredAttackIntervalRef.current = window.setInterval(
        featureNextAttack,
        FEATURED_ATTACK_STEP_MS,
      );
    }

    window.addEventListener('ad-platform:pause-featured-attacks', handlePauseFeaturedAttacks);
    window.addEventListener('ad-platform:resume-featured-attacks', handleResumeFeaturedAttacks);
    window.addEventListener('ad-platform:feature-next-attack', handleFeatureNextAttack);
    return () => {
      window.removeEventListener('ad-platform:pause-featured-attacks', handlePauseFeaturedAttacks);
      window.removeEventListener('ad-platform:resume-featured-attacks', handleResumeFeaturedAttacks);
      window.removeEventListener('ad-platform:feature-next-attack', handleFeatureNextAttack);
    };
  }, [attacks, featuredQueue, placementScopeTeams]);

  function clearAutoRotationTimers(clearFeatured = true): void {
    if (idleTimeoutRef.current !== null) {
      window.clearTimeout(idleTimeoutRef.current);
      idleTimeoutRef.current = null;
    }
    if (featuredAttackIntervalRef.current !== null) {
      window.clearInterval(featuredAttackIntervalRef.current);
      featuredAttackIntervalRef.current = null;
    }
    if (freshAttackTimeoutRef.current !== null) {
      window.clearTimeout(freshAttackTimeoutRef.current);
      freshAttackTimeoutRef.current = null;
    }
    if (clearFeatured && featuredAttackIdRef.current !== null) {
      featuredAttackIdRef.current = null;
      setFeaturedAttackId(null);
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
    const now = window.performance.now();
    if (featuredAttackIdRef.current === null && now - lastActivityRef.current < 250) {
      return;
    }
    lastActivityRef.current = now;
    featuredAttackPausedRef.current = false;
    clearAutoRotationTimers();
    startAutoRotationFromIdle();
  }

  function featureNextAttack(): void {
    if (attacks.length === 0) {
      return;
    }
    const queue = featuredQueue.length > 0 ? featuredQueue : attacks;
    const attack = queue[featuredAttackIndexRef.current % queue.length];
    featuredAttackIndexRef.current += 1;
    featureAttack(attack, true);
  }

  function featureAttack(
    attack: CyberAttackEvent,
    animate: boolean,
    rotate = true,
  ): void {
    setFeaturedAttackId(attack.id);
    if (!rotate) {
      return;
    }

    const attackTeamNames = listAttackTeams(attacks);
    const placementTeamNames = listPlacementTeams(attackTeamNames, placementScopeTeams);
    const landmarksByTeam = getLandmarkPlacements(placementTeamNames, attacks, teamLocations);
    const attacker = landmarksByTeam[attack.attacker];
    const victim = landmarksByTeam[attack.victim];
    const target = getFeaturedAttackRotation(attacker, victim);
    if (target && animate) {
      animateRotationTo(target);
    } else if (target) {
      rotationRef.current = target;
      setRotation(target);
    }
  }

  function animateRotationTo(target: GlobeRotation): void {
    cancelRotationAnimation();
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      rotationRef.current = target;
      setRotation(target);
      return;
    }
    const start = rotationRef.current;
    const startTime = window.performance.now();
    const duration = 2800;
    const deltaLon = normalizeLongitude(target.lon - start.lon);
    const deltaLat = target.lat - start.lat;

    function tick(timestamp: number): void {
      const progress = clamp((timestamp - startTime) / duration, 0, 1);
      const eased = 1 - Math.pow(1 - progress, 4);
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
    clearAutoRotationTimers(false);
    if (attacks.length === 0 || featuredAttackPausedRef.current) {
      return;
    }
    idleTimeoutRef.current = window.setTimeout(() => {
      if (dragRef.current !== null) {
        startAutoRotationFromIdle();
        return;
      }
      featureNextAttack();
      featuredAttackIntervalRef.current = window.setInterval(
        featureNextAttack,
        FEATURED_ATTACK_STEP_MS,
      );
    }, AUTO_ROTATE_IDLE_MS);
  }

  function handlePointerDown(event: ReactPointerEvent<HTMLDivElement>): void {
    if (event.button !== 0 || isInteractivePointerTarget(event.target)) {
      return;
    }
    markUserActivity();
    event.preventDefault();
    event.currentTarget.setPointerCapture(event.pointerId);
    setDragging(true);
    dragRef.current = {
      pointerId: event.pointerId,
      startLat: rotation.lat,
      startLon: rotation.lon,
      x: event.clientX,
      y: event.clientY,
    };
  }

  function handlePointerMove(event: ReactPointerEvent<HTMLDivElement>): void {
    const drag = dragRef.current;
    if (!drag || drag.pointerId !== event.pointerId) {
      return;
    }
    markUserActivity();
    const nextLon = normalizeLongitude(drag.startLon - (event.clientX - drag.x) * 0.42);
    const nextLat = clamp(drag.startLat + (event.clientY - drag.y) * 0.34, -MAX_ROTATION_LAT, MAX_ROTATION_LAT);
    const nextRotation = { lat: nextLat, lon: nextLon };
    rotationRef.current = nextRotation;
    setRotation(nextRotation);
  }

  function handlePointerUp(event: ReactPointerEvent<HTMLDivElement>): void {
    markUserActivity();
    if (dragRef.current?.pointerId === event.pointerId) {
      if (event.currentTarget.hasPointerCapture(event.pointerId)) {
        event.currentTarget.releasePointerCapture(event.pointerId);
      }
      dragRef.current = null;
      setDragging(false);
    }
  }

  function handlePointerLostCapture(event: ReactPointerEvent<HTMLDivElement>): void {
    if (dragRef.current?.pointerId === event.pointerId) {
      dragRef.current = null;
      setDragging(false);
    }
  }

  return (
    <div
      ref={mapRootRef}
      data-testid="cyber-attack-map"
      data-featured-attack-id={featuredAttackId ?? undefined}
      data-keyboard-mode={denseKeyboardMode ? 'jump' : 'direct'}
      data-motion-enabled={motionEnabled ? 'true' : 'false'}
      data-rotation-lat={rotation.lat.toFixed(2)}
      data-rotation-lon={rotation.lon.toFixed(2)}
      data-route-count={arcs.length}
      data-team-count={nodes.length}
      aria-describedby={`${mapId}-instructions`}
      aria-label="Interactive attack globe. Drag to rotate the globe, tab to teams and routes, then press Enter or Space to inspect."
      role="region"
      className={cn(
        'relative aspect-[2/1] w-full touch-none overflow-hidden rounded-sm border',
        className,
      )}
      onPointerDown={handlePointerDown}
      onPointerMove={handlePointerMove}
      onPointerUp={handlePointerUp}
      onPointerCancel={handlePointerUp}
      onLostPointerCapture={handlePointerLostCapture}
      onMouseMove={markUserActivity}
      style={{
        background:
          presentation
            ? 'radial-gradient(circle at 50% 48%, color-mix(in oklab, var(--attack-map-node-highlight) 18%, transparent) 0%, transparent 38%), radial-gradient(circle at 48% 52%, color-mix(in oklab, var(--attack-map-land) 20%, transparent) 0%, transparent 64%), var(--attack-map-background)'
            : 'radial-gradient(circle at 48% 48%, color-mix(in oklab, var(--attack-map-node-highlight) 14%, transparent) 0%, transparent 34%), radial-gradient(circle at 48% 52%, color-mix(in oklab, var(--attack-map-land) 18%, transparent) 0%, transparent 62%), var(--attack-map-background)',
        borderColor: 'var(--attack-map-border)',
        cursor: dragging ? 'grabbing' : 'grab',
      }}
    >
      <p id={`${mapId}-instructions`} className="sr-only">
        Attack map routes and team nodes are keyboard focusable. Press Enter or
        Space on a focused team or route to inspect it.
      </p>
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
              haloClassName={haloClassName}
              markerId={markerId}
              motionEnabled={motionEnabled}
              onSelectAttack={onSelectAttack}
              tabReachable={!denseKeyboardMode}
            />
          ))}
        </g>

        {nodes.map((node) => {
          const featured = isFeaturedTeam(node, featuredAttack);
          const showLabel =
            showDefaultLabels ||
            shouldShowTeamLabel(node, hoveredTeamId, focusedTeamSet) ||
            featured;
          return (
            <TeamNodeLabel
              key={`${node.id}-label`}
              node={node}
              showLabel={showLabel}
            />
          );
        })}

        {nodes.map((node) => {
          const featured = isFeaturedTeam(node, featuredAttack);
          const showLabel =
            showDefaultLabels ||
            shouldShowTeamLabel(node, hoveredTeamId, focusedTeamSet) ||
            featured;
          return (
            <TeamNodeDot
              key={node.id}
              node={node}
              haloClassName={haloClassName}
              showLabel={showLabel}
              onHoverChange={setHoveredTeamId}
              onSelectTeam={onSelectTeam}
              tabReachable={!denseKeyboardMode}
            />
          );
        })}

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

      {presentation ? null : (
        <div className="absolute right-3 top-3 flex flex-wrap justify-end gap-1.5">
          {[
            { label: 'Americas', value: { lat: -6, lon: -82 } },
            { label: 'EMEA', value: { lat: -6, lon: 34 } },
            { label: 'APAC', value: { lat: -6, lon: 118 } },
          ].map((preset) => (
            <button
              key={preset.label}
              type="button"
              className="min-h-9 rounded-sm border px-3 py-1.5 text-[11px] font-semibold sm:min-h-0 sm:px-2 sm:py-1 sm:text-[10px]"
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
      )}

      {presentation || !denseKeyboardMode ? null : (
        <div
          className="absolute left-3 top-3 grid max-w-[min(26rem,calc(100%-1.5rem))] gap-1.5 rounded-sm border p-2 sm:grid-cols-2"
          style={{
            backgroundColor: 'var(--attack-map-legend-background)',
            borderColor: 'var(--attack-map-legend-border)',
            color: 'var(--attack-map-legend-foreground)',
          }}
        >
          <label className="grid gap-1">
            <span className="text-[11px] font-semibold uppercase opacity-80">Jump to team</span>
            <select
              className="h-11 min-w-0 rounded-sm border bg-card px-2 text-sm normal-case text-foreground sm:text-xs"
              value={selectedTeamId ?? ''}
              onChange={(event) => {
                event.stopPropagation();
                const value = event.target.value;
                onSelectTeam?.(value === '' ? null : value);
                if (value !== '') {
                  onSelectAttack?.(null);
                }
              }}
              onPointerDown={(event) => event.stopPropagation()}
            >
              <option value="">Select team</option>
              {nodes
                .slice()
                .sort((left, right) => left.name.localeCompare(right.name))
                .map((node) => (
                  <option key={node.id} value={node.id}>
                    {node.name}
                  </option>
                ))}
            </select>
          </label>
          <label className="grid gap-1">
            <span className="text-[11px] font-semibold uppercase opacity-80">Jump to route</span>
            <select
              className="h-11 min-w-0 rounded-sm border bg-card px-2 text-sm normal-case text-foreground sm:text-xs"
              value={selectedAttackId ?? ''}
              onChange={(event) => {
                event.stopPropagation();
                const value = event.target.value;
                onSelectAttack?.(value === '' ? null : value);
                if (value !== '') {
                  onSelectTeam?.(null);
                }
              }}
              onPointerDown={(event) => event.stopPropagation()}
            >
              <option value="">Select route</option>
              {arcs.map((arc) => (
                <option key={arc.id} value={arc.attackIds[0]}>
                  {arc.attacker} to {arc.victim}, {arc.service}
                </option>
              ))}
            </select>
          </label>
        </div>
      )}

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
        @keyframes ${haloClassName}-fresh {
          0% { opacity: 0; stroke-width: 7; }
          16% { opacity: 0.9; stroke-width: 5; }
          100% { opacity: 0; stroke-width: 1; }
        }
        .${beamClassName} {
          animation: ${beamClassName}-keyframes 4.8s linear infinite;
        }
        .${haloClassName} {
          transform-box: fill-box;
          transform-origin: center;
          animation: ${haloClassName}-keyframes 1.7s cubic-bezier(0.22, 1, 0.36, 1) infinite;
        }
        .${haloClassName}-fresh {
          animation: ${haloClassName}-fresh 1.8s cubic-bezier(0.22, 1, 0.36, 1) 2;
        }
        [data-attack-arc-hit]:focus-visible,
        [data-attack-team-hit]:focus-visible {
          outline: none;
        }
        [data-attack-arc-hit]:focus-visible {
          stroke: var(--attack-map-node-highlight);
          stroke-opacity: 0.62;
          stroke-width: 6.5;
        }
        [data-attack-team-hit]:focus-visible circle:first-of-type {
          fill-opacity: 0.34;
        }
        [data-attack-team-hit]:focus-visible circle:nth-of-type(2) {
          stroke-opacity: 0.95;
          stroke-width: 2.6;
        }
        @media (prefers-reduced-motion: reduce) {
          .${beamClassName},
          .${haloClassName},
          .${haloClassName}-fresh {
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
        r={GLOBE_RADIUS + 14}
        fill="var(--attack-map-node-highlight-halo)"
        fillOpacity="var(--attack-map-outer-glow-opacity)"
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
        strokeOpacity="var(--attack-map-rim-opacity)"
        strokeWidth="var(--attack-map-rim-width)"
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
  haloClassName,
  markerId,
  motionEnabled,
  onSelectAttack,
  tabReachable,
}: {
  arc: AttackArc;
  beamClassName: string;
  dimmed: boolean;
  haloClassName: string;
  markerId: string;
  motionEnabled: boolean;
  onSelectAttack?: (attackId: string | null) => void;
  tabReachable: boolean;
}): ReactElement | null {
  if (!arc.visible) {
    return null;
  }
  const emphasized = arc.selected || arc.highlighted || arc.active;
  const serviceStyle = getServiceRouteStyle(arc.service);
  const depthOpacity = clamp(0.36 + arc.visibility * 0.64, 0.24, 1);
  const bundleWeight = clamp(Math.log2(arc.count + 1) * 0.22, 0, 0.72);
  const trafficWeight = clamp(0.72 + arc.recency * 0.42 + bundleWeight, 0.72, 1.52);
  const opacity = dimmed
    ? 0.035
    : arc.fresh
      ? 0.96
    : arc.partial
      ? arc.featured
        ? 0.92
        : 0.12 * depthOpacity * trafficWeight
      : arc.featured
        ? 0.98
        : emphasized
          ? 0.78 * depthOpacity * trafficWeight
          : 0.2 * depthOpacity * trafficWeight;
  const strokeWidth = arc.partial
    ? arc.featured
      ? '4.4'
      : String(1 + arc.recency * 0.85 + bundleWeight)
    : arc.featured
      ? '5.2'
      : arc.selected
        ? '3.6'
        : emphasized
          ? String(2 + arc.recency * 0.95 + bundleWeight)
          : String(1 + arc.recency * 0.65 + bundleWeight);
  const packetEnabled =
    motionEnabled &&
    !dimmed &&
    !arc.partial &&
    arc.visibility > 0.16 &&
    (arc.featured || arc.fresh || arc.selected || arc.highlighted || arc.recency > 0.86);

  return (
    <g>
      {arc.featured && !dimmed ? (
        <path
          d={arc.path}
          fill="none"
          stroke={arc.color}
          strokeLinecap="round"
          strokeOpacity={arc.partial ? '0.42' : '0.32'}
          strokeWidth={arc.partial ? '12' : '15'}
        />
      ) : null}
      <path
        id={arc.domId}
        data-attack-arc="true"
        data-attack-count={String(arc.count)}
        data-attack-id={arc.id}
        data-featured={arc.featured ? 'true' : 'false'}
        data-fresh={arc.fresh ? 'true' : 'false'}
        data-partial={arc.partial ? 'true' : 'false'}
        data-selected={arc.selected ? 'true' : 'false'}
        d={arc.path}
        fill="none"
        markerEnd={emphasized && !arc.partial ? `url(#${markerId})` : undefined}
        stroke={arc.color}
        strokeLinecap="round"
        strokeDasharray={arc.partial && !arc.featured ? serviceStyle.partialDash : undefined}
        strokeOpacity={opacity}
        strokeWidth={strokeWidth}
      />
      {arc.fresh && !dimmed ? (
        <path
          d={arc.path}
          fill="none"
          stroke="var(--attack-map-node-highlight)"
          strokeLinecap="round"
          strokeOpacity="0.78"
          strokeWidth="7"
          className={`${haloClassName}-fresh`}
        />
      ) : null}
      {packetEnabled ? (
        <>
          <path
            d={arc.path}
            fill="none"
            stroke={arc.featured ? 'var(--attack-map-node-highlight)' : arc.color}
            strokeDasharray={arc.featured ? '54 706' : emphasized ? serviceStyle.activeDash : serviceStyle.idleDash}
            strokeLinecap="round"
            strokeOpacity={arc.featured ? '1' : emphasized ? '0.95' : '0.48'}
            strokeWidth={arc.featured ? '5.1' : emphasized ? serviceStyle.activeWidth : serviceStyle.idleWidth}
            className={beamClassName}
          />
          <circle r={arc.featured ? '6.4' : emphasized ? '4.5' : '3.1'} fill={arc.featured ? 'var(--attack-map-node-highlight)' : arc.color} opacity={emphasized ? '0.95' : '0.62'}>
            <animateMotion dur={arc.featured ? serviceStyle.featuredDuration : emphasized ? serviceStyle.activeDuration : `${serviceStyle.idleDuration - arc.recency * 0.55}s`} begin={arc.delay} repeatCount="indefinite">
              <mpath href={`#${arc.domId}`} />
            </animateMotion>
          </circle>
        </>
      ) : null}
      {arc.featured && !dimmed ? (
        <FeaturedTransmissionEndpoints arc={arc} haloClassName={haloClassName} />
      ) : null}
      <path
        d={arc.path}
        aria-label={`${arc.selected ? 'Clear' : 'Inspect'} attack route from ${arc.attacker} to ${arc.victim} on ${arc.service}, ${arc.count} transmission${arc.count === 1 ? '' : 's'}, tick ${arc.tick}`}
        fill="none"
        role="button"
        stroke="transparent"
        strokeWidth="14"
        pointerEvents="stroke"
        tabIndex={tabReachable ? 0 : -1}
        className="cursor-pointer"
        data-attack-id={arc.id}
        data-attack-arc-hit="true"
        data-selected={arc.selected ? 'true' : 'false'}
        onPointerDown={(event) => event.stopPropagation()}
        onClick={() => onSelectAttack?.(arc.selected ? null : arc.attackIds[0])}
        onKeyDown={(event) => {
          activateSvgButton(event, () => onSelectAttack?.(arc.selected ? null : arc.attackIds[0]));
        }}
      />
    </g>
  );
}

function FeaturedTransmissionEndpoints({
  arc,
  haloClassName,
}: {
  arc: AttackArc;
  haloClassName: string;
}): ReactElement {
  return (
    <g pointerEvents="none">
      <circle
        cx={arc.startX}
        cy={arc.startY}
        r="12"
        fill="none"
        stroke="var(--attack-map-node-highlight)"
        strokeOpacity="0.58"
        strokeWidth="2"
        className={haloClassName}
      />
      <circle
        cx={arc.startX}
        cy={arc.startY}
        r="4.4"
        fill="var(--attack-map-node-highlight)"
        opacity="0.98"
      />
      <circle
        cx={arc.endX}
        cy={arc.endY}
        r="15"
        fill="var(--attack-map-node-highlight-halo)"
        fillOpacity="0.32"
      />
      <circle
        cx={arc.endX}
        cy={arc.endY}
        r="8.5"
        fill="none"
        stroke="var(--attack-map-node-highlight)"
        strokeOpacity="0.9"
        strokeWidth="2.4"
      />
      <path
        d={`M ${arc.endX - 12} ${arc.endY} L ${arc.endX + 12} ${arc.endY} M ${arc.endX} ${arc.endY - 12} L ${arc.endX} ${arc.endY + 12}`}
        fill="none"
        stroke="var(--attack-map-node-highlight)"
        strokeLinecap="round"
        strokeOpacity="0.72"
        strokeWidth="1.5"
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
    <g
      data-attack-team-id={node.id}
      data-attack-team-label="true"
      pointerEvents="none"
    >
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
  tabReachable,
}: {
  node: TeamNode;
  haloClassName: string;
  showLabel: boolean;
  onHoverChange: (teamId: string | null) => void;
  onSelectTeam?: (teamId: string | null) => void;
  tabReachable: boolean;
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
      aria-label={`${node.selected ? 'Clear' : 'Inspect'} ${node.name}, ${node.landmark.name}, ${node.outgoing} outgoing and ${node.incoming} incoming attacks`}
      className="cursor-pointer"
      data-attack-team-id={node.id}
      data-attack-team-hit="true"
      role="button"
      tabIndex={tabReachable ? 0 : -1}
      onPointerDown={(event) => event.stopPropagation()}
      onClick={() => onSelectTeam?.(node.selected ? null : node.id)}
      onFocus={() => onHoverChange(node.id)}
      onBlur={() => onHoverChange(null)}
      onKeyDown={(event) => {
        activateSvgButton(event, () => onSelectTeam?.(node.selected ? null : node.id));
      }}
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
  const titleWidth = node.name.length * 6.5;
  const metaWidth = `${node.landmark.name} / ${node.outgoing} out`.length * 5.2;
  return clamp(Math.max(titleWidth, metaWidth) + 22, 82, 178);
}

function getRawLabelWidth(value: string): number {
  return clamp(value.length * 6.2 + 22, 82, 178);
}

function activateSvgButton(
  event: ReactKeyboardEvent<SVGElement>,
  action: () => void,
): void {
  if (event.key !== 'Enter' && event.key !== ' ') {
    return;
  }
  event.preventDefault();
  event.stopPropagation();
  action();
}

function isInteractivePointerTarget(target: EventTarget | null): boolean {
  return (
    target instanceof Element &&
    target.closest(
      'a,button,input,select,textarea,[data-attack-arc-hit],[data-attack-team-hit]',
    ) !== null
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

function isFeaturedTeam(
  node: TeamNode,
  featuredAttack: CyberAttackEvent | null,
): boolean {
  return (
    featuredAttack !== null &&
    (node.name === featuredAttack.attacker || node.name === featuredAttack.victim)
  );
}

function buildAttackPath(start: ArcPoint, end: ArcPoint, attackId: string): string {
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

function getArcEndpoints(
  start: TeamNode,
  end: TeamNode,
  rotation: GlobeRotation,
): { end: ArcPoint; start: ArcPoint } {
  if (start.visible === end.visible) {
    return { end, start };
  }
  if (start.visible) {
    return {
      end: findHorizonPoint(start, end, rotation),
      start,
    };
  }
  return {
    end,
    start: findHorizonPoint(end, start, rotation),
  };
}

function findHorizonPoint(
  visiblePoint: GeoPoint,
  hiddenPoint: GeoPoint,
  rotation: GlobeRotation,
): ArcPoint {
  let low = 0;
  let high = 1;
  let projected = projectGlobePoint(visiblePoint, rotation);

  for (let index = 0; index < 24; index += 1) {
    const middle = (low + high) / 2;
    const candidate = interpolateGeoPoint(visiblePoint, hiddenPoint, middle);
    const candidateProjection = projectGlobePoint(candidate, rotation);

    if (candidateProjection.z > 0.015) {
      low = middle;
      projected = candidateProjection;
    } else {
      high = middle;
    }
  }

  return {
    ...projected,
    id: `${hiddenPoint.lat}:${hiddenPoint.lon}:horizon`,
    visible: true,
  };
}

function interpolateGeoPoint(start: GeoPoint, end: GeoPoint, progress: number): GeoPoint {
  return {
    lat: start.lat + (end.lat - start.lat) * progress,
    lon: normalizeLongitude(start.lon + normalizeLongitude(end.lon - start.lon) * progress),
  };
}

function getArcVisibility(start: TeamNode, end: TeamNode): number {
  if (!start.visible && !end.visible) {
    return 0;
  }
  if (start.visible !== end.visible) {
    return clamp((Math.max(start.z, end.z) + 0.04) / 1.04, 0, 0.5);
  }
  return clamp((Math.min(start.z, end.z) + 0.04) / 1.04, 0, 1);
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

function getLandmarkPlacements(
  teamNames: string[],
  attacks: CyberAttackEvent[] = [],
  teamLocations: Record<string, GeoPoint> = {},
): Record<string, Landmark> {
  const orderedTeams = [...teamNames].sort();
  const placements: Record<string, Landmark> = {};
  const attackLocations = getAttackProvidedLocations(attacks);

  orderedTeams.forEach((team, index) => {
    const provided = teamLocations[team] ?? attackLocations[team];
    placements[team] = provided
      ? {
          code: `provided-${hashString(team)}`,
          name: 'Configured location',
          lat: provided.lat,
          lon: provided.lon,
        }
      : LANDMARKS[index % LANDMARKS.length];
  });

  return placements;
}

function getAttackProvidedLocations(attacks: CyberAttackEvent[]): Record<string, GeoPoint> {
  const locations: Record<string, GeoPoint> = {};
  attacks.forEach((attack) => {
    if (attack.attackerLocation) {
      locations[attack.attacker] = attack.attackerLocation;
    }
    if (attack.victimLocation) {
      locations[attack.victim] = attack.victimLocation;
    }
  });
  return locations;
}

function getFeaturedAttackRotation(
  attacker?: Landmark,
  victim?: Landmark,
): GlobeRotation | null {
  if (!attacker && !victim) {
    return null;
  }
  if (!attacker || !victim) {
    const target = attacker ?? victim;
    return target
      ? {
          lat: clamp(target.lat * 0.72, -MAX_ROTATION_LAT, MAX_ROTATION_LAT),
          lon: target.lon,
        }
      : null;
  }

  const midpoint = {
    lat: clamp((attacker.lat + victim.lat) / 2, -MAX_ROTATION_LAT, MAX_ROTATION_LAT),
    lon: getMidLongitude(attacker.lon, victim.lon),
  };
  const latCandidates = Array.from(new Set([
    clamp(midpoint.lat, -MAX_ROTATION_LAT, MAX_ROTATION_LAT),
    clamp(midpoint.lat * 0.72, -MAX_ROTATION_LAT, MAX_ROTATION_LAT),
    clamp(midpoint.lat - 12, -MAX_ROTATION_LAT, MAX_ROTATION_LAT),
    clamp(midpoint.lat + 12, -MAX_ROTATION_LAT, MAX_ROTATION_LAT),
    clamp(attacker.lat * 0.58 + victim.lat * 0.42, -MAX_ROTATION_LAT, MAX_ROTATION_LAT),
    clamp(attacker.lat * 0.42 + victim.lat * 0.58, -MAX_ROTATION_LAT, MAX_ROTATION_LAT),
  ].map((lat) => Math.round(lat * 10) / 10)));
  const lonAnchors = [
    midpoint.lon,
    getMidLongitude(attacker.lon, midpoint.lon),
    getMidLongitude(midpoint.lon, victim.lon),
  ];
  const lonCandidates = Array.from(new Set(
    lonAnchors.flatMap((lon) => [-42, -28, -16, 0, 16, 28, 42].map((offset) => (
      Math.round(normalizeLongitude(lon + offset) * 10) / 10
    ))),
  ));
  const candidates = latCandidates.flatMap((lat) => (
    lonCandidates.map((lon) => ({ lat, lon }))
  ));

  return candidates.reduce((best, candidate) => (
    scoreFeaturedRotation(candidate, attacker, victim) > scoreFeaturedRotation(best, attacker, victim)
      ? candidate
      : best
  ));
}

function getMidLongitude(left: number, right: number): number {
  return normalizeLongitude(left + normalizeLongitude(right - left) / 2);
}

function scoreFeaturedRotation(
  rotation: GlobeRotation,
  attacker: Landmark,
  victim: Landmark,
): number {
  const start = projectGlobePoint(attacker, rotation);
  const end = projectGlobePoint(victim, rotation);
  const weakestEndpoint = Math.min(start.z, end.z);
  const combinedDepth = start.z + end.z;
  const centerDistancePenalty = getFeaturedEndpointPenalty(start) + getFeaturedEndpointPenalty(end);
  const horizontalImbalancePenalty = Math.abs((start.x + end.x) / 2 - CENTER_X) / 26;
  const verticalImbalancePenalty = Math.abs((start.y + end.y) / 2 - CENTER_Y) / 42;
  const hiddenPenalty = Number(!start.visible) * 32 + Number(!end.visible) * 32;
  const labelRoomBonus = getLabelRoomScore(start) + getLabelRoomScore(end);

  return (
    weakestEndpoint * 9 +
    combinedDepth * 2.4 +
    labelRoomBonus * 2.2 -
    centerDistancePenalty -
    horizontalImbalancePenalty -
    verticalImbalancePenalty -
    hiddenPenalty
  );
}

function getFeaturedEndpointPenalty(point: ProjectedPoint): number {
  const distanceFromCenter = Math.hypot(point.x - CENTER_X, point.y - CENTER_Y);
  const softPenalty = Math.max(0, (distanceFromCenter - FEATURED_ROUTE_SAFE_RADIUS) / 10);
  const hardPenalty = Math.max(0, (distanceFromCenter - FEATURED_ROUTE_HARD_RIM_RADIUS) / 3);
  return softPenalty + hardPenalty * hardPenalty;
}

function getLabelRoomScore(point: ProjectedPoint): number {
  const horizontalRoom = Math.min(point.x - LABEL_SAFE_INSET_X, MAP_WIDTH - LABEL_SAFE_INSET_X - point.x);
  const verticalRoom = Math.min(point.y - LABEL_SAFE_INSET_Y, MAP_HEIGHT - LABEL_SAFE_INSET_Y - point.y);
  return clamp(Math.min(horizontalRoom / 140, verticalRoom / 90), -1, 1);
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

function getAttackTickRange(attacks: CyberAttackEvent[]): {
  max: number;
  min: number;
} {
  if (attacks.length === 0) {
    return { max: 0, min: 0 };
  }
  return attacks.reduce(
    (range, attack) => ({
      max: Math.max(range.max, attack.tick),
      min: Math.min(range.min, attack.tick),
    }),
    { max: attacks[0]?.tick ?? 0, min: attacks[0]?.tick ?? 0 },
  );
}

function getAttackRecency(
  tick: number,
  range: { max: number; min: number },
): number {
  if (range.max <= range.min) {
    return 1;
  }
  return clamp((tick - range.min) / (range.max - range.min), 0, 1);
}

function bundleAttackRoutes(attacks: CyberAttackEvent[]): AttackRoute[] {
  const routes = new Map<string, AttackRoute>();
  attacks.forEach((attack) => {
    const key = `${attack.attacker}\u0000${attack.victim}\u0000${attack.service}`;
    const existing = routes.get(key);
    if (!existing) {
      routes.set(key, {
        ...attack,
        attackIds: [attack.id],
        count: 1,
      });
      return;
    }

    existing.attackIds.push(attack.id);
    existing.count += 1;
    if (attack.tick >= existing.tick) {
      existing.id = attack.id;
      existing.tick = attack.tick;
    }
  });

  return Array.from(routes.values()).sort((left, right) => (
    left.tick - right.tick ||
    left.attacker.localeCompare(right.attacker) ||
    left.victim.localeCompare(right.victim) ||
    left.service.localeCompare(right.service)
  ));
}

function buildFeaturedAttackQueue(attacks: CyberAttackEvent[]): CyberAttackEvent[] {
  const tickRange = getAttackTickRange(attacks);
  const serviceCounts = new Map<string, number>();
  const teamCounts = new Map<string, number>();
  attacks.forEach((attack) => {
    serviceCounts.set(attack.service, (serviceCounts.get(attack.service) ?? 0) + 1);
    teamCounts.set(attack.attacker, (teamCounts.get(attack.attacker) ?? 0) + 1);
    teamCounts.set(attack.victim, (teamCounts.get(attack.victim) ?? 0) + 1);
  });

  return [...attacks].sort((left, right) => (
    scoreFeaturedAttack(right, tickRange, serviceCounts, teamCounts) -
      scoreFeaturedAttack(left, tickRange, serviceCounts, teamCounts) ||
    right.tick - left.tick ||
    left.id.localeCompare(right.id)
  ));
}

function scoreFeaturedAttack(
  attack: CyberAttackEvent,
  tickRange: { max: number; min: number },
  serviceCounts: Map<string, number>,
  teamCounts: Map<string, number>,
): number {
  const recency = getAttackRecency(attack.tick, tickRange);
  const serviceRarity = 1 / Math.max(1, serviceCounts.get(attack.service) ?? 1);
  const teamActivity =
    (teamCounts.get(attack.attacker) ?? 0) + (teamCounts.get(attack.victim) ?? 0);
  const crossRegion = getAttackRegion(attack.attacker) === getAttackRegion(attack.victim) ? 0 : 1;
  return recency * 7 + crossRegion * 2.4 + serviceRarity * 1.6 + Math.log2(teamActivity + 1);
}

function getAttackRegion(teamName: string): number {
  return hashString(teamName) % 4;
}

function getServiceRouteStyle(service: string): {
  activeDash: string;
  activeDuration: string;
  activeWidth: string;
  featuredDuration: string;
  idleDash: string;
  idleDuration: number;
  idleWidth: string;
  partialDash: string;
} {
  const normalized = service.toLowerCase();
  if (normalized.includes('redis')) {
    return {
      activeDash: '16 22 5 34',
      activeDuration: '2.75s',
      activeWidth: '3.1',
      featuredDuration: '1.65s',
      idleDash: '10 32',
      idleDuration: 5.2,
      idleWidth: '2',
      partialDash: '2 8',
    };
  }
  if (normalized.includes('postgres') || normalized.includes('db')) {
    return {
      activeDash: '42 34',
      activeDuration: '3.7s',
      activeWidth: '3.8',
      featuredDuration: '2.6s',
      idleDash: '28 48',
      idleDuration: 6.6,
      idleWidth: '2.5',
      partialDash: '6 9',
    };
  }
  return {
    activeDash: '28 34',
    activeDuration: '3.15s',
    activeWidth: '3.3',
    featuredDuration: '2.05s',
    idleDash: '18 46',
    idleDuration: 5.8,
    idleWidth: '2.2',
    partialDash: '2 9',
  };
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
