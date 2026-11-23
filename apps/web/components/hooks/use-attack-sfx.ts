'use client';

import { useCallback, useEffect, useRef, useState, type MutableRefObject } from 'react';

import { playFirstBloodScore, playFirstBloodStinger, type FirstBloodSound } from '@/lib/first-blood-audio';

declare global {
  interface Window {
    webkitAudioContext?: typeof AudioContext;
  }
}

type BrowserAudioContext = AudioContext;

const ATTACK_SFX_THROTTLE_MS = 700;
const ATTACK_SFX_MAX_BLIPS = 3;
const ATTACK_SFX_ENABLED_KEY = 'ad-platform-attack-sfx-enabled';
const ATTACK_SFX_VOLUME_KEY = 'ad-platform-attack-sfx-volume';
const ATTACK_SFX_PREFERENCE_EVENT = 'ad-platform:attack-sfx-preferences';
const ATTACK_SFX_DEFAULT_VOLUME = 0.75;

export type AttackSfxPreferences = {
  enabled: boolean;
  volume: number;
};

// The server has no localStorage or media query, so it always renders with these
// values. The client's first (hydration) render must use the exact same object so
// the server HTML and client tree match; the mount effect below then syncs the real
// stored preference. Reading localStorage in the useState initializer instead would
// desync hydration: React keeps the server attributes, and a follow-up setState to
// the already-current value bails out, leaving the control stuck on the SSR default.
const SERVER_DEFAULT_PREFERENCES: AttackSfxPreferences = {
  enabled: true,
  volume: ATTACK_SFX_DEFAULT_VOLUME,
};

export function useAttackSfxPreview() {
  const audioContextRef = useRef<BrowserAudioContext | null>(null);
  const preferencesRef = useRef<AttackSfxPreferences>(readAttackSfxPreferences());

  useEffect(() => {
    function syncPreferences() {
      preferencesRef.current = readAttackSfxPreferences();
    }

    window.addEventListener('storage', syncPreferences);
    window.addEventListener(ATTACK_SFX_PREFERENCE_EVENT, syncPreferences);
    syncPreferences();

    return () => {
      window.removeEventListener('storage', syncPreferences);
      window.removeEventListener(ATTACK_SFX_PREFERENCE_EVENT, syncPreferences);
      const context = audioContextRef.current;
      audioContextRef.current = null;
      if (context && context.state !== 'closed') {
        void context.close().catch(() => {
          // The preview context is disposable; ignore page-leave cleanup failures.
        });
      }
    };
  }, []);

  return useCallback(() => {
    const preferences = preferencesRef.current;
    if (!preferences.enabled || preferences.volume <= 0) {
      return;
    }

    const context = getOrCreateAudioContext(audioContextRef);
    if (!context || context.state === 'closed') {
      return;
    }
    if (context.state === 'suspended') {
      void context.resume().catch(() => {
        // The click already counts as activation in normal browsers.
      });
    }

    const startTime = context.currentTime + 0.015;
    playAttackBlip(context, startTime, 0, preferences.volume);
    playAttackBlip(context, startTime + 0.09, 1, preferences.volume * 0.85);
    playAttackBlip(context, startTime + 0.18, 2, preferences.volume * 0.7);
  }, []);
}

export function useAttackSfxPreferences() {
  const [preferences, setPreferences] = useState<AttackSfxPreferences>(
    SERVER_DEFAULT_PREFERENCES,
  );

  useEffect(() => {
    const syncPreferences = () => setPreferences(readAttackSfxPreferences());

    window.addEventListener('storage', syncPreferences);
    window.addEventListener(ATTACK_SFX_PREFERENCE_EVENT, syncPreferences);
    syncPreferences();

    return () => {
      window.removeEventListener('storage', syncPreferences);
      window.removeEventListener(ATTACK_SFX_PREFERENCE_EVENT, syncPreferences);
    };
  }, []);

  const setEnabled = useCallback((enabled: boolean) => {
    writeAttackSfxPreferences({ enabled });
  }, []);

  const setVolume = useCallback((volume: number) => {
    writeAttackSfxPreferences({ volume });
  }, []);

  const toggleEnabled = useCallback(() => {
    writeAttackSfxPreferences({ enabled: !readAttackSfxPreferences().enabled });
  }, []);

  return { preferences, setEnabled, setVolume, toggleEnabled };
}

// Browsers keep a page's audio permission after the first gesture, but each map
// instance has its own context. Maximizing remounts the map after that gesture.
let pageAudioActivated = false;

