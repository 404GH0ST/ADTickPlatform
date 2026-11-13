// One shared timeline for the globe camera, charge, strike, impact and audio.
export const FIRST_BLOOD_FOCUS_MS = 1200;
export const FIRST_BLOOD_CHARGE_MS = 1600;
export const FIRST_BLOOD_INTRO_MS =
  FIRST_BLOOD_FOCUS_MS + FIRST_BLOOD_CHARGE_MS;
export const FIRST_BLOOD_FLIGHT_MS = 1050;
export const FIRST_BLOOD_IMPACT_MS = 1600;
export const FIRST_BLOOD_RECOVER_MS = 1000;
export const FIRST_BLOOD_ATTACK_MS =
  FIRST_BLOOD_FLIGHT_MS + FIRST_BLOOD_IMPACT_MS + FIRST_BLOOD_RECOVER_MS;
export const FIRST_BLOOD_TOTAL_MS =
  FIRST_BLOOD_INTRO_MS + FIRST_BLOOD_ATTACK_MS;
// The result remains readable even when sound is muted or unavailable.
export const FIRST_BLOOD_BANNER_HOLD_MS = 4000;
export const FIRST_BLOOD_BANNER_EXIT_MS = 280;
export type FirstBloodStage =
  'focus' | 'charge' | 'flight' | 'impact' | 'recover';
