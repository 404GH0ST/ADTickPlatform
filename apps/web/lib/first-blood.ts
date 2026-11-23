// One shared timeline for the globe camera, charge, strike, impact and audio.
// ~6.6s: a full cinematic beat structure that still reads as one live moment.
export const FIRST_BLOOD_FOCUS_MS = 1200;
export const FIRST_BLOOD_CHARGE_MS = 1700;
export const FIRST_BLOOD_INTRO_MS =
  FIRST_BLOOD_FOCUS_MS + FIRST_BLOOD_CHARGE_MS;
export const FIRST_BLOOD_FLIGHT_MS = 1100;
export const FIRST_BLOOD_IMPACT_MS = 1600;
export const FIRST_BLOOD_RECOVER_MS = 1000;
export const FIRST_BLOOD_ATTACK_MS =
  FIRST_BLOOD_FLIGHT_MS + FIRST_BLOOD_IMPACT_MS + FIRST_BLOOD_RECOVER_MS;
export const FIRST_BLOOD_TOTAL_MS =
  FIRST_BLOOD_INTRO_MS + FIRST_BLOOD_ATTACK_MS;
export const FIRST_BLOOD_HIT_AT_MS =
  FIRST_BLOOD_INTRO_MS + FIRST_BLOOD_FLIGHT_MS;
// The announcer lands just after the impact transient, so the hit reads first
// and the voice is the payoff rather than a separate second ending.
export const FIRST_BLOOD_ANNOUNCER_AT_MS = FIRST_BLOOD_HIT_AT_MS + 120;
// The result banner appears as the camera returns, while the voice still speaks.
export const FIRST_BLOOD_BANNER_AT_MS =
  FIRST_BLOOD_HIT_AT_MS + FIRST_BLOOD_IMPACT_MS;
// The result remains readable even when sound is muted or unavailable.
export const FIRST_BLOOD_BANNER_HOLD_MS = 4500;
export const FIRST_BLOOD_BANNER_EXIT_MS = 380;
export type FirstBloodStage =
  'focus' | 'charge' | 'flight' | 'impact' | 'recover';

export type FirstBloodTimeline = {
  pace: 'full' | 'quick';
  focusMs: number;
  chargeMs: number;
  introMs: number;
  flightMs: number;
  impactMs: number;
  recoverMs: number;
  totalMs: number;
  hitAtMs: number;
  announcerAtMs: number;
  bannerAtMs: number;
  bannerHoldMs: number;
};

function buildTimeline(
  pace: FirstBloodTimeline['pace'],
  focusMs: number,
  chargeMs: number,
  flightMs: number,
  impactMs: number,
  recoverMs: number,
  bannerHoldMs: number,
): FirstBloodTimeline {
  const introMs = focusMs + chargeMs;
  const hitAtMs = introMs + flightMs;
  return {
    pace, focusMs, chargeMs, introMs, flightMs, impactMs, recoverMs, bannerHoldMs, hitAtMs,
    totalMs: hitAtMs + impactMs + recoverMs,
    announcerAtMs: hitAtMs + 120,
    bannerAtMs: hitAtMs + impactMs,
  };
}

export const FIRST_BLOOD_FULL = buildTimeline(
  'full',
  FIRST_BLOOD_FOCUS_MS,
  FIRST_BLOOD_CHARGE_MS,
  FIRST_BLOOD_FLIGHT_MS,
  FIRST_BLOOD_IMPACT_MS,
  FIRST_BLOOD_RECOVER_MS,
  FIRST_BLOOD_BANNER_HOLD_MS,
);
// A first blood that waited its turn keeps its own camera move, strike and
// banner in about 5.5s: long enough to read, short enough to stay near live.
export const FIRST_BLOOD_QUICK = buildTimeline('quick', 700, 600, 700, 900, 500, 2200);