export function useAttackSfx({ firstBloodOnly = false }: { firstBloodOnly?: boolean } = {}) {
  const armedRef = useRef(false);
  const audioContextRef = useRef<BrowserAudioContext | null>(null);
  const lastPlayedAtRef = useRef(0);
  const celebratedIdsRef = useRef(new Set<string>());
  const playingMilestoneRef = useRef<FirstBloodSound | null>(null);
  const preferencesRef = useRef<AttackSfxPreferences>(readAttackSfxPreferences());

  const getAudioContext = useCallback(() => {
    return getOrCreateAudioContext(audioContextRef);
  }, []);

  useEffect(() => {
    function syncPreferences() {
      preferencesRef.current = readAttackSfxPreferences();
      if (!preferencesRef.current.enabled || preferencesRef.current.volume <= 0) {
        playingMilestoneRef.current?.stop();
      } else playingMilestoneRef.current?.setVolume(preferencesRef.current.volume);
    }
    function stopHiddenSound() {
      if (document.hidden) playingMilestoneRef.current?.stop();
    }
    document.addEventListener('visibilitychange', stopHiddenSound);

    window.addEventListener('storage', syncPreferences);
    window.addEventListener(ATTACK_SFX_PREFERENCE_EVENT, syncPreferences);
    syncPreferences();

    return () => {
      window.removeEventListener('storage', syncPreferences);
      window.removeEventListener(ATTACK_SFX_PREFERENCE_EVENT, syncPreferences);
      document.removeEventListener('visibilitychange', stopHiddenSound);
    };
  }, []);

  useEffect(() => {
    function armAudio() {
      pageAudioActivated = true;
      armedRef.current = true;
      const context = getAudioContext();
      if (firstBloodOnly && context && preferencesRef.current.enabled) {
        void getFirstBloodAnnouncerBuffer(context).catch(() => {});
      }
      if (context?.state === 'suspended') {
        void context.resume().catch(() => {
          // Browser autoplay policy can still reject this; future attacks stay silent.
        });
      }
    }

    if (pageAudioActivated || navigator.userActivation?.hasBeenActive) armAudio();
    window.addEventListener('pointerdown', armAudio, { passive: true });
    window.addEventListener('keydown', armAudio);
    return () => {
      window.removeEventListener('pointerdown', armAudio);
      window.removeEventListener('keydown', armAudio);
    };
  }, [getAudioContext, firstBloodOnly]);

  useEffect(() => () => {
    playingMilestoneRef.current?.stop();
    const context = audioContextRef.current;
    audioContextRef.current = null;
    if (context && context.state !== 'closed') {
      void context.close().catch(() => {
        // Ignore cleanup failures; the page is leaving this surface.
      });
    }
  }, []);

  return useCallback((attackIds: string[], firstBloodIds: string[] = [], signal?: AbortSignal, spatial?: { attackerPan: number; victimPan: number; quick?: boolean }) => {
    // Milestone sound belongs to the globe's actual sequence. The feed handles
    // ordinary arrivals, so neither remounts nor queued captures double the audio.
    const firstBloodSet = new Set(firstBloodIds);
    const audibleIds = firstBloodOnly ? firstBloodIds : attackIds.filter((id) => !firstBloodSet.has(id));
    if (audibleIds.length === 0 || !armedRef.current || signal?.aborted) {
      return;
    }

    const preferences = preferencesRef.current;
    if (!preferences.enabled || preferences.volume <= 0) {
      return;
    }

    const now = window.performance.now();
    const milestones = (firstBloodOnly ? firstBloodIds : []).filter((id) => !celebratedIdsRef.current.has(id));
    if (firstBloodOnly && milestones.length === 0) return;
    if (milestones.length === 0 && now - lastPlayedAtRef.current < ATTACK_SFX_THROTTLE_MS) {
      return;
    }
    lastPlayedAtRef.current = now;

    const context = getAudioContext();
    if (!context || context.state === 'closed') {
      return;
    }

    if (milestones.length > 0) {
      milestones.forEach((id) => celebratedIdsRef.current.add(id));
      if (context.state !== 'running') {
        // A suspended clock would start the score late against the visuals'
        // timers. Skip this cue (the banner still carries the result) and wake
        // the context for the next one.
        void context.resume().catch(() => {});
        return;
      }
      playingMilestoneRef.current?.stop();
      const playCue = spatial?.quick ? playFirstBloodStinger : playFirstBloodScore;
      playingMilestoneRef.current = playCue(context, {
        volume: preferences.volume,
        signal,
        announcer: getFirstBloodAnnouncerBuffer(context),
        attackerPan: spatial?.attackerPan,
        victimPan: spatial?.victimPan,
      });
      return playingMilestoneRef.current;
    }
    const blipCount = Math.min(audibleIds.length, ATTACK_SFX_MAX_BLIPS);
    const startTime = context.currentTime + 0.015;
    for (let index = 0; index < blipCount; index += 1) {
      playAttackBlip(context, startTime + index * 0.072, index, preferences.volume);
    }
  }, [getAudioContext, firstBloodOnly]);
}

