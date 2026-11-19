'use client';

import type {
  KeyboardEvent as ReactKeyboardEvent,
  PointerEvent as ReactPointerEvent,
  ReactElement,
  ReactNode,
  CSSProperties,
} from 'react';
import { memo, useCallback, useEffect, useId, useMemo, useRef, useState } from 'react';
import { ArrowDown, ArrowLeft, ArrowRight, ArrowUp, ChevronDown, Minus, Plus } from 'lucide-react';

import {
  FIRST_BLOOD_FOCUS_MS, FIRST_BLOOD_CHARGE_MS, FIRST_BLOOD_INTRO_MS,
  FIRST_BLOOD_FLIGHT_MS, FIRST_BLOOD_IMPACT_MS, FIRST_BLOOD_RECOVER_MS,
  FIRST_BLOOD_BANNER_HOLD_MS, FIRST_BLOOD_BANNER_EXIT_MS, FIRST_BLOOD_TOTAL_MS,
  FIRST_BLOOD_BANNER_AT_MS,
  type FirstBloodStage,
} from '@/lib/first-blood';
import { useAttackSfx } from '@/components/hooks/use-attack-sfx';
import { useActiveTheme } from '@/components/theme/active-theme-provider';
import { ScrollableRegion } from '@/components/ui/scrollable-region';
import globeJson from '@/data/globe.json';
import { FirstBloodBanner } from './first-blood-banner';
import { cn } from '@/lib/utils';

export type CyberAttackEvent = {
  first_blood?: boolean;
  id: string;
  attacker: string;
  victim: string;
  attacker_institution?: string;
  victim_institution?: string;
  service: string;
  tick: number;
  attackerLocation?: GeoPoint;
  victimLocation?: GeoPoint;
};

// `attack` leads the cinematic; `attacks` is every first blood it announces.
type FirstBloodSequence = { attack: CyberAttackEvent; attacks: CyberAttackEvent[]; stage: FirstBloodStage };
type ClosingFirstBlood = { attacks: CyberAttackEvent[]; leaving: boolean };
type GlobeCamera = { rotation: GlobeRotation; zoom: number; x: number; y: number };

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
    type: 'Feature';
  }>;
  type: 'FeatureCollection';
};

type GlobeVector = [x: number, y: number, z: number];

type CountryRing = {
  points: GlobeVector[];
  /** 0-4, chosen so countries that share a border never share a tint. */
  tint: number;
};

const COUNTRY_TINT_COUNT = 5;

