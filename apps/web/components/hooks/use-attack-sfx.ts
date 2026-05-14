'use client';

import { useCallback, useEffect, useRef } from 'react';

declare global {
  interface Window {
    webkitAudioContext?: typeof AudioContext;
  }
}

type BrowserAudioContext = AudioContext;

const ATTACK_SFX_THROTTLE_MS = 700;
const ATTACK_SFX_MAX_BLIPS = 3;

export function useAttackSfx() {
  const armedRef = useRef(false);
  const audioContextRef = useRef<BrowserAudioContext | null>(null);
  const lastPlayedAtRef = useRef(0);

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
      playAttackBlip(context, startTime + index * 0.072, index);
    }
  }, [getAudioContext]);
}

function playAttackBlip(
  context: BrowserAudioContext,
  startTime: number,
  index: number,
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
    gain.gain.exponentialRampToValueAtTime(0.06, startTime + 0.012);
    gain.gain.exponentialRampToValueAtTime(0.0001, startTime + 0.11);

    oscillator.connect(gain);
    gain.connect(context.destination);
    oscillator.start(startTime);
    oscillator.stop(startTime + 0.12);
  } catch {
    // Audio is decorative; never let an SFX failure affect live attack updates.
  }
}
