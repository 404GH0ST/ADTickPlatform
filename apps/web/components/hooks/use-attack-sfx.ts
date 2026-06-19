'use client';

import { useCallback, useEffect, useRef, useState, type MutableRefObject } from 'react';

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
    return getOrCreateAudioContext(audioContextRef);
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

let cachedAudioBuffer: AudioBuffer | null = null;

async function getAttackAudioBuffer(context: BrowserAudioContext): Promise<AudioBuffer> {
  if (cachedAudioBuffer) {
    return cachedAudioBuffer;
  }
  const response = await fetch('/freesound_community-laser-gun-81720.mp3');
  const arrayBuffer = await response.arrayBuffer();
  cachedAudioBuffer = await context.decodeAudioData(arrayBuffer);
  return cachedAudioBuffer;
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
      source.start(startTime);
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
