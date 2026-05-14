'use client';

import { useCallback, useEffect, useRef, useState } from 'react';

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

export function useAttackSfxPreferences() {
  const [preferences, setPreferences] = useState<AttackSfxPreferences>(() =>
    readAttackSfxPreferences(),
  );

  useEffect(() => {
    const mediaQuery =
      typeof window === 'undefined' || !window.matchMedia
        ? null
        : window.matchMedia('(prefers-reduced-motion: reduce)');
    const syncPreferences = () => setPreferences(readAttackSfxPreferences());

    window.addEventListener('storage', syncPreferences);
    window.addEventListener(ATTACK_SFX_PREFERENCE_EVENT, syncPreferences);
    mediaQuery?.addEventListener('change', syncPreferences);
    syncPreferences();

    return () => {
      window.removeEventListener('storage', syncPreferences);
      window.removeEventListener(ATTACK_SFX_PREFERENCE_EVENT, syncPreferences);
      mediaQuery?.removeEventListener('change', syncPreferences);
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

export function useAttackSfx() {
  const armedRef = useRef(false);
  const audioContextRef = useRef<BrowserAudioContext | null>(null);
  const lastPlayedAtRef = useRef(0);
  const preferencesRef = useRef<AttackSfxPreferences>(readAttackSfxPreferences());

  const getAudioContext = useCallback(() => {
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
  }, []);

  useEffect(() => {
    function syncPreferences() {
      preferencesRef.current = readAttackSfxPreferences();
    }

    const mediaQuery =
      typeof window === 'undefined' || !window.matchMedia
        ? null
        : window.matchMedia('(prefers-reduced-motion: reduce)');

    window.addEventListener('storage', syncPreferences);
    window.addEventListener(ATTACK_SFX_PREFERENCE_EVENT, syncPreferences);
    mediaQuery?.addEventListener('change', syncPreferences);
    syncPreferences();

    return () => {
      window.removeEventListener('storage', syncPreferences);
      window.removeEventListener(ATTACK_SFX_PREFERENCE_EVENT, syncPreferences);
      mediaQuery?.removeEventListener('change', syncPreferences);
    };
  }, []);

  useEffect(() => {
    function armAudio() {
      armedRef.current = true;
      const context = getAudioContext();
      if (context?.state === 'suspended') {
        void context.resume().catch(() => {
          // Browser autoplay policy can still reject this; future attacks stay silent.
        });
      }
    }

    window.addEventListener('pointerdown', armAudio, { passive: true });
    window.addEventListener('keydown', armAudio);
    return () => {
      window.removeEventListener('pointerdown', armAudio);
      window.removeEventListener('keydown', armAudio);
    };
  }, [getAudioContext]);

  useEffect(() => () => {
    const context = audioContextRef.current;
    audioContextRef.current = null;
    if (context && context.state !== 'closed') {
      void context.close().catch(() => {
        // Ignore cleanup failures; the page is leaving this surface.
      });
    }
  }, []);

  return useCallback((attackIds: string[]) => {
    if (attackIds.length === 0 || !armedRef.current) {
      return;
    }

    const preferences = preferencesRef.current;
    if (!preferences.enabled || preferences.volume <= 0) {
      return;
    }

    const now = window.performance.now();
    if (now - lastPlayedAtRef.current < ATTACK_SFX_THROTTLE_MS) {
      return;
    }
    lastPlayedAtRef.current = now;

    const context = getAudioContext();
    if (!context || context.state === 'closed') {
      return;
    }

    const blipCount = Math.min(attackIds.length, ATTACK_SFX_MAX_BLIPS);
    const startTime = context.currentTime + 0.015;
    for (let index = 0; index < blipCount; index += 1) {
      playAttackBlip(context, startTime + index * 0.072, index, preferences.volume);
    }
  }, [getAudioContext]);
}

function playAttackBlip(
  context: BrowserAudioContext,
  startTime: number,
  index: number,
  volume: number,
) {
  try {
    const oscillator = context.createOscillator();
    const gain = context.createGain();
    const baseFrequency = 540 + index * 92;

    oscillator.type = 'triangle';
    oscillator.frequency.setValueAtTime(baseFrequency, startTime);
    oscillator.frequency.exponentialRampToValueAtTime(
      baseFrequency * 1.9,
      startTime + 0.045,
    );

    gain.gain.setValueAtTime(0.0001, startTime);
    gain.gain.exponentialRampToValueAtTime(0.06 * volume, startTime + 0.012);
    gain.gain.exponentialRampToValueAtTime(0.0001, startTime + 0.11);

    oscillator.connect(gain);
    gain.connect(context.destination);
    oscillator.start(startTime);
    oscillator.stop(startTime + 0.12);
  } catch {
    // Audio is decorative; never let an SFX failure affect live attack updates.
  }
}

function readAttackSfxPreferences(): AttackSfxPreferences {
  if (typeof window === 'undefined') {
    return { enabled: true, volume: ATTACK_SFX_DEFAULT_VOLUME };
  }

  const storedEnabled = readStoredValue(ATTACK_SFX_ENABLED_KEY);
  const storedVolume = readStoredValue(ATTACK_SFX_VOLUME_KEY);
  const reducedMotion =
    window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false;
  const volume = storedVolume === null ? Number.NaN : Number(storedVolume);

  return {
    enabled:
      storedEnabled === null
        ? !reducedMotion
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