type TeamNode = ProjectedPoint & {
  id: string;
  name: string;
  /** Display-only; empty when the team has no institution set. */
  institution: string;
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
  firstBlood: boolean;
  celebrating: boolean;
  intro: boolean;
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
const COMPACT_MAP_WIDTH = 600;
const CENTER_X = MAP_WIDTH / 2 - 22;
const CENTER_Y = MAP_HEIGHT / 2 + 2;
const GLOBE_RADIUS = 256;
const ZOOM_MIN = 1;
const ZOOM_MAX = 2.6;
const ZOOM_STEP = 0.35;
const MAX_ROTATION_LAT = 62;
const ROTATION_STEP = 12;
const ROTATION_CONTROLS = [
  { label: 'Rotate left', Icon: ArrowLeft, lat: 0, lon: ROTATION_STEP },
  { label: 'Rotate right', Icon: ArrowRight, lat: 0, lon: -ROTATION_STEP },
  { label: 'Rotate up', Icon: ArrowUp, lat: ROTATION_STEP, lon: 0 },
  { label: 'Rotate down', Icon: ArrowDown, lat: -ROTATION_STEP, lon: 0 },
];
const REGION_PRESETS = [
  { label: 'Americas', lon: -82 },
  { label: 'EMEA', lon: 34 },
  { label: 'APAC', lon: 118 },
];
const MAP_CONTROL_CLASS = 'inline-flex min-h-11 min-w-11 items-center justify-center rounded-sm border border-[var(--attack-map-legend-border)] bg-[var(--attack-map-legend-background)] text-[color:var(--attack-map-legend-foreground)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:opacity-40';
const AUTO_ROTATE_IDLE_MS = 10_000;
const FEATURED_ATTACK_STEP_MS = 6_400;
const FRESH_ATTACK_FLY_MS = 1_500;
const FRESH_FRAME_DEBOUNCE_MS = 350;
const DRAG_INTERRUPT_PX = 6;
// Below this depth an endpoint is too close to the rim (or behind it) to share
// one first-blood shot with the other team; the camera travels instead.
const FIRST_BLOOD_SPLIT_DEPTH = 0.12;
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
const LATITUDE_LINES = [-60, -30, 0, 30, 60].map((lat) => ({
  lat,
  points: Array.from({ length: 73 }, (_, index) => toGlobeVector({ lat, lon: -180 + index * 5 })),
}));
const LONGITUDE_LINES = [-150, -120, -90, -60, -30, 0, 30, 60, 90, 120, 150, 180].map((lon) => ({
  lon,
  points: Array.from({ length: 37 }, (_, index) => toGlobeVector({ lat: -90 + index * 5, lon })),
}));

export function CyberAttackMap({
  attacks,
  autoCameraPreference = null,
  highlightedAttackIDs = [],
  liveAttackIDs = [],
  focusedTeams = [],
  placementScopeTeams,
  selectedAttackId = null,
  selectedTeamId = null,
  className,
  expanded = false,
  compactActions,
  onSelectAttack,
  onSelectTeam,
  onFeaturedAttackChange,
  onAutoCameraChange,
  presentation = false,
  teamLocations,
}: {
  attacks: CyberAttackEvent[];
  autoCameraPreference?: boolean | null;
  highlightedAttackIDs?: string[];
  liveAttackIDs?: string[];
  focusedTeams?: string[];
  placementScopeTeams?: string[];
  selectedAttackId?: string | null;
  selectedTeamId?: string | null;
  className?: string;
  expanded?: boolean;
  compactActions?: ReactNode;
  onSelectAttack?: (attackId: string | null) => void;
  onSelectTeam?: (teamId: string | null) => void;
  onFeaturedAttackChange?: (attackId: string | null) => void;
  onAutoCameraChange?: (enabled: boolean) => void;
  presentation?: boolean;
  teamLocations?: Record<string, GeoPoint>;
}): ReactElement {
  const { id: themeId } = useActiveTheme();
  const [compactSurface, setCompactSurface] = useState(false);
  const [canvasAspectRatio, setCanvasAspectRatio] = useState(MAP_WIDTH / MAP_HEIGHT);
  // Match the expanded canvas aspect ratio while keeping space at the globe's rim.
  const frameWidth = expanded
    ? Math.max(COMPACT_MAP_WIDTH, MAP_HEIGHT * canvasAspectRatio)
    : compactSurface ? COMPACT_MAP_WIDTH : MAP_WIDTH;
  const frameHeight = expanded ? Math.max(MAP_HEIGHT, COMPACT_MAP_WIDTH / canvasAspectRatio) : MAP_HEIGHT;
  const [surfaceActive, setSurfaceActive] = useState(true);
  const [reducedMotion, setReducedMotion] = useState(false);
  const autoCameraEnabled = autoCameraPreference ?? !reducedMotion;
  const autoCameraEnabledRef = useRef(autoCameraEnabled);
  autoCameraEnabledRef.current = autoCameraEnabled;
  const [hoveredTeamId, setHoveredTeamId] = useState<string | null>(null);
  const [featuredAttackId, setFeaturedAttackId] = useState<string | null>(null);
  const [dragging, setDragging] = useState(false);
  const [zoom, setZoom] = useState(1);
  const [rotation, setRotation] = useState<GlobeRotation>({ lat: -6, lon: 34 });
  const [cameraCenter, setCameraCenter] = useState({ x: CENTER_X, y: CENTER_Y });
  const [firstBloodSequence, setFirstBloodSequence] = useState<FirstBloodSequence | null>(null);
  const [closingFirstBlood, setClosingFirstBlood] = useState<ClosingFirstBlood | null>(null);
  const [lastFirstBlood, setLastFirstBlood] = useState<CyberAttackEvent[]>([]);
  const firstBloodPresentationActive = firstBloodSequence !== null || closingFirstBlood !== null;
  const sequenceRef = useRef<FirstBloodSequence | null>(null);
  const closingRef = useRef<ClosingFirstBlood | null>(null);
  const sequenceBusyRef = useRef(false);
  const sequenceQueueRef = useRef<CyberAttackEvent[]>([]);
  const sequenceTimersRef = useRef<Set<number>>(new Set());
  const cinematicFrameRef = useRef<number | null>(null);
  const cameraRef = useRef<GlobeCamera>({ rotation, zoom, ...cameraCenter });
  const savedCameraRef = useRef<GlobeCamera | null>(null);
  const sequenceAudioRef = useRef<AbortController | null>(null);
  const playFirstBloodSfx = useAttackSfx({ firstBloodOnly: true });
  const mapRootRef = useRef<HTMLDivElement | null>(null);
  const canvasRef = useRef<HTMLDivElement | null>(null);
  const dragRef = useRef<DragState | null>(null);
  const animationFrameRef = useRef<number | null>(null);
  const featuredAttackIndexRef = useRef(0);
  const featuredAttackIntervalRef = useRef<number | null>(null);
  const featuredAttackIdRef = useRef<string | null>(featuredAttackId);
  const featuredAttackPausedRef = useRef(false);
  const previousAttackIdsRef = useRef<Set<string> | null>(null);
  const freshAttackTimersRef = useRef<Map<string, number>>(new Map());
  const freshFrameTimeoutRef = useRef<number | null>(null);
  const [introAttackIds, setIntroAttackIds] = useState<Set<string>>(() => new Set());
  const [celebratingAttackIds, setCelebratingAttackIds] = useState<Set<string>>(() => new Set());
  const [freshAttackIds, setFreshAttackIds] = useState<Set<string>>(() => new Set());
  const idleTimeoutRef = useRef<number | null>(null);
  const lastActivityRef = useRef(0);
  const rotationRef = useRef<GlobeRotation>(rotation);
  const mapId = useId().replace(/:/g, '');
  const clipId = `${mapId}-globe-clip`;
  const markerId = `${mapId}-arrow`;
  const beamClassName = `globe-beam-${mapId}`;
  const haloClassName = `globe-halo-${mapId}`;
  const flyClassName = `globe-fly-${mapId}`;
  const focusedTeamSet = useMemo(() => new Set(focusedTeams), [focusedTeams]);
  const hasFocus =
    selectedAttackId !== null ||
    selectedTeamId !== null ||
    featuredAttackId !== null ||
    highlightedAttackIDs.length > 0 ||
    focusedTeams.length > 0;
  const featuredQueue = useMemo(() => buildFeaturedAttackQueue(attacks), [attacks]);
  const scene = useMemo(() => {
    const attackTeamNames = listAttackTeams(attacks);
    const placementTeamNames = listPlacementTeams(attackTeamNames, placementScopeTeams);
    const teamStats = buildTeamStats(attacks, attackTeamNames);
    const landmarksByTeam = getLandmarkPlacements(placementTeamNames, attacks, teamLocations);
    const serviceNames = listAttackServices(attacks);
    const serviceColors = buildServiceColors(serviceNames);
    const tickRange = getAttackTickRange(attacks);
    const routeBundles = bundleAttackRoutes(attacks);
    const institutionsByTeam = listTeamInstitutions(attacks);

    return { attackTeamNames, teamStats, landmarksByTeam, serviceNames, serviceColors, tickRange, routeBundles, institutionsByTeam };
  }, [attacks, placementScopeTeams, teamLocations]);
  const { arcs, featuredAttack, nodes, services } = useMemo(() => {
    const { attackTeamNames, teamStats, landmarksByTeam, serviceNames, serviceColors, tickRange, routeBundles, institutionsByTeam } = scene;
    const highlightedSet = new Set(highlightedAttackIDs);

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
        institution: institutionsByTeam.get(name) ?? '',
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
        firstBlood: attack.first_blood === true,
        celebrating: celebratingAttackIds.has(attack.id),
        intro: introAttackIds.has(attack.id) || sequenceQueueRef.current.some((queued) => queued.id === attack.id),
        id: attack.id,
        attackIds: attack.attackIds,
        domId: `${mapId}-arc-${index}`,
        attacker: attack.attacker,
        victim: attack.victim,
        service: attack.service,
        tick: attack.tick,
        count: attack.count,
        color: celebratingAttackIds.has(attack.id) ? 'var(--signal-negative)' : serviceColors[attack.service],
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
    scene,
    attacks,
    focusedTeamSet,
    featuredAttackId,
    freshAttackIds,
    celebratingAttackIds,
    introAttackIds,
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
  const cinematicMotionEnabled = surfaceActive && autoCameraEnabled && !reducedMotion;
  const motionEnabled = cinematicMotionEnabled && attacks.length <= 160 && arcs.length <= PACKET_ANIMATION_ROUTE_LIMIT;
  const sceneRef = useRef(scene);
  sceneRef.current = scene;
  const compactSurfaceRef = useRef(compactSurface);
  compactSurfaceRef.current = compactSurface;
  const frameRef = useRef({ width: frameWidth, height: frameHeight });
  frameRef.current = { width: frameWidth, height: frameHeight };
  const introAttacks = attacks.filter((attack) => introAttackIds.has(attack.id));
  const firstBloodAnnouncement = attacks.find((attack) => celebratingAttackIds.has(attack.id));

  const endDrag = useCallback(() => {
    const drag = dragRef.current;
    if (!drag) return;

    dragRef.current = null;
    const canvas = canvasRef.current;
    if (canvas?.hasPointerCapture(drag.pointerId)) canvas.releasePointerCapture(drag.pointerId);
    setDragging(false);
  }, []);

  useEffect(() => {
    const canvas = canvasRef.current;
    const media = window.matchMedia('(prefers-reduced-motion: reduce)');
    let intersecting = true;
    const sync = () => {
      setSurfaceActive(intersecting && !document.hidden);
      setReducedMotion(media.matches);
    };
    const observer = new IntersectionObserver(([entry]) => {
      intersecting = entry.isIntersecting;
      sync();
    });
    const resizeObserver = new ResizeObserver(([entry]) => {
      const { width, height } = entry.contentRect;
      setCompactSurface(width < 520);
      if (expanded && width > 0 && height > 0) {
        setCanvasAspectRatio(width / height);
      }
    });
    if (canvas) {
      observer.observe(canvas);
      resizeObserver.observe(canvas);
    }
    document.addEventListener('visibilitychange', sync);
    media.addEventListener('change', sync);
    window.addEventListener('blur', endDrag);
    sync();
    return () => {
      observer.disconnect();
      resizeObserver.disconnect();
      document.removeEventListener('visibilitychange', sync);
      media.removeEventListener('change', sync);
      window.removeEventListener('blur', endDrag);
    };
  }, [endDrag, expanded]);

  useEffect(() => {
    rotationRef.current = rotation;
    cameraRef.current = { rotation, zoom, ...cameraCenter };
  }, [rotation, zoom, cameraCenter]);

  useEffect(() => {
    featuredAttackIdRef.current = featuredAttackId;
    onFeaturedAttackChange?.(featuredAttackId);
  }, [featuredAttackId, onFeaturedAttackChange]);

  useEffect(() => {
    if (autoCameraEnabled) {
      clearAutoRotationTimers();
      startAutoRotationFromIdle();
    } else if (featuredAttackIdRef.current && !attacks.some((attack) => attack.id === featuredAttackIdRef.current)) {
      setFeaturedAttackId(null);
    }
    return () => clearAutoRotationTimers(false);
  }, [attacks, featuredQueue, placementScopeTeams, surfaceActive, firstBloodPresentationActive, autoCameraEnabled]);

  useEffect(() => {
    const nextAttackIds = new Set(attacks.map((attack) => attack.id));
    const previousAttackIds = previousAttackIdsRef.current;
    // Keep history across filters so revisiting a capture cannot replay its arrival.
    previousAttackIdsRef.current = new Set([...(previousAttackIds ?? []), ...nextAttackIds]);
    if (previousAttackIds === null) {
      return undefined;
    }

    const freshIds = attacks
      .map((attack) => attack.id)
      .filter((id) => !previousAttackIds.has(id));
    if (freshIds.length === 0) {
      return undefined;
    }

    const timers = freshAttackTimersRef.current;
    setFreshAttackIds((current) => {
      const next = new Set(current);
      freshIds.forEach((id) => next.add(id));
      return next;
    });
    // Expire a whole arrival batch with one timer and one React update.
    const batchKey = freshIds.join('\u0000');
    const freshSet = new Set(freshIds);
    const liveSet = new Set(liveAttackIDs);
    const milestoneIds = attacks
      .filter((attack) => attack.first_blood && freshSet.has(attack.id) && liveSet.has(attack.id))
      .map((attack) => attack.id);
    const milestone = milestoneIds.length > 0;
    if (milestone) {
      // The feed is newest-first; queue in capture order so the earliest
      // first blood of a burst leads its sequence.
      sequenceQueueRef.current.push(...attacks.filter((attack) => milestoneIds.includes(attack.id)).reverse());
      if (!sequenceBusyRef.current) startNextFirstBlood();
    }
    // Milestones live until their own queued sequence ends; normal captures
    // retain the short arrival animation even when a cinematic is running.
    const ordinaryIds = freshIds.filter((id) => !milestoneIds.includes(id));
    if (ordinaryIds.length > 0) {
      timers.set(batchKey, window.setTimeout(() => {
        timers.delete(batchKey);
        setFreshAttackIds((current) => new Set([...current].filter((id) => !ordinaryIds.includes(id))));
      }, FRESH_ATTACK_FLY_MS));
    }

    const latestFreshId = milestoneIds[0] ?? freshIds[0];
    if (freshFrameTimeoutRef.current !== null) {
      window.clearTimeout(freshFrameTimeoutRef.current);
    }
    freshFrameTimeoutRef.current = window.setTimeout(() => {
      freshFrameTimeoutRef.current = null;
      if (sequenceRef.current !== null || milestone || !autoCameraEnabledRef.current || !surfaceActive || document.hidden || featuredAttackIntervalRef.current === null || dragRef.current !== null || featuredAttackPausedRef.current) {
        return;
      }
      const target = attacks.find((attack) => attack.id === latestFreshId);
      if (!target) {
        return;
      }
      featureAttack(target, true);
      if (featuredAttackIntervalRef.current !== null) window.clearInterval(featuredAttackIntervalRef.current);
      featuredAttackIntervalRef.current = window.setInterval(
        featureNextAttack,
        FEATURED_ATTACK_STEP_MS,
      );
    }, FRESH_FRAME_DEBOUNCE_MS);

    return undefined;
  }, [attacks, liveAttackIDs]);

  useEffect(() => {
    const timers = freshAttackTimersRef.current;
    return () => {
      timers.forEach((handle) => window.clearTimeout(handle));
      timers.clear();
      sequenceTimersRef.current.forEach((timer) => window.clearTimeout(timer));
      sequenceAudioRef.current?.abort();
      if (cinematicFrameRef.current !== null) window.cancelAnimationFrame(cinematicFrameRef.current);
      if (freshFrameTimeoutRef.current !== null) {
        window.clearTimeout(freshFrameTimeoutRef.current);
        freshFrameTimeoutRef.current = null;
      }
    };
  }, []);

  useEffect(() => {
    const ids = new Set(attacks.map((attack) => attack.id));
    sequenceQueueRef.current = sequenceQueueRef.current.filter((attack) => ids.has(attack.id));
    const group = sequenceRef.current?.attacks ?? closingRef.current?.attacks ?? [];
    const survivors = group.filter((attack) => ids.has(attack.id));
    if (survivors.length === group.length) return;
    const leadId = sequenceRef.current?.attack.id ?? group[0]?.id;
    if (leadId && ids.has(leadId)) {
      // A withdrawn co-award only drops its row; the lead keeps playing.
      if (sequenceRef.current) {
        sequenceRef.current = { ...sequenceRef.current, attacks: survivors };
        setFirstBloodSequence(sequenceRef.current);
      }
      if (closingRef.current) {
        closingRef.current = { ...closingRef.current, attacks: survivors };
        setClosingFirstBlood(closingRef.current);
      }
      return;
    }
    // A deletion and its replacement first blood may share one SSE frame.
    // Cancel the withdrawn award while preserving valid arrivals in the queue.
    const queued = [...survivors, ...sequenceQueueRef.current];
    interruptFirstBlood();
    sequenceQueueRef.current = queued;
    if (queued.length > 0) {
      setFreshAttackIds((current) => new Set([...current, ...queued.map((attack) => attack.id)]));
      startNextFirstBlood();
    }
  }, [attacks]);

  useEffect(() => {
    if (!autoCameraEnabled || reducedMotion) {
      // Resuming the camera should not restart a first-blood sequence in progress.
      savedCameraRef.current = null;
    }
    if (!cinematicMotionEnabled && cinematicFrameRef.current !== null) {
      window.cancelAnimationFrame(cinematicFrameRef.current);
      cinematicFrameRef.current = null;
    }
  }, [autoCameraEnabled, reducedMotion, cinematicMotionEnabled]);

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
      if (!autoCameraEnabledRef.current) return;
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
  }, [attacks, featuredQueue, placementScopeTeams, surfaceActive]);

  function animateCinematicCamera(target: GlobeCamera, duration: number): void {
    if (!autoCameraEnabledRef.current || window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    if (cinematicFrameRef.current !== null) window.cancelAnimationFrame(cinematicFrameRef.current);
    const start = cameraRef.current;
    const started = performance.now();
    const deltaLon = normalizeLongitude(target.rotation.lon - start.rotation.lon);
    function frame(now: number): void {
      const hidden = document.hidden || mapRootRef.current?.dataset.surfaceActive === 'false';
      const progress = hidden ? 1 : clamp((now - started) / duration, 0, 1);
      const ease = progress < 0.5 ? 4 * progress ** 3 : 1 - (-2 * progress + 2) ** 3 / 2;
      const next = {
        rotation: {
          lat: start.rotation.lat + (target.rotation.lat - start.rotation.lat) * ease,
          lon: normalizeLongitude(start.rotation.lon + deltaLon * ease),
        },
        zoom: start.zoom + (target.zoom - start.zoom) * ease,
        x: start.x + (target.x - start.x) * ease,
        y: start.y + (target.y - start.y) * ease,
      };
      cameraRef.current = next;
      rotationRef.current = next.rotation;
      setRotation(next.rotation);
      setZoom(next.zoom);
      setCameraCenter({ x: next.x, y: next.y });
      cinematicFrameRef.current = progress < 1 ? requestAnimationFrame(frame) : null;
    }
    frame(started);
  }

  function setFirstBloodStage(stage: FirstBloodStage): void {
    if (!sequenceRef.current) return;
    const next = { ...sequenceRef.current, stage };
    sequenceRef.current = next;
    setFirstBloodSequence(next);
  }

  function scheduleSequence(callback: () => void, delay: number): void {
    const timer = window.setTimeout(() => {
      sequenceTimersRef.current.delete(timer);
      callback();
    }, delay);
    sequenceTimersRef.current.add(timer);
  }

  function showClosingFirstBlood(next: ClosingFirstBlood | null): void {
    closingRef.current = next;
    setClosingFirstBlood(next);
    if (next && !next.leaving) setLastFirstBlood(next.attacks);
  }

  function dismissFirstBloodResult(): void {
    if (closingFirstBlood) interruptFirstBlood();
    setLastFirstBlood([]);
    mapRootRef.current?.querySelector<HTMLButtonElement>('[data-fx="map-control"]')?.focus();
  }

  function startNextFirstBlood(): void {
    // Everything pending plays as one sequence: the earliest capture leads the
    // cinematic and the banner lists the rest, so a burst never goes stale.
    const group = sequenceQueueRef.current.splice(0);
    const attack = group[0];
    if (!attack) return;
    sequenceBusyRef.current = true;
    clearAutoRotationTimers(false);
    savedCameraRef.current = autoCameraEnabledRef.current && !window.matchMedia('(prefers-reduced-motion: reduce)').matches
      ? { ...cameraRef.current }
      : null;
    sequenceRef.current = { attack, attacks: group, stage: 'focus' };
    setFirstBloodSequence(sequenceRef.current);
    setIntroAttackIds(new Set([attack.id]));
    setCelebratingAttackIds(new Set());
    if (autoCameraEnabledRef.current) {
      setFeaturedAttackId(attack.id);
      featuredAttackIdRef.current = attack.id;
    }
    const { landmarksByTeam } = sceneRef.current;
    const attacker = landmarksByTeam[attack.attacker];
    const victim = landmarksByTeam[attack.victim];
    const target = getFeaturedAttackRotation(attacker, victim);
    // Each beat's camera. A route that fits one hemisphere is framed in a
    // single shot; a route that wraps the globe (one team behind it, or both
    // pinned to the rim) opens on the attacker and whips around after the
    // strike so the impact always lands on screen.
    let plan: { focus: GlobeCamera; charge: GlobeCamera; flight: GlobeCamera; punch: GlobeCamera; settle: GlobeCamera } | null = null;
    let attackerPan = -0.5;
    let victimPan = 0.5;
    if (target && attacker && victim) {
      const start = projectGlobePoint(attacker, target);
      const end = projectGlobePoint(victim, target);
      const zoomed = (camera: GlobeCamera, scale: number): GlobeCamera => ({
        ...camera, zoom: clamp(camera.zoom * scale, ZOOM_MIN, ZOOM_MAX),
      });
      if (start.visible && end.visible && Math.min(start.z, end.z) >= FIRST_BLOOD_SPLIT_DEPTH) {
        const { width, height } = frameRef.current;
        // Frame both endpoints and their burst/labels. Never zoom one team out
        // of shot, including wide routes and the narrow mobile surface.
        const fitZoom = clamp(Math.min(
          compactSurfaceRef.current ? 1.5 : 1.85,
          width / (Math.abs(start.x - end.x) + 210),
          height / (Math.abs(start.y - end.y) + 340),
        ), ZOOM_MIN, ZOOM_MAX);
        const shot = { rotation: target, zoom: fitZoom, x: (start.x + end.x) / 2, y: (start.y + end.y) / 2 + 38 };
        // A slow push-in while charging, a lean toward the target as the
        // strike travels, and a hard punch on contact that settles back.
        const lean = (amount: number, scale: number): GlobeCamera => zoomed({
          ...shot,
          x: shot.x + (end.x - shot.x) * amount,
          y: shot.y + (end.y - 38 - shot.y) * amount,
        }, scale);
        plan = { focus: shot, charge: lean(0, 1.05), flight: lean(0.22, 1.09), punch: lean(0.26, 1.17), settle: lean(0.2, 1.08) };
        attackerPan = (start.x - CENTER_X) / GLOBE_RADIUS;
        victimPan = (end.x - CENTER_X) / GLOBE_RADIUS;
      } else {
        const centerOn = (point: GeoPoint): GlobeCamera => ({
          rotation: { lat: clamp(point.lat * 0.85, -MAX_ROTATION_LAT, MAX_ROTATION_LAT), lon: normalizeLongitude(point.lon) },
          zoom: compactSurfaceRef.current ? 1.2 : 1.35,
          x: CENTER_X,
          y: CENTER_Y + 38,
        });
        const launch = centerOn(interpolateGreatCircle(attacker, victim, 0.2));
        const arrival = centerOn(interpolateGreatCircle(attacker, victim, 0.8));
        plan = { focus: launch, charge: zoomed(launch, 1.05), flight: arrival, punch: zoomed(arrival, 1.12), settle: zoomed(arrival, 1.04) };
        // The sound travels with the camera: launch left of centre, land right.
        attackerPan = clamp((projectGlobePoint(attacker, launch.rotation).x - CENTER_X) / GLOBE_RADIUS, -0.6, -0.15);
        victimPan = clamp((projectGlobePoint(victim, arrival.rotation).x - CENTER_X) / GLOBE_RADIUS, 0.15, 0.6);
      }
      if (savedCameraRef.current) animateCinematicCamera(plan.focus, FIRST_BLOOD_FOCUS_MS);
    }
    const moveTo = (camera: GlobeCamera | undefined, duration: number) => {
      if (camera && savedCameraRef.current) animateCinematicCamera(camera, duration);
    };
    sequenceAudioRef.current = new AbortController();
    const sequenceAudio = sequenceAudioRef.current;
    const sound = playFirstBloodSfx([attack.id], [attack.id], sequenceAudio.signal, { attackerPan, victimPan });
    scheduleSequence(() => {
      setFirstBloodStage('charge');
      moveTo(plan?.charge, FIRST_BLOOD_CHARGE_MS);
    }, FIRST_BLOOD_FOCUS_MS);
    scheduleSequence(() => {
      setIntroAttackIds(new Set());
      setCelebratingAttackIds(new Set([attack.id]));
      setFirstBloodStage('flight');
      moveTo(plan?.flight, FIRST_BLOOD_FLIGHT_MS);
    }, FIRST_BLOOD_INTRO_MS);
    scheduleSequence(() => {
      setFirstBloodStage('impact');
      moveTo(plan?.punch, 110);
      scheduleSequence(() => moveTo(plan?.settle, 700), 130);
    }, FIRST_BLOOD_INTRO_MS + FIRST_BLOOD_FLIGHT_MS);
    let minimumHoldDone = false;
    let sequenceDone = false;
    let audioDone = !sound;
    let leaving = false;
    const release = () => {
      if (sequenceAudio.signal.aborted || !minimumHoldDone || !sequenceDone || !audioDone || leaving) return;
      leaving = true;
      showClosingFirstBlood({ attacks: closingRef.current?.attacks ?? group, leaving: true });
      scheduleSequence(() => {
        if (sequenceAudio.signal.aborted) return;
        showClosingFirstBlood(null);
        sequenceBusyRef.current = false;
        if (sequenceQueueRef.current.length > 0) startNextFirstBlood();
      }, FIRST_BLOOD_BANNER_EXIT_MS);
    };
    // The banner rises with the camera return, while the announcer speaks.
    scheduleSequence(() => {
      setFirstBloodStage('recover');
      const saved = savedCameraRef.current;
      if (saved) animateCinematicCamera(saved, FIRST_BLOOD_RECOVER_MS);
      showClosingFirstBlood({ attacks: sequenceRef.current?.attacks ?? group, leaving: false });
      scheduleSequence(() => {
        minimumHoldDone = true;
        release();
      }, FIRST_BLOOD_BANNER_HOLD_MS);
    }, FIRST_BLOOD_BANNER_AT_MS);
    scheduleSequence(() => {
      const ids = new Set(group.map((entry) => entry.id));
      sequenceRef.current = null;
      savedCameraRef.current = null;
      setFirstBloodSequence(null);
      setCelebratingAttackIds(new Set());
      setFreshAttackIds((current) => new Set([...current].filter((id) => !ids.has(id))));
      sequenceDone = true;
      release();
    }, FIRST_BLOOD_TOTAL_MS);
    // Keep the result up through the full announcer, including slow decoding.
    if (sound) void sound.finished.then(() => { audioDone = true; release(); });
  }

  function interruptFirstBlood(): void {
    if (!sequenceRef.current && !sequenceBusyRef.current) return;
    sequenceBusyRef.current = false;
    const closingIds = closingRef.current?.attacks.map((attack) => attack.id) ?? [];
    showClosingFirstBlood(null);
    sequenceTimersRef.current.forEach((timer) => window.clearTimeout(timer));
    sequenceTimersRef.current.clear();
    const interruptedIds = new Set([...(sequenceRef.current?.attacks.map((attack) => attack.id) ?? []), ...closingIds, ...sequenceQueueRef.current.map((attack) => attack.id)]);
    setFreshAttackIds((current) => new Set([...current].filter((id) => !interruptedIds.has(id))));
    sequenceQueueRef.current = [];
    sequenceAudioRef.current?.abort();
    if (cinematicFrameRef.current !== null) window.cancelAnimationFrame(cinematicFrameRef.current);
    cinematicFrameRef.current = null;
    sequenceRef.current = null;
    setFirstBloodSequence(null);
    setIntroAttackIds(new Set());
    setCelebratingAttackIds(new Set());
    const saved = savedCameraRef.current;
    savedCameraRef.current = null;
    if (saved) {
      setZoom(saved.zoom);
      setCameraCenter({ x: saved.x, y: saved.y });
    }
  }

  function clearAutoRotationTimers(clearFeatured = true): void {
    if (idleTimeoutRef.current !== null) {
      window.clearTimeout(idleTimeoutRef.current);
      idleTimeoutRef.current = null;
    }
    if (featuredAttackIntervalRef.current !== null) {
      window.clearInterval(featuredAttackIntervalRef.current);
      featuredAttackIntervalRef.current = null;
    }
    if (clearFeatured && !sequenceBusyRef.current && featuredAttackIdRef.current !== null) {
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
    // Passing the cursor over the globe must not cancel its live focal moment.
    if (sequenceBusyRef.current || !autoCameraEnabledRef.current) return;
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
    if (!autoCameraEnabledRef.current || sequenceBusyRef.current || !surfaceActive || document.hidden || attacks.length === 0) {
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
    duration?: number,
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
      animateRotationTo(target, duration);
    } else if (target) {
      rotationRef.current = target;
      setRotation(target);
    }
  }

  function animateRotationTo(target: GlobeRotation, duration = 2800): void {
    cancelRotationAnimation();
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      rotationRef.current = target;
      setRotation(target);
      return;
    }
    const start = rotationRef.current;
    const startTime = window.performance.now();
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
    if (!autoCameraEnabledRef.current || sequenceBusyRef.current || attacks.length === 0 || featuredAttackPausedRef.current || !surfaceActive) {
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
    if (dragRef.current || !event.isPrimary || event.button !== 0 || isInteractivePointerTarget(event.target)) {
      return;
    }
    // A plain click (e.g. the gesture that unlocks audio on a venue screen)
    // must not cancel the cinematic; only an actual drag takes the camera.
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
    if (sequenceBusyRef.current) {
      if (Math.hypot(event.clientX - drag.x, event.clientY - drag.y) < DRAG_INTERRUPT_PX) return;
      interruptFirstBlood();
    }
    markUserActivity();
    const nextLon = normalizeLongitude(drag.startLon - (event.clientX - drag.x) * 0.42);
    const nextLat = clamp(drag.startLat + (event.clientY - drag.y) * 0.34, -MAX_ROTATION_LAT, MAX_ROTATION_LAT);
    const nextRotation = { lat: nextLat, lon: nextLon };
    rotationRef.current = nextRotation;
    setRotation(nextRotation);
  }

  function handlePointerUp(event: ReactPointerEvent<HTMLDivElement>): void {
    if (dragRef.current?.pointerId === event.pointerId) {
      endDrag();
      markUserActivity();
    }
  }

  function rotateBy(lat: number, lon: number): void {
    interruptFirstBlood();
    markUserActivity();
    cancelRotationAnimation();
    const current = rotationRef.current;
    const next = {
      lat: clamp(current.lat + lat, -MAX_ROTATION_LAT, MAX_ROTATION_LAT),
      lon: normalizeLongitude(current.lon + lon),
    };
    rotationRef.current = next;
    setRotation(next);
  }

  function zoomBy(delta: number): void {
    interruptFirstBlood();
    setZoom((current) =>
      Math.round(clamp(current + delta, ZOOM_MIN, ZOOM_MAX) * 100) / 100,
    );
  }

  // Zoom by shrinking the SVG viewBox around the globe centre. Purely visual,
  // so projection/rotation math is untouched; HTML chrome stays unscaled.
  const viewWidth = frameWidth / zoom;
  const viewHeight = frameHeight / zoom;
  const viewBox = `${cameraCenter.x - viewWidth / 2} ${cameraCenter.y - viewHeight / 2} ${viewWidth} ${viewHeight}`;
  const firstBloodResult = useMemo(() => {
    const result = closingFirstBlood?.attacks ?? lastFirstBlood;
    if (result.length === 0) return result;
    const firstBloodIds = new Set(attacks.filter((attack) => attack.first_blood).map(({ id }) => id));
    return result.filter(({ id }) => firstBloodIds.has(id));
  }, [attacks, closingFirstBlood, lastFirstBlood]);

  const globeControls = (
    <div className="grid gap-3">
      <div className={cn('flex flex-wrap items-center gap-y-2', expanded ? 'gap-x-2' : 'gap-x-4')}>
        {presentation ? null : (
          <div role="group" aria-label="Globe regions" className="flex flex-wrap gap-1.5 sm:gap-2">
            {REGION_PRESETS.map(({ label, lon }) => (
              <button
                key={label}
                type="button"
                data-fx="map-control"
                className={cn(MAP_CONTROL_CLASS, 'px-1.5 text-[0.6875rem] font-semibold sm:px-2 sm:text-xs')}
                onClick={() => {
                  interruptFirstBlood();
                  markUserActivity();
                  animateRotationTo({ lat: -6, lon });
                }}
              >
                {label}
              </button>
            ))}
          </div>
        )}
        <div role="group" aria-label="Rotate globe" className="flex flex-wrap gap-2">
          {ROTATION_CONTROLS.map(({ label, Icon, lat, lon }) => (
            <button
              key={label}
              type="button"
              aria-label={label}
              title={label}
              data-fx="map-control"
              className={MAP_CONTROL_CLASS}
              onClick={() => rotateBy(lat, lon)}
            >
              <Icon className="size-4" aria-hidden="true" />
            </button>
          ))}
        </div>
        <div role="group" aria-label="Globe zoom" className="flex flex-wrap gap-2">
          {[
            { Icon: Plus, delta: ZOOM_STEP, label: 'Zoom in', disabled: zoom >= ZOOM_MAX },
            { Icon: Minus, delta: -ZOOM_STEP, label: 'Zoom out', disabled: zoom <= ZOOM_MIN },
          ].map(({ Icon, delta, label, disabled }) => (
            <button
              key={label}
              type="button"
              aria-label={label}
              title={label}
              disabled={disabled}
              data-fx="map-control"
              className={MAP_CONTROL_CLASS}
              onClick={() => zoomBy(delta)}
            >
              <Icon className="size-4" aria-hidden="true" />
            </button>
          ))}
        </div>
        {onAutoCameraChange ? (
          <button
            type="button"
            aria-pressed={autoCameraEnabled}
            className={cn(MAP_CONTROL_CLASS, 'max-w-full flex-wrap gap-x-2 px-2 py-1 text-xs')}
            data-fx="map-control"
            onClick={() => onAutoCameraChange(!autoCameraEnabled)}
          >
            <span>Auto camera</span>
            <span aria-hidden="true">{autoCameraEnabled ? 'On' : 'Paused'}</span>
          </button>
        ) : null}
      </div>
      {presentation || !denseKeyboardMode ? null : (
        <div className={cn(
          'grid w-full max-w-xl gap-2',
          expanded
            ? 'grid-cols-[repeat(auto-fit,minmax(min(100%,9rem),1fr))]'
            : 'grid-cols-[repeat(auto-fit,minmax(min(100%,11rem),1fr))]',
        )}>
          <label className="grid min-w-0 gap-1 text-xs font-semibold">
            Jump to team
            <select
              className="min-h-11 w-full min-w-0 rounded-sm border bg-card px-2 text-sm font-normal text-foreground"
              value={selectedTeamId ?? ''}
              onChange={(event) => {
                const value = event.target.value;
                onSelectTeam?.(value === '' ? null : value);
                if (value !== '') onSelectAttack?.(null);
              }}
            >
              <option value="">Select team</option>
              {nodes.slice().sort((left, right) => left.name.localeCompare(right.name)).map((node) => (
                <option key={node.id} value={node.id}>
                  {node.institution ? `${node.name} (${node.institution})` : node.name}
                </option>
              ))}
            </select>
          </label>
          <label className="grid min-w-0 gap-1 text-xs font-semibold">
            Jump to route
            <select
              className="min-h-11 w-full min-w-0 rounded-sm border bg-card px-2 text-sm font-normal text-foreground"
              value={selectedAttackId ?? ''}
              onChange={(event) => {
                const value = event.target.value;
                onSelectAttack?.(value === '' ? null : value);
                if (value !== '') onSelectTeam?.(null);
              }}
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
    </div>
  );

  const legend = services.length ? (
    <div className="flex min-w-0 flex-wrap gap-1.5">
      {services.map((service) => (
        <div
          key={service.name}
          data-fx="map-legend"
          className="flex min-w-0 max-w-full items-center gap-1.5 rounded-sm border px-2 py-1"
          style={{
            backgroundColor: 'var(--attack-map-legend-background)',
            borderColor: 'var(--attack-map-legend-border)',
          }}
        >
          <span className="size-2 shrink-0 rounded-full" style={{ backgroundColor: service.color }} />
          <span
            className="min-w-0 wrap-anywhere text-[10px] font-semibold"
            style={{ color: 'var(--attack-map-legend-foreground)' }}
          >
            {service.name}
          </span>
        </div>
      ))}
    </div>
  ) : null;
  const legendBelowMap = legend !== null && !compactActions && (expanded || themeId === 'neubrutalism');

  return (
    <div
      ref={mapRootRef}
      data-testid="cyber-attack-map"
      data-featured-attack-id={featuredAttackId ?? undefined}
      data-first-blood-phase={introAttacks.length > 0 ? 'intro' : celebratingAttackIds.size > 0 ? 'attack' : undefined}
      data-first-blood-stage={firstBloodSequence?.stage}
      data-first-blood-closing={closingFirstBlood ? 'true' : undefined}
      data-zoom={zoom.toFixed(3)}
      data-team-placements={Object.entries(scene.landmarksByTeam).map(([team, place]) => `${team}@${place.lat},${place.lon}`).join(';')}
      data-keyboard-mode={denseKeyboardMode ? 'jump' : 'direct'}
      data-surface-active={surfaceActive ? 'true' : 'false'}
      data-motion-enabled={motionEnabled ? 'true' : 'false'}
      data-auto-camera={autoCameraEnabled ? 'on' : 'paused'}
      data-rotation-lat={rotation.lat.toFixed(2)}
      data-rotation-lon={rotation.lon.toFixed(2)}
      data-route-count={arcs.length}
      data-team-count={nodes.length}
      aria-describedby={`${mapId}-instructions`}
      aria-label="Interactive attack globe"
      role="region"
      className={cn(
        'flex min-w-0 flex-col rounded-sm border',
        expanded && 'shrink-0 @[60rem]/expanded-map:min-h-0',
        expanded && (compactActions ? 'relative h-[calc(100%-32px)] min-h-0 @[60rem]/expanded-map:h-auto' : 'min-h-[calc(100%-2.75rem)]'),
      )}
      style={{
        background:
          presentation
            ? 'radial-gradient(circle at 50% 48%, color-mix(in oklab, var(--attack-map-node-highlight) 18%, transparent) 0%, transparent 38%), radial-gradient(circle at 48% 52%, color-mix(in oklab, var(--attack-map-land) 20%, transparent) 0%, transparent 64%), var(--attack-map-background)'
            : 'radial-gradient(circle at 48% 48%, color-mix(in oklab, var(--attack-map-node-highlight) 14%, transparent) 0%, transparent 34%), radial-gradient(circle at 48% 52%, color-mix(in oklab, var(--attack-map-land) 18%, transparent) 0%, transparent 62%), var(--attack-map-background)',
        borderColor: 'var(--attack-map-border)',
      }}
    >
      <p id={`${mapId}-instructions`} className="sr-only">
        Use the rotation buttons or drag the globe to change the view.
        {denseKeyboardMode && !presentation
          ? ' Use the jump menus to inspect a team or route.'
          : ' Press Enter or Space on a focused team or route to inspect it.'}
      </p>
      {compactActions ? (
        <details data-globe-controls className="group pointer-events-none absolute inset-x-2 top-2 z-30 portrait:relative portrait:inset-auto portrait:mx-2 portrait:mt-2 [@media(height>=640px)]:relative [@media(height>=640px)]:inset-auto [@media(height>=640px)]:mx-2 [@media(height>=640px)]:mt-2">
          <summary className={cn(MAP_CONTROL_CLASS, 'pointer-events-auto min-h-[44px] cursor-pointer list-none gap-2 px-2 py-1 text-sm font-semibold [@media(orientation:landscape)_and_(height<640px)]:max-w-1/3 [&::-webkit-details-marker]:hidden')}>
            <span className="min-w-0 wrap-anywhere">Globe controls</span>
            <ChevronDown className="size-4 shrink-0 group-open:rotate-180" aria-hidden />
          </summary>
          <div className="pointer-events-auto absolute inset-x-0 top-full mt-1 grid max-h-[min(50dvh,24rem)] gap-3 overflow-y-auto overscroll-contain rounded-sm border border-border bg-popover p-3 text-popover-foreground shadow-lg">
            {compactActions}
            {globeControls}
            {legend}
          </div>
        </details>
      ) : (
        <div className="rounded-t-[inherit] border-b border-border bg-card p-3">{globeControls}</div>
      )}
      <div data-fx="map-stage" className={cn(
        '@container relative isolate flex min-w-0 flex-1 flex-col',
        !legendBelowMap && 'rounded-b-[inherit]',
        compactActions && 'min-h-0',
      )}>
        {firstBloodResult.length > 0 ? (
          <div
            data-first-blood-layer
            className={cn(
              'pointer-events-none absolute inset-0 z-20 overflow-hidden rounded-b-[inherit]',
              !closingFirstBlood && 'hidden',
            )}
          >
            <FirstBloodBanner
              attacks={firstBloodResult}
              leaving={closingFirstBlood?.leaving ?? false}
              reducedMotion={!closingFirstBlood || !cinematicMotionEnabled}
              retained={!closingFirstBlood}
              onDismiss={dismissFirstBloodResult}
            />
          </div>
        ) : null}
        <div
          ref={canvasRef}
          data-fx="map-canvas"
          className={cn('relative aspect-[2/1] w-full touch-none overflow-hidden rounded-b-[inherit]', className)}
          style={{ cursor: dragging ? 'grabbing' : 'grab' }}
          onPointerDown={handlePointerDown}
          onPointerMove={handlePointerMove}
          onPointerUp={handlePointerUp}
          onPointerCancel={handlePointerUp}
          onLostPointerCapture={handlePointerUp}
          onMouseMove={markUserActivity}
        >
          <svg
            viewBox={viewBox}
            preserveAspectRatio="xMidYMid meet"
            className={cn('absolute inset-0 h-full w-full', firstBloodSequence?.stage === 'impact' && cinematicMotionEnabled ? `${flyClassName}-recoil` : '')}
            xmlns="http://www.w3.org/2000/svg"
          >
            <defs>
              <filter id={`${mapId}-first-blood-glow`} filterUnits="userSpaceOnUse" x="-1000" y="-1000" width="4000" height="4000">
                <feGaussianBlur stdDeviation="5" result="blur" />
                <feMerge>
                  <feMergeNode in="blur" />
                  <feMergeNode in="SourceGraphic" />
                </feMerge>
              </filter>
              <clipPath id={clipId}>
                <circle cx={CENTER_X} cy={CENTER_Y} r={GLOBE_RADIUS} />
              </clipPath>
              <radialGradient id={`${mapId}-sphere`} cx="34%" cy="24%" r="76%">
                <stop offset="0%" stopColor="var(--attack-map-land)" stopOpacity="0.28" />
                <stop offset="58%" stopColor="var(--attack-map-background)" stopOpacity="0.1" />
                <stop offset="100%" stopColor="var(--foreground)" stopOpacity="0.2" />
              </radialGradient>
              {/* Halftone ocean: unused unless a theme shows [data-fx="globe-texture"]. */}
              <pattern id={`${mapId}-dots`} width="7" height="7" patternUnits="userSpaceOnUse">
                <circle cx="3.5" cy="3.5" r="1.05" data-fx="globe-dot" fill="var(--attack-map-grid)" />
              </pattern>
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

            <g opacity={firstBloodSequence && firstBloodSequence.stage !== 'recover' ? 0.48 : closingFirstBlood ? 0.65 : 1} style={{ transition: `opacity ${FIRST_BLOOD_RECOVER_MS}ms ease` }}>
              <GlobeSurface clipId={clipId} mapId={mapId} rotation={rotation} />
            </g>

            <g clipPath={`url(#${clipId})`}>
              {arcs.map((arc) => (
                <AttackArcPath
                  key={arc.id}
                  arc={arc}
                  beamClassName={beamClassName}
                  dimmed={closingFirstBlood ? true : firstBloodSequence ? arc.id !== firstBloodSequence.attack.id : hasFocus && !arc.active}
                  flyClassName={flyClassName}
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
              const focal = firstBloodSequence && (node.id === firstBloodSequence.attack.attacker || node.id === firstBloodSequence.attack.victim);
              const showLabel = closingFirstBlood ? false : firstBloodSequence ? Boolean(focal) : (
                showDefaultLabels || shouldShowTeamLabel(node, hoveredTeamId, focusedTeamSet) || featured
              );
              const insetLabel = compactSurface && focal;
              const labelOnLeft = node.x > cameraCenter.x;
              const labelScale = insetLabel ? 1.5 : 1;
              const labelWidth = getTeamLabelWidth(node) * labelScale;
              const labelNode: TeamNode = insetLabel ? {
                ...node,
                labelX: labelOnLeft
                  ? clamp(node.x - 34, cameraCenter.x - viewWidth / 2 + labelWidth + 10, cameraCenter.x + viewWidth / 2 - 10)
                  : clamp(node.x + 34, cameraCenter.x - viewWidth / 2 + 10, cameraCenter.x + viewWidth / 2 - labelWidth - 10),
                labelY: node.y + 48,
                labelAnchor: labelOnLeft ? 'end' : 'start',
              } : node;
              return (
                <TeamNodeLabel
                  key={`${node.id}-label`}
                  node={labelNode}
                  scale={labelScale}
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
              const unrelated = firstBloodSequence && firstBloodSequence.stage !== 'recover' && node.id !== firstBloodSequence.attack.attacker && node.id !== firstBloodSequence.attack.victim;
              const struck = firstBloodSequence?.attack.victim === node.id && firstBloodSequence.stage === 'impact' && cinematicMotionEnabled;
              return (
                <g key={node.id} opacity={closingFirstBlood ? 0.4 : unrelated ? 0.2 : struck ? 0.12 : 1} style={{ transition: 'opacity 400ms ease' }}>
                  <TeamNodeDot
                    node={node}
                    haloClassName={haloClassName}
                    showLabel={showLabel}
                    onHoverChange={setHoveredTeamId}
                    onSelectTeam={onSelectTeam}
                    tabReachable={!denseKeyboardMode}
                  />
                </g>
              );
            })}

            {firstBloodSequence ? (
              <FirstBloodChoreography
                sequence={firstBloodSequence}
                arc={arcs.find((arc) => arc.id === firstBloodSequence.attack.id)}
                nodes={nodes}
                className={flyClassName}
                motion={cinematicMotionEnabled}
                glowId={`${mapId}-first-blood-glow`}
                clipId={clipId}
              />
            ) : null}
          </svg>

          {firstBloodSequence && cinematicMotionEnabled ? (() => {
            const arc = arcs.find((entry) => entry.id === firstBloodSequence.attack.id);
            const fx = arc ? clamp(((arc.endX - (cameraCenter.x - viewWidth / 2)) / viewWidth) * 100, 0, 100) : 50;
            const fy = arc ? clamp(((arc.endY - (cameraCenter.y - viewHeight / 2)) / viewHeight) * 100, 0, 100) : 50;
            return (
              <div aria-hidden="true" data-first-blood-overlay={firstBloodSequence.stage}
                className="pointer-events-none absolute inset-0 overflow-hidden"
                style={{ '--fx': `${fx}%`, '--fy': `${fy}%` } as CSSProperties}>
                <div data-fx="vignette" className={`${flyClassName}-vignette absolute inset-0`} />
                {firstBloodSequence.stage === 'flight' || firstBloodSequence.stage === 'impact' ? <>
                  <div data-fx="speed" className={`${flyClassName}-speed absolute inset-0`} />
                  <div data-fx="speed" className={`${flyClassName}-speed absolute inset-0`} data-alt="true" />
                </> : null}
                <div data-fx="bar" className={`${flyClassName}-bar absolute inset-x-0 top-0`} data-edge="top" />
                <div data-fx="bar" className={`${flyClassName}-bar absolute inset-x-0 bottom-0`} data-edge="bottom" />
                {firstBloodSequence.stage === 'impact' ? <>
                  <div data-fx="impact-frame" className={`${flyClassName}-impact-frame absolute inset-0`} />
                  <div data-fx="tint" className={`${flyClassName}-tint absolute inset-0`} />
                  <div data-fx="flash" className={`${flyClassName}-flash absolute inset-0`} />
                </> : null}
              </div>
            );
          })() : null}

          {introAttacks.length > 0 || (firstBloodAnnouncement && firstBloodSequence?.stage !== 'recover') ? (
            <FirstBloodNotice
              attacks={introAttacks.length > 0 ? introAttacks : firstBloodAnnouncement ? [firstBloodAnnouncement] : []}
              intro={introAttacks.length > 0}
              stage={firstBloodSequence?.stage ?? 'focus'}
              motionClass={cinematicMotionEnabled ? `${flyClassName}-notice` : undefined}
            />
          ) : null}
          {attacks.length === 0 ? (
            <p data-testid="attack-map-waiting" className="pointer-events-none absolute inset-x-6 bottom-12 text-center text-sm" style={{ color: 'var(--attack-map-node-label)' }}>
              Waiting for the first capture
            </p>
          ) : null}
          {!expanded && !legendBelowMap && legend ? (
            <div className="pointer-events-none absolute inset-x-3 bottom-3">{legend}</div>
          ) : null}

        </div>
      </div>

      {legendBelowMap ? (
        <ScrollableRegion
          axis="vertical"
          label="Map legend"
          className={cn(
            'shrink-0 overflow-y-auto rounded-b-[inherit] border-t border-border bg-card p-2',
            expanded ? 'max-h-[min(5rem,12dvh)]' : 'max-h-[min(9rem,40dvh)]',
          )}
        >
          {legend}
        </ScrollableRegion>
      ) : null}

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
        @keyframes ${flyClassName}-bolt {
          0% { stroke-dashoffset: 0.22; opacity: 0; }
          12% { opacity: 1; }
          85% { opacity: 1; }
          100% { stroke-dashoffset: -1; opacity: 0; }
        }
        @keyframes ${flyClassName}-head {
          0% { stroke-dashoffset: -0.03; opacity: 0; }
          12% { opacity: 1; }
          85% { opacity: 1; }
          100% { stroke-dashoffset: -1.25; opacity: 0; }
        }
        @keyframes ${flyClassName}-impact {
          0%, 70% { opacity: 0; transform: scale(0.2); }
          78% { opacity: 0.95; }
          100% { opacity: 0; transform: scale(2.6); }
        }
        .${beamClassName} {
          animation: ${beamClassName}-keyframes 4.8s linear infinite;
        }
        @keyframes ${flyClassName}-lock {
          0% { opacity: 0; transform: scale(1.8); }
          25%, 85% { opacity: 0.9; transform: scale(1); }
          100% { opacity: 0.3; transform: scale(0.9); }
        }
        .${flyClassName}-lock {
          transform-box: fill-box;
          transform-origin: center;
          animation: ${flyClassName}-lock ${FIRST_BLOOD_INTRO_MS}ms cubic-bezier(0.16, 1, 0.3, 1) 1 both;
        }
        @keyframes ${flyClassName}-notice-in {
          from { opacity: 0; transform: translateY(6px); }
          to { opacity: 1; transform: translateY(0); }
        }
        .${flyClassName}-notice { animation: ${flyClassName}-notice-in 360ms cubic-bezier(0.16, 1, 0.3, 1) both; }
        @keyframes ${flyClassName}-settle {
          from { opacity: 1; }
          to { opacity: 0; }
        }
        .${flyClassName}-notice[data-first-blood-notice-stage="impact"] {
          animation: ${flyClassName}-settle 300ms ease ${FIRST_BLOOD_IMPACT_MS - 300}ms both;
        }
        .${flyClassName}-settle { animation: ${flyClassName}-settle ${FIRST_BLOOD_RECOVER_MS}ms ease both; }
        @keyframes ${flyClassName}-charge-ring {
          from { stroke-dashoffset: 1; transform: rotate(-90deg) scale(1.35); opacity: 0.2; }
          to { stroke-dashoffset: 0; transform: rotate(270deg) scale(0.65); opacity: 1; }
        }
        .${flyClassName}-charge-ring {
          animation: ${flyClassName}-charge-ring ${FIRST_BLOOD_CHARGE_MS}ms cubic-bezier(0.7, 0, 0.84, 0) both;
        }
        @keyframes ${flyClassName}-charge-core {
          from { transform: scale(0.45); stroke-width: 1; opacity: 0.2; }
          to { transform: scale(1.2); stroke-width: 9; opacity: 0.85; }
        }
        .${flyClassName}-charge-core { animation: ${flyClassName}-charge-core ${FIRST_BLOOD_CHARGE_MS}ms ease-in both; }
        @keyframes ${flyClassName}-charge-in {
          0% { transform: translateY(-12px); opacity: 0; }
          25% { opacity: 0.85; }
          100% { transform: translateY(42px); opacity: 0; }
        }
        .${flyClassName}-charge-in { animation: ${flyClassName}-charge-in 520ms ease-in infinite both; }
        @keyframes ${flyClassName}-launch {
          from { transform: scale(0.6); opacity: 1; }
          to { transform: scale(2.3); opacity: 0; }
        }
        .${flyClassName}-launch { animation: ${flyClassName}-launch ${FIRST_BLOOD_FLIGHT_MS}ms cubic-bezier(0.16, 1, 0.3, 1) both; }
        @keyframes ${flyClassName}-strike {
          0% { stroke-dashoffset: 0.23; opacity: 0; }
          8%, 96% { opacity: 1; }
          100% { stroke-dashoffset: -1; opacity: 0; }
        }
        .${flyClassName}-strike { animation: ${flyClassName}-strike ${FIRST_BLOOD_FLIGHT_MS}ms cubic-bezier(0.7, 0, 0.84, 0) both; }
        @keyframes ${flyClassName}-shock {
          0% { transform: scale(0.4); opacity: 1; }
          100% { transform: scale(4); opacity: 0; }
        }
        .${flyClassName}-shock { animation: ${flyClassName}-shock ${FIRST_BLOOD_IMPACT_MS - 150}ms cubic-bezier(0.16, 1, 0.3, 1) both; }
        @keyframes ${flyClassName}-shard {
          from { transform: translateY(0) rotate(0); opacity: 1; }
          65% { opacity: 0.8; }
          to { transform: translateY(calc(-1 * var(--shard-distance))) rotate(var(--shard-turn)); opacity: 0; }
        }
        .${flyClassName}-shard { animation: ${flyClassName}-shard ${FIRST_BLOOD_IMPACT_MS - 50}ms cubic-bezier(0.16, 1, 0.3, 1) both; }
        @keyframes ${flyClassName}-reform {
          from { transform: scale(2); opacity: 0; stroke-dasharray: 2 12; }
          to { transform: scale(1); opacity: 0.65; stroke-dasharray: 14 0; }
        }
        .${flyClassName}-reform { animation: ${flyClassName}-reform ${FIRST_BLOOD_RECOVER_MS}ms cubic-bezier(0.16, 1, 0.3, 1) both; }
        @keyframes ${flyClassName}-recoil {
          0%, 100% { transform: translate(0, 0) scale(1); }
          6% { transform: translate(-7px, 3px) scale(1.012); }
          14% { transform: translate(6px, -4px) scale(1.008); }
          24% { transform: translate(-4px, 2px); }
          36% { transform: translate(3px, -1.5px); }
          50% { transform: translate(-1.5px, 1px); }
          70% { transform: translate(0.6px, 0); }
        }
        .${flyClassName}-recoil { animation: ${flyClassName}-recoil 600ms ease-out both; }
        @keyframes ${flyClassName}-spin { to { transform: rotate(360deg); } }
        .${flyClassName}-spin { animation: ${flyClassName}-spin 2.4s linear infinite; }
        .${flyClassName}-spin-reverse { animation: ${flyClassName}-spin 1.6s linear infinite reverse; }
        @keyframes ${flyClassName}-rumble { ${FIRST_BLOOD_RUMBLE_KEYFRAMES} }
        .${flyClassName}-rumble { animation: ${flyClassName}-rumble ${FIRST_BLOOD_CHARGE_MS}ms linear both; }
        @keyframes ${flyClassName}-flicker {
          0% { opacity: 1; } 30% { opacity: 0.4; } 45% { opacity: 1; } 58% { opacity: 0.3; } 68% { opacity: 1; }
          76% { opacity: 0.35; } 83% { opacity: 1; } 88% { opacity: 0.4; } 92% { opacity: 1; } 95% { opacity: 0.5; } 97%, 100% { opacity: 1; }
        }
        .${flyClassName}-flicker { animation: ${flyClassName}-flicker ${FIRST_BLOOD_CHARGE_MS}ms linear both; }
        @keyframes ${flyClassName}-burst {
          from { transform: translateY(0) scaleY(0.4); opacity: 1; }
          to { transform: translateY(-26px) scaleY(1.4); opacity: 0; }
        }
        .${flyClassName}-burst { animation: ${flyClassName}-burst 480ms cubic-bezier(0.16, 1, 0.3, 1) both; }
        @keyframes ${flyClassName}-target { 0%, 100% { opacity: 0.3; } 50% { opacity: 1; } }
        .${flyClassName}-target { animation: ${flyClassName}-target 420ms ease-in-out infinite; }
        @keyframes ${flyClassName}-core-flash {
          from { transform: scale(0.3); opacity: 1; }
          to { transform: scale(3.4); opacity: 0; }
        }
        .${flyClassName}-core-flash { animation: ${flyClassName}-core-flash 320ms ease-out both; }
        @keyframes ${flyClassName}-crack {
          0% { stroke-dashoffset: 1; opacity: 1; }
          30% { stroke-dashoffset: 0; opacity: 1; }
          100% { stroke-dashoffset: 0; opacity: 0; }
        }
        .${flyClassName}-crack { animation: ${flyClassName}-crack ${FIRST_BLOOD_IMPACT_MS - 100}ms cubic-bezier(0.16, 1, 0.3, 1) both; }
        @keyframes ${flyClassName}-spark {
          from { transform: translateY(0); opacity: 1; }
          70% { opacity: 0.9; }
          to { transform: translateY(calc(-1 * var(--spark-distance))); opacity: 0; }
        }
        .${flyClassName}-spark { animation: ${flyClassName}-spark 700ms cubic-bezier(0.1, 0.9, 0.3, 1) both; }
        @keyframes ${flyClassName}-glitch {
          0% { transform: translate(-3px, 0); } 15% { transform: translate(3px, 1px); } 30% { transform: translate(-2px, -1px); }
          45% { transform: translate(1px, 0); } 60%, 100% { transform: translate(0, 0); }
        }
        .${flyClassName}-glitch { animation: ${flyClassName}-glitch 360ms steps(1, end) both; }
        @keyframes ${flyClassName}-ember {
          from { transform: translateY(0); opacity: 0; }
          20% { opacity: 0.9; }
          to { transform: translateY(calc(-1 * var(--ember-rise))); opacity: 0; }
        }
        .${flyClassName}-ember { animation: ${flyClassName}-ember 800ms ease-out both; }
        @keyframes ${flyClassName}-overlay-in { from { opacity: 0; } to { opacity: 0.7; } }
        @keyframes ${flyClassName}-vignette-charge {
          0% { opacity: 0.7; } 40% { opacity: 0.8; } 55% { opacity: 0.66; } 70% { opacity: 0.9; }
          80% { opacity: 0.74; } 90% { opacity: 1; } 95% { opacity: 0.85; } 100% { opacity: 1; }
        }
        @keyframes ${flyClassName}-overlay-out { from { opacity: 1; } to { opacity: 0; } }
        .${flyClassName}-vignette {
          background: radial-gradient(ellipse at 50% 50%, transparent 38%,
            color-mix(in srgb, var(--signal-negative) 16%, transparent) 72%,
            color-mix(in srgb, #000 58%, transparent) 100%);
          animation: ${flyClassName}-overlay-in ${FIRST_BLOOD_FOCUS_MS}ms ease-out both;
        }
        [data-first-blood-overlay="charge"] .${flyClassName}-vignette { animation: ${flyClassName}-vignette-charge ${FIRST_BLOOD_CHARGE_MS}ms ease-in both; }
        [data-first-blood-overlay="flight"] .${flyClassName}-vignette,
        [data-first-blood-overlay="impact"] .${flyClassName}-vignette { animation: none; opacity: 1; }
        [data-first-blood-overlay="recover"] .${flyClassName}-vignette { animation: ${flyClassName}-overlay-out ${FIRST_BLOOD_RECOVER_MS}ms ease both; }
        @keyframes ${flyClassName}-bar-in-top { from { transform: translateY(-100%); } to { transform: translateY(0); } }
        @keyframes ${flyClassName}-bar-in-bottom { from { transform: translateY(100%); } to { transform: translateY(0); } }
        @keyframes ${flyClassName}-bar-out-top { from { transform: translateY(0); } to { transform: translateY(-100%); } }
        @keyframes ${flyClassName}-bar-out-bottom { from { transform: translateY(0); } to { transform: translateY(100%); } }
        .${flyClassName}-bar {
          height: 7%;
          background: color-mix(in srgb, #000 84%, var(--signal-negative));
          animation: ${flyClassName}-bar-in-top ${FIRST_BLOOD_FOCUS_MS}ms cubic-bezier(0.16, 1, 0.3, 1) both;
        }
        .${flyClassName}-bar[data-edge="bottom"] { animation-name: ${flyClassName}-bar-in-bottom; }
        [data-first-blood-overlay="recover"] .${flyClassName}-bar { animation: ${flyClassName}-bar-out-top ${FIRST_BLOOD_RECOVER_MS}ms ease-in both; }
        [data-first-blood-overlay="recover"] .${flyClassName}-bar[data-edge="bottom"] { animation-name: ${flyClassName}-bar-out-bottom; }
        @keyframes ${flyClassName}-flash { 0% { opacity: 0; } 8% { opacity: 1; } 100% { opacity: 0; } }
        .${flyClassName}-flash {
          background: radial-gradient(circle at var(--fx) var(--fy),
            color-mix(in srgb, #fff 85%, transparent) 0%,
            color-mix(in srgb, var(--signal-negative) 55%, transparent) 18%,
            transparent 60%);
          mix-blend-mode: screen;
          animation: ${flyClassName}-flash 420ms ease-out both;
        }
        @keyframes ${flyClassName}-crackle { 0%, 100% { opacity: 0; } 18% { opacity: 1; } 34% { opacity: 0.15; } 46% { opacity: 0.9; } 60% { opacity: 0; } }
        .${flyClassName}-crackle { animation: ${flyClassName}-crackle 260ms steps(1, end) infinite; }
        @keyframes ${flyClassName}-crackle-build { from { opacity: 0.15; transform: scale(0.8); } to { opacity: 1; transform: scale(1.15); } }
        .${flyClassName}-crackle-build { animation: ${flyClassName}-crackle-build ${FIRST_BLOOD_CHARGE_MS}ms ease-in both; }
        @keyframes ${flyClassName}-scorch { from { stroke-opacity: 0.9; stroke-width: 7; } to { stroke-opacity: 0; stroke-width: 1; } }
        .${flyClassName}-scorch { animation: ${flyClassName}-scorch ${FIRST_BLOOD_IMPACT_MS}ms ease-out both; }
        @keyframes ${flyClassName}-global-shock {
          from { transform: scale(0.4); opacity: 0.95; stroke-width: 4; }
          to { transform: scale(9); opacity: 0; stroke-width: 0.4; }
        }
        .${flyClassName}-global-shock {
          transform-box: fill-box;
          transform-origin: center;
          animation: ${flyClassName}-global-shock ${FIRST_BLOOD_IMPACT_MS - 200}ms cubic-bezier(0.12, 0.8, 0.3, 1) both;
        }
        .${flyClassName}-speed {
          background: repeating-conic-gradient(from 0deg at var(--fx) var(--fy),
            transparent 0deg 2.6deg,
            color-mix(in srgb, var(--signal-negative) 34%, transparent) 2.6deg 3deg);
          -webkit-mask-image: radial-gradient(circle at var(--fx) var(--fy), transparent 0 26%, #000 75%);
          mask-image: radial-gradient(circle at var(--fx) var(--fy), transparent 0 26%, #000 75%);
          animation: ${flyClassName}-speed-a 140ms steps(1, end) infinite;
        }
        .${flyClassName}-speed[data-alt] {
          background: repeating-conic-gradient(from 1.4deg at var(--fx) var(--fy),
            transparent 0deg 3.4deg,
            color-mix(in srgb, var(--attack-map-node-label) 22%, transparent) 3.4deg 3.7deg);
          animation-name: ${flyClassName}-speed-b;
        }
        @keyframes ${flyClassName}-speed-a { 0% { opacity: 0.9; } 50% { opacity: 0.25; } }
        @keyframes ${flyClassName}-speed-b { 0% { opacity: 0.2; } 50% { opacity: 0.8; } }
        [data-first-blood-overlay="impact"] .${flyClassName}-speed {
          animation: ${flyClassName}-overlay-out 360ms ease-out both;
        }
        @keyframes ${flyClassName}-impact-frame { 0%, 55% { opacity: 0.82; } 100% { opacity: 0; } }
        .${flyClassName}-impact-frame {
          background: var(--attack-map-node-label);
          mix-blend-mode: difference;
          animation: ${flyClassName}-impact-frame 150ms steps(2, end) both;
        }
        @keyframes ${flyClassName}-tint { from { opacity: 0.3; } to { opacity: 0; } }
        .${flyClassName}-tint { background: var(--signal-negative); animation: ${flyClassName}-tint 420ms ease-out 70ms both; }
        [data-surface-active="false"] * { animation-play-state: paused !important; }
        .${flyClassName}-bolt {
          animation: ${flyClassName}-bolt 1.05s cubic-bezier(0.22, 1, 0.36, 1) 1 both;
        }
        .${flyClassName}-head {
          animation: ${flyClassName}-head 1.05s cubic-bezier(0.22, 1, 0.36, 1) 1 both;
        }
        .${flyClassName}-impact {
          transform-box: fill-box;
          transform-origin: center;
          animation: ${flyClassName}-impact 1.4s cubic-bezier(0.22, 1, 0.36, 1) 1 both;
        }
        .${haloClassName} {
          transform-box: fill-box;
          transform-origin: center;
          animation: ${haloClassName}-keyframes 1.7s cubic-bezier(0.22, 1, 0.36, 1) infinite;
        }
        .${haloClassName}-fresh {
          animation: ${haloClassName}-fresh 1.8s cubic-bezier(0.22, 1, 0.36, 1) 2;
        }
        [data-attack-arc-hit]:focus,
        [data-attack-team-hit]:focus,
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
          .${flyClassName}-lock,
          .${beamClassName},
          .${haloClassName},
          .${haloClassName}-fresh,
          .${flyClassName}-bolt,
          .${flyClassName}-head,
          .${flyClassName}-impact {
            animation: none !important;
          }
        }
      `,
        }}
      />
    </div>
  );
}

const FIRST_BLOOD_RETICLE = 'M -10 -24 H -24 V -10 M 10 -24 H 24 V -10 M -24 10 V 24 H -10 M 24 10 V 24 H 10';
// Deterministic impact debris, computed once rather than per render.
const FIRST_BLOOD_CRACKS = Array.from({ length: 8 }, (_, index) => {
  const angle = ((index * 45 + (index % 2 ? 14 : -9)) * Math.PI) / 180;
  const reach = 54 + (index % 3) * 12;
  return [0, 0.34, 0.66, 1].map((t, step) => {
    const bend = step === 0 ? 0 : (step % 2 ? 0.2 : -0.14) * (index % 2 ? 1 : -1);
    const r = t * reach;
    return `${step ? 'L' : 'M'} ${(Math.sin(angle + bend) * r).toFixed(1)} ${(-Math.cos(angle + bend) * r).toFixed(1)}`;
  }).join(' ');
});
const FIRST_BLOOD_SPARKS = Array.from({ length: 20 }, (_, index) => ({
  angle: index * 18 + (index % 3) * 7,
  distance: 62 + ((index * 37) % 50),
  delay: (index * 23) % 120,
  size: index % 4 === 0 ? 2.4 : 1.4,
}));
const FIRST_BLOOD_EMBERS = Array.from({ length: 7 }, (_, index) => ({
  x: (index - 3) * 9 + (index % 2 ? 3 : -2),
  delay: index * 90,
  rise: 34 + (index % 3) * 14,
}));
// Lightning that crackles around the charging core, each bolt on its own beat.
const FIRST_BLOOD_BOLTS = Array.from({ length: 7 }, (_, index) => {
  const angle = (index * 360) / 7 + (index % 2 ? 11 : -7);
  const points = [14, 24, 33, 42, 52].map((radius, step) => {
    const jitter = step === 0 ? 0 : (step % 2 ? 9 : -8) * (index % 3 === 1 ? -1 : 1);
    const a = ((angle + jitter) * Math.PI) / 180;
    return `${step ? 'L' : 'M'} ${(Math.sin(a) * radius).toFixed(1)} ${(-Math.cos(a) * radius).toFixed(1)}`;
  });
  return { d: points.join(' '), delay: (index * 53) % 230 };
});
// A shake whose amplitude builds through the charge, ending at the launch.
const FIRST_BLOOD_RUMBLE_KEYFRAMES = Array.from({ length: 21 }, (_, index) => {
  const amplitude = (index / 20) ** 2 * 3.2;
  const x = (index % 2 ? 1 : -1) * amplitude;
  const y = ((index % 3) - 1) * amplitude * 0.7;
  return `${index * 5}% { transform: translate(${x.toFixed(2)}px, ${y.toFixed(2)}px); }`;
}).join(' ');

function FirstBloodChoreography({ sequence, arc, nodes, className, motion, glowId, clipId }: {
  sequence: FirstBloodSequence;
  arc?: AttackArc;
  nodes: TeamNode[];
  className: string;
  motion: boolean;
  glowId: string;
  clipId: string;
}): ReactElement | null {
  if (!arc) return null;
  const attacker = nodes.find((node) => node.id === sequence.attack.attacker);
  const victim = nodes.find((node) => node.id === sequence.attack.victim);
  const { stage } = sequence;
  const intro = stage === 'focus' || stage === 'charge';
  const hit = stage === 'impact';
  const glow = motion ? `url(#${glowId})` : undefined;
  return (
    <g pointerEvents="none" fill="none" stroke="var(--signal-negative)"
      data-first-blood-effect={intro ? undefined : 'true'}
      className={motion && stage === 'recover' ? `${className}-settle` : undefined}>
      {attacker?.visible ? (
        <g data-first-blood-endpoint="attacker" transform={`translate(${arc.startX} ${arc.startY})`}>
          <title>First blood submitted by {sequence.attack.attacker}</title>
          {intro ? (
            <>
              <path d={FIRST_BLOOD_RETICLE} strokeWidth="2" className={motion ? `${className}-lock` : undefined} />
              {motion ? <circle r="34" strokeWidth="1.2" strokeDasharray="3 7" className={`${className}-spin`} /> : null}
              {stage === 'charge' ? (
                <g data-first-blood-charge="true" className={motion ? `${className}-rumble` : undefined}>
                  <circle r="44" strokeWidth="2.5" pathLength="1" strokeDasharray="1"
                    className={motion ? `${className}-charge-ring` : undefined} />
                  {motion ? <>
                    <circle r="60" strokeWidth="1.5" strokeDasharray="14 10" strokeOpacity="0.7" className={`${className}-spin`} />
                    <circle r="72" strokeWidth="1" strokeDasharray="2 9" strokeOpacity="0.5" className={`${className}-spin-reverse`} />
                  </> : null}
                  <circle r="17" strokeWidth="5" strokeOpacity="0.35" filter={glow}
                    className={motion ? `${className}-charge-core` : undefined} />
                  <circle r="7" fill="var(--signal-negative)" stroke="none" filter={glow}
                    className={motion ? `${className}-flicker` : undefined} />
                  {motion ? (
                    <g className={`${className}-crackle-build`} strokeLinejoin="round" strokeLinecap="round">
                      {FIRST_BLOOD_BOLTS.map((bolt, index) => (
                        <path key={index} d={bolt.d} strokeWidth={index % 2 ? 1.2 : 1.8}
                          stroke={index % 3 ? 'var(--signal-negative)' : 'var(--attack-map-node-label)'}
                          className={`${className}-crackle`} style={{ animationDelay: `${bolt.delay}ms` }} />
                      ))}
                    </g>
                  ) : null}
                  {Array.from({ length: motion ? 16 : 8 }, (_, index) => (
                    <g key={index} transform={`rotate(${index * (motion ? 22.5 : 45)})`}>
                      <path d={index % 2 && motion ? 'M 0 -92 V -66' : 'M 0 -78 V -55'} strokeWidth={index % 2 && motion ? 1.4 : 2}
                        className={motion ? `${className}-charge-in` : undefined}
                        style={motion ? { animationDelay: `${(index * 37) % 300}ms` } : undefined} />
                    </g>
                  ))}
                </g>
              ) : null}
            </>
          ) : (
            <>
              <circle r="28" strokeWidth="3" className={motion && stage === 'flight' ? `${className}-launch` : undefined} opacity={stage === 'flight' ? 1 : 0.35} />
              {motion && stage === 'flight' ? Array.from({ length: 8 }, (_, index) => (
                <g key={index} transform={`rotate(${index * 45 + 22.5})`}>
                  <path d="M 0 -30 V -46" strokeWidth="2.5" strokeLinecap="round" className={`${className}-burst`} />
                </g>
              )) : null}
            </>
          )}
        </g>
      ) : null}
      {stage === 'flight' ? (
        <g data-first-blood-strike="true">
          <path d={arc.path} strokeWidth="18" strokeOpacity="0.13" />
          <path d={arc.path} strokeWidth="3" strokeOpacity="0.45" />
          {motion ? <path d={arc.path} pathLength="1" strokeDasharray="0.22 1" strokeLinecap="round" strokeWidth="16"
            strokeOpacity="0.5" filter={glow} className={`${className}-strike`} /> : null}
          <path d={arc.path} pathLength="1" strokeDasharray="0.22 1" strokeLinecap="round" strokeWidth="8"
            className={motion ? `${className}-strike` : undefined} />
          {motion ? [1, 2, 3].map((echo) => (
            <path key={echo} d={arc.path} pathLength="1" strokeDasharray="0.04 1" strokeLinecap="round"
              strokeWidth={6 - echo * 1.4} strokeOpacity={0.7 - echo * 0.17}
              className={`${className}-strike`} style={{ animationDelay: `${echo * 45}ms` }} />
          )) : null}
          {motion ? <path d={arc.path} pathLength="1" strokeDasharray="0.008 1" strokeLinecap="round" strokeWidth="4"
            stroke="var(--attack-map-node-label)" className={`${className}-strike`} /> : null}
        </g>
      ) : null}
      {motion && hit ? (
        <g data-first-blood-aftermath="true">
          <path d={arc.path} strokeWidth="5" strokeLinecap="round" filter={glow} className={`${className}-scorch`} />
          <g clipPath={`url(#${clipId})`}>
            <circle cx={arc.endX} cy={arc.endY} r="40" strokeWidth="2.5" className={`${className}-global-shock`} />
            <circle cx={arc.endX} cy={arc.endY} r="40" strokeWidth="1" strokeOpacity="0.6"
              className={`${className}-global-shock`} style={{ animationDelay: '180ms' }} />
          </g>
        </g>
      ) : null}
      {victim?.visible ? (
        <g data-first-blood-endpoint="victim" transform={`translate(${arc.endX} ${arc.endY})`}>
          <title>First blood captured from {sequence.attack.victim}</title>
          <path d={FIRST_BLOOD_RETICLE}
            strokeWidth="1.6" opacity={hit ? 0.2 : 0.8} />
          {motion && (intro || stage === 'flight') ? (
            <g data-first-blood-target="true">
              <circle r="36" strokeWidth="1" strokeDasharray="4 6" className={`${className}-spin-reverse`} />
              <path d="M -44 0 H -32 M 32 0 H 44 M 0 -44 V -32 M 0 32 V 44" strokeWidth="1.5"
                className={`${className}-target`} />
            </g>
          ) : null}
          {hit ? (
            <g data-first-blood-impact="true">
              <circle r="24" strokeWidth="3" className={motion ? `${className}-shock` : undefined} />
              {motion ? <>
                <circle r="12" fill="var(--attack-map-node-label)" stroke="none" className={`${className}-core-flash`} />
                <circle r="30" strokeWidth="10" strokeOpacity="0.5" filter={glow} className={`${className}-shock`} />
                <g transform="translate(-3 1)" opacity="0.7">
                  <circle r="26" strokeWidth="2" className={`${className}-shock`} style={{ animationDelay: '40ms' }} />
                </g>
                <g transform="translate(3 -1)" opacity="0.7">
                  <circle r="26" strokeWidth="2" stroke="var(--attack-map-node-label)" className={`${className}-shock`} style={{ animationDelay: '40ms' }} />
                </g>
                <circle r="20" strokeWidth="1.5" className={`${className}-shock`} style={{ animationDelay: '130ms' }} />
                <circle r="22" strokeWidth="1.2" stroke="var(--attack-map-node-label)" className={`${className}-shock`} style={{ animationDelay: '260ms' }} />
                {FIRST_BLOOD_CRACKS.map((crack, index) => (
                  <path key={`crack-${index}`} d={crack} pathLength="1" strokeDasharray="1" strokeWidth="1.6"
                    strokeLinejoin="round" className={`${className}-crack`} />
                ))}
                {Array.from({ length: 12 }, (_, index) => {
                  const distance = 48 + (index % 3) * 13;
                  return (
                    <g key={index} transform={`rotate(${index * 30})`}>
                      <path d={index % 3 === 0 ? 'M -9 -24 H -24 V -9' : 'M -4 -24 H 4'} strokeWidth={index % 3 === 0 ? 2 : 1.5}
                        data-first-blood-shard="true" className={`${className}-shard`}
                        style={{ '--shard-distance': `${distance}px`, '--shard-turn': `${index % 2 ? -38 : 28}deg` } as CSSProperties} />
                    </g>
                  );
                })}
                {FIRST_BLOOD_SPARKS.map((spark, index) => (
                  <g key={`spark-${index}`} transform={`rotate(${spark.angle})`}>
                    <circle cy="-10" r={spark.size} fill={index % 3 ? 'var(--signal-negative)' : 'var(--attack-map-node-label)'} stroke="none"
                      className={`${className}-spark`}
                      style={{ '--spark-distance': `${spark.distance}px`, animationDelay: `${spark.delay}ms` } as CSSProperties} />
                  </g>
                ))}
              </> : null}
              <path d="M -7 -7 L 7 7 M 7 -7 L -7 7" strokeWidth="3" className={motion ? `${className}-glitch` : undefined} />
            </g>
          ) : null}
          {stage === 'recover' ? <>
            <circle r="17" strokeWidth="2" className={motion ? `${className}-reform` : undefined} />
            {motion ? FIRST_BLOOD_EMBERS.map((ember, index) => (
              <circle key={index} cx={ember.x} cy="-6" r="1.6" fill="var(--signal-negative)" stroke="none"
                className={`${className}-ember`}
                style={{ '--ember-rise': `${ember.rise}px`, animationDelay: `${ember.delay}ms` } as CSSProperties} />
            )) : null}
          </> : null}
        </g>
      ) : null}
    </g>
  );
}

function FirstBloodNotice({ attacks, intro, stage, motionClass }: {
  attacks: CyberAttackEvent[];
  intro: boolean;
  stage: FirstBloodStage;
  motionClass?: string;
}): ReactElement {
  return (
    <div role="status" data-testid={intro ? 'first-blood-intro' : 'first-blood-announcement'}
      className={cn('pointer-events-none absolute inset-x-4 bottom-14 z-10 sm:inset-x-6', motionClass)}
      data-first-blood-notice-stage={stage}
      style={{ color: 'var(--attack-map-node-label)' }}>
      {/* A scrim rather than a solid box keeps the globe and impact visible above the text. */}
      <div data-first-blood-notice-panel="true" className="space-y-3 px-1 pb-3 pt-8"
        style={{ background: 'linear-gradient(to top, color-mix(in srgb, var(--attack-map-background) 94%, transparent) 0%, color-mix(in srgb, var(--attack-map-background) 78%, transparent) 55%, transparent 100%)' }}>
        {attacks.map((attack) => (
          <div key={attack.id}>
            <h3 className="grid grid-cols-[minmax(0,1fr)_1.75rem_minmax(0,1fr)] items-center gap-2 text-[clamp(1.05rem,2vw,1.65rem)] leading-tight sm:gap-4">
              <span className="min-w-0 break-words font-semibold">
                {attack.attacker}
                <FirstBloodNoticeInstitution value={attack.attacker_institution} />
              </span>
              <svg aria-hidden="true" viewBox="0 0 28 12" className="h-3 w-7" fill="none" stroke="currentColor" strokeWidth="1.5">
                <path d="M 0 6 H 26 M 20 1 L 26 6 L 20 11" />
              </svg>
              <span className="min-w-0 break-words text-right font-normal">
                {attack.victim}
                <FirstBloodNoticeInstitution value={attack.victim_institution} />
              </span>
            </h3>
            <div className="mt-2 flex items-start justify-between gap-3 text-xs">
              <p data-first-blood-notice-tag="true" className="min-w-0 break-words" style={{ color: 'var(--signal-negative)' }}>First blood · {attack.service}</p>
              <p className="shrink-0 text-[11px]">Tick #{attack.tick}</p>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function FirstBloodNoticeInstitution({ value }: { value?: string }): ReactElement | null {
  if (!value) return null;
  return (
    <span
      data-first-blood-notice-institution="true"
      className="mt-0.5 block text-[11px] font-medium uppercase leading-snug tracking-[0.1em] sm:text-xs"
      style={{ color: 'var(--attack-map-legend-foreground)' }}
    >
      {value}
    </span>
  );
}

const GlobeSurface = memo(function GlobeSurface({
  clipId,
  mapId,
  rotation,
}: {
  clipId: string;
  mapId: string;
  rotation: GlobeRotation;
}): ReactElement {
  const projection = createGlobeLineProjection(rotation);
  const landByTint = Array.from({ length: COUNTRY_TINT_COUNT }, () => '');
  let outline = '';
  // A ring's projected line already outlines its fill when the whole ring is
  // visible. Batch by paint so rotating the surface updates only a few paths.
  for (const country of COUNTRY_RINGS) {
    const { path, fullyVisible } = projectGlobeLine(country.points, projection);
    outline += path;
    if (fullyVisible) landByTint[country.tint] += `${path} Z`;
  }

  return (
    <g pointerEvents="none">
      <circle
        cx={CENTER_X}
        cy={CENTER_Y}
        r={GLOBE_RADIUS + 14}
        data-fx="globe-halo"
        fill="var(--attack-map-node-highlight-halo)"
        fillOpacity="var(--attack-map-outer-glow-opacity)"
      />
      <circle
        cx={CENTER_X}
        cy={CENTER_Y}
        r={GLOBE_RADIUS}
        data-fx="globe-sphere"
        fill={`url(#${mapId}-sphere)`}
        stroke="var(--attack-map-border)"
        strokeOpacity="0.92"
        strokeWidth="1.4"
      />
      <circle
        cx={CENTER_X}
        cy={CENTER_Y}
        r={GLOBE_RADIUS}
        data-fx="globe-texture"
        display="none"
        fill={`url(#${mapId}-dots)`}
      />
      <g clipPath={`url(#${clipId})`}>
        <ProjectedGraticule projection={projection} />
        {landByTint.map((path, tint) => path && (
          <path
            key={tint}
            d={path}
            data-fx="globe-land"
            data-tint={tint}
            fill="var(--attack-map-land)"
            fillOpacity="0.18"
            stroke="none"
          />
        ))}
        <path
          d={outline}
          data-fx="globe-borders"
          fill="none"
          stroke="var(--attack-map-outline)"
          strokeOpacity="0.46"
          strokeWidth="0.82"
        />
        {LANDMARKS.map((landmark) => {
          const point = projectGlobePoint(landmark, rotation);
          if (!point.visible) {
            return null;
          }
          return (
            <circle
              key={landmark.code}
              data-fx="globe-landmark"
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
        data-fx="globe-rim"
        fill="none"
        stroke="var(--attack-map-node-highlight)"
        strokeOpacity="var(--attack-map-rim-opacity)"
        strokeWidth="var(--attack-map-rim-width)"
      />
    </g>
  );
});

function ProjectedGraticule({ projection }: { projection: GlobeLineProjection }): ReactElement {
  return (
    <g>
      {LATITUDE_LINES.map(({ lat, points }) => (
        <path
          key={`lat-${lat}`}
          d={projectGlobeLine(points, projection).path}
          fill="none"
          data-fx="globe-graticule"
          data-equator={lat === 0 ? 'true' : undefined}
          stroke="var(--attack-map-grid)"
          strokeOpacity={lat === 0 ? '0.42' : '0.22'}
        />
      ))}
      {LONGITUDE_LINES.map(({ lon, points }) => (
        <path
          key={`lon-${lon}`}
          d={projectGlobeLine(points, projection).path}
          fill="none"
          data-fx="globe-graticule"
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
  flyClassName,
  haloClassName,
  markerId,
  motionEnabled,
  onSelectAttack,
  tabReachable,
}: {
  arc: AttackArc;
  beamClassName: string;
  dimmed: boolean;
  flyClassName: string;
  haloClassName: string;
  markerId: string;
  motionEnabled: boolean;
  onSelectAttack?: (attackId: string | null) => void;
  tabReachable: boolean;
}): ReactElement | null {
  if (!arc.visible || arc.intro) {
    return null;
  }
  const signalColor = arc.celebrating ? arc.color : 'var(--attack-map-node-highlight)';
  const emphasized = arc.celebrating || arc.selected || arc.highlighted || arc.active;
  const serviceStyle = getServiceRouteStyle(arc.service);
  const depthOpacity = clamp(0.36 + arc.visibility * 0.64, 0.24, 1);
  const bundleWeight = clamp(Math.log2(arc.count + 1) * 0.22, 0, 0.72);
  const trafficWeight = clamp(0.72 + arc.recency * 0.42 + bundleWeight, 0.72, 1.52);
  const packetEnabled =
    motionEnabled &&
    !dimmed &&
    !arc.partial &&
    !arc.fresh &&
    arc.visibility > 0.16 &&
    (arc.featured || arc.selected || arc.highlighted || arc.recency > 0.86);
  const flyEnabled =
    motionEnabled && !arc.celebrating && !dimmed && arc.fresh && arc.visible && arc.visibility > 0.05;
  const opacity = dimmed
    ? 0.035
    : arc.fresh
      ? flyEnabled
        ? 0.24 * depthOpacity * trafficWeight
        : 0.96
    : arc.partial
      ? arc.featured
        ? 0.92
        : 0.12 * depthOpacity * trafficWeight
      : arc.featured
        ? 0.98
        : emphasized
          ? 0.78 * depthOpacity * trafficWeight
          : 0.2 * depthOpacity * trafficWeight;
  const strokeWidth = arc.celebrating ? '2' : arc.partial
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

  return (
    <g>
      {arc.featured && !arc.celebrating && !dimmed ? (
        <path
          d={arc.path}
          fill="none"
          stroke={arc.color}
          strokeLinecap="round"
          strokeOpacity={arc.partial ? '0.42' : '0.32'}
          strokeWidth={arc.partial ? '12' : '15'}
        />
      ) : null}
      {/* Ink outline under the route: hidden unless a theme shows it. */}
      <path
        d={arc.path}
        data-fx="arc-casing"
        data-dimmed={dimmed ? 'true' : undefined}
        data-emphasized={emphasized ? 'true' : undefined}
        data-fresh={arc.fresh ? 'true' : undefined}
        display="none"
        fill="none"
        pointerEvents="none"
        stroke="var(--attack-map-border)"
        strokeDasharray={arc.partial && !arc.featured ? serviceStyle.partialDash : undefined}
        strokeLinecap="round"
        strokeWidth={Number(strokeWidth) + 3}
      />
      <path
        id={arc.domId}
        data-attack-arc="true"
        data-dimmed={dimmed ? 'true' : undefined}
        data-emphasized={emphasized ? 'true' : undefined}
        data-first-blood={arc.firstBlood ? 'true' : 'false'}
        data-attack-count={String(arc.count)}
        data-attack-id={arc.id}
        data-featured={arc.featured ? 'true' : 'false'}
        data-fresh={arc.fresh ? 'true' : 'false'}
        data-partial={arc.partial ? 'true' : 'false'}
        data-selected={arc.selected ? 'true' : 'false'}
        d={arc.path}
        fill="none"
        markerEnd={emphasized && !arc.partial && !arc.celebrating ? `url(#${markerId})` : undefined}
        stroke={arc.color}
        strokeLinecap="round"
        strokeDasharray={arc.partial && !arc.featured ? serviceStyle.partialDash : undefined}
        strokeOpacity={opacity}
        strokeWidth={strokeWidth}
      />
      {arc.fresh && !arc.celebrating && !dimmed && !flyEnabled ? (
        <path
          d={arc.path}
          fill="none"
          pointerEvents="none"
          stroke={signalColor}
          strokeLinecap="round"
          strokeOpacity="0.78"
          strokeWidth="7"
          className={`${haloClassName}-fresh`}
        />
      ) : null}
      {flyEnabled ? (
        <>
          <path
            d={arc.path}
            fill="none"
            pointerEvents="none"
            stroke={signalColor}
            strokeLinecap="round"
            strokeOpacity="0.22"
            strokeWidth="1.5"
          />
          <path
            d={arc.path}
            fill="none"
            pointerEvents="none"
            stroke={signalColor}
            strokeLinecap="round"
            strokeWidth="3.6"
            pathLength={1}
            strokeDasharray="0.25 1"
            className={`${flyClassName}-bolt`}
          />
          <path
            d={arc.path}
            fill="none"
            pointerEvents="none"
            stroke={signalColor}
            strokeLinecap="round"
            strokeWidth="6"
            pathLength={1}
            strokeDasharray="0.001 1"
            className={`${flyClassName}-head`}
          />
          <circle
            cx={arc.endX}
            cy={arc.endY}
            r="6"
            fill="none"
            pointerEvents="none"
            stroke={signalColor}
            strokeWidth="2.4"
            className={`${flyClassName}-impact`}
          />
        </>
      ) : null}
      {packetEnabled ? (
        <>
          <path
            d={arc.path}
            fill="none"
            pointerEvents="none"
            stroke={arc.featured ? signalColor : arc.color}
            strokeDasharray={arc.featured ? '54 706' : emphasized ? serviceStyle.activeDash : serviceStyle.idleDash}
            strokeLinecap="round"
            strokeOpacity={arc.featured ? '1' : emphasized ? '0.95' : '0.48'}
            strokeWidth={arc.featured ? '5.1' : emphasized ? serviceStyle.activeWidth : serviceStyle.idleWidth}
            className={beamClassName}
          />
          <circle pointerEvents="none" r={arc.featured ? '6.4' : emphasized ? '4.5' : '3.1'} fill={arc.featured ? signalColor : arc.color} opacity={emphasized ? '0.95' : '0.62'}>
            <animateMotion dur={arc.featured ? serviceStyle.featuredDuration : emphasized ? serviceStyle.activeDuration : `${serviceStyle.idleDuration - arc.recency * 0.55}s`} begin={arc.delay} repeatCount="indefinite">
              <mpath href={`#${arc.domId}`} />
            </animateMotion>
          </circle>
        </>
      ) : null}
      {arc.featured && !arc.celebrating && !dimmed ? (
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
  scale = 1,
}: {
  node: TeamNode;
  showLabel: boolean;
  scale?: number;
}): ReactElement | null {
  if (!node.visible || !showLabel) {
    return null;
  }
  const meta = getTeamLabelMeta(node);

  return (
    <g
      data-attack-team-id={node.id}
      data-attack-team-label="true"
      data-attack-team-institution={node.institution || undefined}
      pointerEvents="none"
    >
      <line
        data-fx="team-leader"
        x1={node.x}
        x2={node.labelX}
        y1={node.y}
        y2={node.labelY}
        stroke="var(--attack-map-node-label)"
        strokeOpacity={node.selected ? '0.9' : '0.52'}
      />
      <g transform={`translate(${node.labelX} ${node.labelY}) scale(${scale}) translate(${-node.labelX} ${-node.labelY})`}>
      <TeamLabelPlate node={node} />
      <text
        x={node.labelX}
        y={node.labelY - 7}
        data-fx="team-name"
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
        data-fx="team-meta"
        textAnchor={node.labelAnchor}
        fill="var(--attack-map-node-label)"
        fontSize="9.5"
        fontWeight="500"
        className="select-none font-sans"
      >
        {meta.institution ? (
          <>
            <tspan data-fx="team-institution" fontWeight="600">{meta.institution}</tspan>
            <tspan data-fx="team-meta-separator" opacity="0.6">{' · '}</tspan>
          </>
        ) : null}
        <tspan data-fx="team-meta-count">{meta.count}</tspan>
      </text>
      </g>
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
      aria-label={`${node.selected ? 'Clear' : 'Inspect'} ${node.name}${node.institution ? ` (${node.institution})` : ''}, ${node.outgoing} outgoing and ${node.incoming} incoming attacks`}
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
        data-fx="team-halo"
        fill="var(--attack-map-node-highlight-halo)"
        fillOpacity={emphasized ? '0.3' : '0.08'}
        className={emphasized ? haloClassName : undefined}
      />
      <circle
        cx={node.x}
        cy={node.y}
        r={ringRadius}
        data-fx="team-ring"
        data-emphasized={emphasized ? 'true' : undefined}
        fill="none"
        stroke="var(--attack-map-node-highlight)"
        strokeOpacity={emphasized ? '0.5' : '0.16'}
      />
      <circle
        cx={node.x}
        cy={node.y}
        r={coreRadius}
        data-fx="team-core"
        data-emphasized={emphasized ? 'true' : undefined}
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
      data-fx="team-plate"
      data-selected={node.selected ? 'true' : undefined}
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

// Plates grow to fit an institution so a typical university name reads whole;
// longer ones are cut with an ellipsis (the full name is in the aria-label,
// the jump menu and the inspector).
const LABEL_MAX_WIDTH = 178;
const LABEL_MAX_WIDTH_WITH_INSTITUTION = 216;
// Meta type is 9.5px; the Neubrutalism theme sets it in tracked capitals, so
// size institution room for the wider of the two.
const LABEL_META_CHAR_WIDTH = 5.2;
const LABEL_INSTITUTION_CHAR_WIDTH = 6;

function getTeamLabelMeta(node: TeamNode): { count: string; institution: string } {
  const count = `${node.outgoing} out`;
  if (!node.institution) {
    return { count, institution: '' };
  }
  const room =
    LABEL_MAX_WIDTH_WITH_INSTITUTION - 22 - (count.length + 3) * LABEL_META_CHAR_WIDTH;
  const maxChars = Math.max(8, Math.floor(room / LABEL_INSTITUTION_CHAR_WIDTH));
  const chars = Array.from(node.institution);
  const institution =
    chars.length > maxChars ? `${chars.slice(0, maxChars - 1).join('').trimEnd()}…` : node.institution;
  return { count, institution };
}

function getTeamLabelWidth(node: TeamNode): number {
  const titleWidth = node.name.length * 6.5;
  const meta = getTeamLabelMeta(node);
  const metaWidth = meta.institution
    ? Array.from(meta.institution).length * LABEL_INSTITUTION_CHAR_WIDTH +
      (meta.count.length + 3) * LABEL_META_CHAR_WIDTH
    : meta.count.length * LABEL_META_CHAR_WIDTH;
  const maxWidth = meta.institution ? LABEL_MAX_WIDTH_WITH_INSTITUTION : LABEL_MAX_WIDTH;
  return clamp(Math.max(titleWidth, metaWidth) + 22, 82, maxWidth);
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

function toGlobeVector({ lat, lon }: GeoPoint): GlobeVector {
  const latitude = toRadians(lat);
  const longitude = toRadians(lon);
  const cosLat = Math.cos(latitude);
  return [cosLat * Math.cos(longitude), cosLat * Math.sin(longitude), Math.sin(latitude)];
}

type GlobeLineProjection = {
  sinLon: number;
  cosLon: number;
  sinTilt: number;
  cosTilt: number;
};

function createGlobeLineProjection(rotation: GlobeRotation): GlobeLineProjection {
  const lon = toRadians(rotation.lon);
  const tilt = toRadians(rotation.lat);
  return { sinLon: Math.sin(lon), cosLon: Math.cos(lon), sinTilt: Math.sin(tilt), cosTilt: Math.cos(tilt) };
}

function projectGlobeLine(points: GlobeVector[], { sinLon, cosLon, sinTilt, cosTilt }: GlobeLineProjection) {
  let path = '';
  let drawing = false;
  let fullyVisible = true;
  for (const [x, y, z] of points) {
    const depth = x * cosLon + y * sinLon;
    if (z * sinTilt + depth * cosTilt <= -0.04) {
      drawing = false;
      fullyVisible = false;
      continue;
    }
    const screenX = roundCoord(CENTER_X + GLOBE_RADIUS * (y * cosLon - x * sinLon));
    const screenY = roundCoord(CENTER_Y - GLOBE_RADIUS * (z * cosTilt - depth * sinTilt));
    path += `${drawing ? ' L' : ' M'} ${screenX} ${screenY}`;
    drawing = true;
  }
  return { path, fullyVisible };
}

function parseCountryRings(collection: GlobeFeatureCollection): CountryRing[] {
  const featurePolygons = collection.features.map((feature) => getFeaturePolygons(feature.geometry));
  const tints = assignCountryTints(featurePolygons);
  return featurePolygons.flatMap((polygons, featureIndex) => {
    const tint = tints[featureIndex] ?? 0;
    return polygons.flatMap((polygon) => {
      const outerRing = polygon[0];
      if (!outerRing || outerRing.length < 3) {
        return [];
      }
      return [
        {
          points: outerRing.map(([lon, lat]) => toGlobeVector({ lat, lon })),
          tint,
        },
      ];
    });
  });
}

/**
 * Map-colours the countries: two countries whose outlines meet (on a half
 * degree grid) get different tints, and ties go to the least used tint so the
 * palette stays balanced. Deterministic, so server and client agree.
 */
function assignCountryTints(featurePolygons: Array<Array<Array<Array<[number, number]>>>>): number[] {
  const cells = new Map<string, Set<number>>();
  featurePolygons.forEach((polygons, index) => {
    for (const polygon of polygons) {
      for (const [lon, lat] of polygon[0] ?? []) {
        const key = `${Math.round(lon * 2)}:${Math.round(lat * 2)}`;
        let members = cells.get(key);
        if (!members) {
          members = new Set();
          cells.set(key, members);
        }
        members.add(index);
      }
    }
  });
  const neighbours = featurePolygons.map(() => new Set<number>());
  for (const members of cells.values()) {
    for (const a of members) {
      for (const b of members) {
        if (a !== b) neighbours[a]?.add(b);
      }
    }
  }
  const order = featurePolygons
    .map((_, index) => index)
    .sort((a, b) => (neighbours[b]?.size ?? 0) - (neighbours[a]?.size ?? 0) || a - b);
  const tints = Array.from({ length: featurePolygons.length }, () => -1);
  const usage = Array.from({ length: COUNTRY_TINT_COUNT }, () => 0);
  for (const index of order) {
    const taken = new Set<number>();
    for (const other of neighbours[index] ?? []) taken.add(tints[other] ?? -1);
    let best = -1;
    for (let tint = 0; tint < COUNTRY_TINT_COUNT; tint += 1) {
      if (taken.has(tint)) continue;
      if (best === -1 || (usage[tint] ?? 0) < (usage[best] ?? 0)) best = tint;
    }
    const chosen = best === -1 ? index % COUNTRY_TINT_COUNT : best;
    tints[index] = chosen;
    usage[chosen] = (usage[chosen] ?? 0) + 1;
  }
  return tints;
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
    victim.lon,
    attacker.lon,
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
    // Rounded so server (Node) and browser trig agree on the SSR'd SVG paths;
    // last-bit differences in Math.sin/cos otherwise cause hydration mismatches.
    x: roundCoord(CENTER_X + x),
    y: roundCoord(CENTER_Y - rawY),
    z,
  };
}

function roundCoord(value: number): number {
  return Math.round(value * 100) / 100;
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

/** The point a fraction `t` of the way along the great circle from `from` to `to`. */
function interpolateGreatCircle(from: GeoPoint, to: GeoPoint, t: number): GeoPoint {
  const a = toGlobeVector(from);
  // Exactly antipodal teams have no unique great circle; nudge to pick one.
  let b = toGlobeVector(to);
  let dot = a[0] * b[0] + a[1] * b[1] + a[2] * b[2];
  if (dot < -0.9999) {
    b = toGlobeVector({ lat: to.lat, lon: to.lon + 1.5 });
    dot = a[0] * b[0] + a[1] * b[1] + a[2] * b[2];
  }
  const omega = Math.acos(clamp(dot, -1, 1));
  if (omega < 1e-6) return { lat: from.lat, lon: from.lon };
  const sin = Math.sin(omega);
  const k1 = Math.sin((1 - t) * omega) / sin;
  const k2 = Math.sin(t * omega) / sin;
  const [x, y, z] = [0, 1, 2].map((axis) => a[axis] * k1 + b[axis] * k2);
  return {
    lat: (Math.asin(clamp(z, -1, 1)) * 180) / Math.PI,
    lon: (Math.atan2(y, x) * 180) / Math.PI,
  };
}

function toRadians(value: number): number {
  return (value * Math.PI) / 180;
}

function listAttackServices(attacks: CyberAttackEvent[]): string[] {
  return Array.from(new Set(attacks.map((attack) => attack.service))).sort();
}

function listTeamInstitutions(attacks: CyberAttackEvent[]): Map<string, string> {
  const institutions = new Map<string, string>();
  for (const attack of attacks) {
    if (attack.attacker_institution && !institutions.has(attack.attacker)) {
      institutions.set(attack.attacker, attack.attacker_institution);
    }
    if (attack.victim_institution && !institutions.has(attack.victim)) {
      institutions.set(attack.victim, attack.victim_institution);
    }
  }
  return institutions;
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
    const key = `${attack.attacker}\u0000${attack.victim}\u0000${attack.service}${attack.first_blood ? `\u0000${attack.id}` : ''}`;
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
  'var(--attack-map-route-1)',
  'var(--attack-map-route-2)',
  'var(--attack-map-route-3)',
  'var(--attack-map-route-4)',
  'var(--attack-map-route-5)',
  'var(--attack-map-route-6)',
  'var(--attack-map-route-7)',
  'var(--attack-map-route-8)',
];