// Share the in-flight decode as well as the result across burst arrivals.
let attackAudioBufferPromise: Promise<AudioBuffer> | null = null;

let firstBloodAnnouncerPromise: Promise<AudioBuffer> | null = null;

function getFirstBloodAnnouncerBuffer(context: BrowserAudioContext): Promise<AudioBuffer> {
  if (!firstBloodAnnouncerPromise) {
    firstBloodAnnouncerPromise = fetch('/first-blood-announcer.mp3')
      .then((response) => {
        if (!response.ok) throw new Error('First blood announcer unavailable');
        return response.arrayBuffer();
      })
      .then((data) => context.decodeAudioData(data))
      .catch((error) => {
        firstBloodAnnouncerPromise = null;
        throw error;
      });
  }
  return firstBloodAnnouncerPromise;
}

function getAttackAudioBuffer(context: BrowserAudioContext): Promise<AudioBuffer> {
  if (!attackAudioBufferPromise) {
    attackAudioBufferPromise = fetch('/laser-gun.mp3')
      .then((response) => {
        if (!response.ok) throw new Error('Attack audio unavailable');
        return response.arrayBuffer();
      })
      .then((data) => context.decodeAudioData(data))
      .catch((error) => {
        attackAudioBufferPromise = null;
        throw error;
      });
  }
  return attackAudioBufferPromise;
}

function playAttackBlip(
  context: BrowserAudioContext,
  startTime: number,
  index: number,
  volume: number,
) {
  getAttackAudioBuffer(context)
    .then((buffer) => {
      const source = context.createBufferSource();
      source.buffer = buffer;
      const gainNode = context.createGain();

      // Scale volume based on preferences and stagger index
      const scaledVolume = volume * 0.5 * Math.pow(0.85, index);
      gainNode.gain.setValueAtTime(scaledVolume, startTime);

      source.connect(gainNode);
      gainNode.connect(context.destination);
      source.onended = () => { source.disconnect(); gainNode.disconnect(); };
      source.start(Math.max(startTime, context.currentTime));
    })
    .catch(() => {
      // Audio is decorative; never let an SFX failure affect live attack updates.
    });
}

function getOrCreateAudioContext(
  audioContextRef: MutableRefObject<BrowserAudioContext | null>,
): BrowserAudioContext | null {
  if (typeof window === 'undefined') {
    return null;
  }
  if (audioContextRef.current) {
    return audioContextRef.current;
  }

  const AudioContextCtor = window.AudioContext ?? window.webkitAudioContext;
  if (!AudioContextCtor) {
    return null;
  }

  try {
    audioContextRef.current = new AudioContextCtor();
    return audioContextRef.current;
  } catch {
    return null;
  }
}

function readAttackSfxPreferences(): AttackSfxPreferences {
  if (typeof window === 'undefined') {
    return { enabled: true, volume: ATTACK_SFX_DEFAULT_VOLUME };
  }

  const storedEnabled = readStoredValue(ATTACK_SFX_ENABLED_KEY);
  const storedVolume = readStoredValue(ATTACK_SFX_VOLUME_KEY);
  // Sound is independent of prefers-reduced-motion; users mute it explicitly.
  const volume = storedVolume === null ? Number.NaN : Number(storedVolume);

  return {
    enabled:
      storedEnabled === null
        ? true
        : storedEnabled === 'on' || storedEnabled === 'true',
    volume: Number.isFinite(volume)
      ? clampVolume(volume)
      : ATTACK_SFX_DEFAULT_VOLUME,
  };
}

function writeAttackSfxPreferences(
  preferences: Partial<AttackSfxPreferences>,
): void {
  if (typeof window === 'undefined') {
    return;
  }

  try {
    if (typeof preferences.enabled === 'boolean') {
      window.localStorage.setItem(
        ATTACK_SFX_ENABLED_KEY,
        preferences.enabled ? 'on' : 'off',
      );
    }
    if (typeof preferences.volume === 'number') {
      window.localStorage.setItem(
        ATTACK_SFX_VOLUME_KEY,
        String(clampVolume(preferences.volume)),
      );
    }
  } catch {
    // Local storage is optional; the in-memory defaults still keep SFX usable.
  }

  window.dispatchEvent(new Event(ATTACK_SFX_PREFERENCE_EVENT));
}

function readStoredValue(key: string): string | null {
  try {
    return window.localStorage.getItem(key);
  } catch {
    return null;
  }
}

function clampVolume(volume: number): number {
  return Math.min(1, Math.max(0, volume));
}
