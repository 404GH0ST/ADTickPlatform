import {
  FIRST_BLOOD_FOCUS_MS,
  FIRST_BLOOD_INTRO_MS,
  FIRST_BLOOD_FLIGHT_MS,
  FIRST_BLOOD_IMPACT_MS,
  FIRST_BLOOD_RECOVER_MS,
} from './first-blood';

export type FirstBloodSound = {
  setVolume: (volume: number) => void;
  stop: () => void;
  finished: Promise<void>;
};

type ScoreOptions = {
  volume: number;
  attackerPan?: number;
  victimPan?: number;
  signal?: AbortSignal;
  announcer?: Promise<AudioBuffer>;
};

const noiseBuffers = new WeakMap<BaseAudioContext, AudioBuffer>();
const clampPan = (value: number) => Math.max(-0.75, Math.min(0.75, value));

/** A complete, bounded score on the audio clock, including the camera's return. */
export function playFirstBloodScore(
  context: BaseAudioContext,
  options: ScoreOptions,
): FirstBloodSound {
  if (options.signal?.aborted)
    return { setVolume() {}, stop() {}, finished: Promise.resolve() };
  const focus = FIRST_BLOOD_FOCUS_MS / 1000;
  const launch = FIRST_BLOOD_INTRO_MS / 1000;
  const impact = launch + FIRST_BLOOD_FLIGHT_MS / 1000;
  const recover = impact + FIRST_BLOOD_IMPACT_MS / 1000;
  const duration = recover + FIRST_BLOOD_RECOVER_MS / 1000;
  const start = context.currentTime + 0.02;
  const from = clampPan(options.attackerPan ?? -0.5);
  const to = clampPan(options.victimPan ?? 0.5);
  const sources: AudioScheduledSourceNode[] = [];
  const nodes: AudioNode[] = [];
  const master = context.createGain();
  const limiter = context.createDynamicsCompressor();
  limiter.threshold.value = -10;
  limiter.knee.value = 12;
  limiter.ratio.value = 6;
  limiter.attack.value = 0.004;
  limiter.release.value = 0.16;
  limiter.connect(master);
  master.connect(context.destination);
  master.gain.value = Math.max(0, Math.min(1, options.volume)) * 0.8;
  nodes.push(limiter, master);
  let stopped = false;
  let remaining = 0;
  let announcerPending = false;
  let resolveFinished: () => void = () => {};
  const finished = new Promise<void>((resolve) => {
    resolveFinished = resolve;
  });

  function cleanup() {
    if (remaining !== 0) return;
    options.signal?.removeEventListener('abort', stop);
    nodes.forEach((node) => node.disconnect());
    resolveFinished();
  }

  function stop() {
    if (stopped) return;
    stopped = true;
    const now = context.currentTime;
    master.gain.cancelScheduledValues(now);
    master.gain.setValueAtTime(master.gain.value, now);
    master.gain.linearRampToValueAtTime(0, now + 0.025);
    if (announcerPending) {
      announcerPending = false;
      remaining -= 1;
    }
    sources.forEach((source) => {
      try {
        source.stop(now + 0.03);
      } catch {
        /* Already ended. */
      }
    });
    cleanup();
  }

  type Point = [seconds: number, value: number];
  function automate(param: AudioParam, points: Point[], offset = 0) {
    points.forEach(([at, value], index) => {
      const time = start + offset + at;
      if (index === 0) param.setValueAtTime(value, time);
      else param.linearRampToValueAtTime(value, time);
    });
  }

  function voice(
    source: OscillatorNode | AudioBufferSourceNode,
    at: number,
    length: number,
    levels: Point[],
    pan: Point[],
    filter?: { type: BiquadFilterType; frequencies: Point[]; q?: number },
  ) {
    const gain = context.createGain();
    const panner = context.createStereoPanner();
    automate(gain.gain, levels, at);
    automate(panner.pan, pan, at);
    if (filter) {
      const eq = context.createBiquadFilter();
      eq.type = filter.type;
      eq.Q.value = filter.q ?? 0.7;
      automate(eq.frequency, filter.frequencies, at);
      source.connect(eq);
      eq.connect(gain);
      nodes.push(eq);
    } else source.connect(gain);
    gain.connect(panner);
    panner.connect(limiter);
    nodes.push(source, gain, panner);
    sources.push(source);
    remaining += 1;
    source.onended = () => {
      remaining -= 1;
      cleanup();
    };
    source.start(start + at);
    source.stop(start + at + length);
  }

  function tone(
    type: OscillatorType,
    at: number,
    length: number,
    frequencies: Point[],
    levels: Point[],
    pan: Point[] = [[0, 0]],
  ) {
    const oscillator = context.createOscillator();
    oscillator.type = type;
    automate(oscillator.frequency, frequencies, at);
    voice(oscillator, at, length, levels, pan);
  }

  function air(
    at: number,
    length: number,
    levels: Point[],
    frequencies: Point[],
    pan: Point[] = [[0, 0]],
    type: BiquadFilterType = 'bandpass',
  ) {
    const source = context.createBufferSource();
    source.buffer = getNoiseBuffer(context);
    source.loop = true;
    voice(source, at, length, levels, pan, { type, frequencies });
  }

  // The low bed and filtered air overlap every cue, then fade together at the
  // final camera settle. There is no silent gap after the impact.
  tone(
    'sine',
    0,
    duration,
    [
      [0, 55],
      [focus, 68],
      [launch, 82.4],
      [impact, 49],
      [recover, 65.4],
      [duration, 55],
    ],
    [
      [0, 0],
      [0.06, 0.045],
      [focus, 0.065],
      [launch, 0.04],
      [impact, 0.09],
      [recover, 0.055],
      [duration - 0.2, 0.025],
      [duration, 0],
    ],
  );
  air(
    0,
    duration,
    [
      [0, 0],
      [0.18, 0.09],
      [focus, 0.055],
      [launch - 0.12, 0.15],
      [launch, 0.06],
      [impact, 0.1],
      [recover, 0.055],
      [duration - 0.25, 0.035],
      [duration, 0],
    ],
    [
      [0, 260],
      [focus, 1100],
      [launch, 2300],
      [impact, 650],
      [recover, 420],
      [duration, 180],
    ],
    [
      [0, 0],
      [focus, from],
      [launch, from],
      [impact, to],
      [recover, to * 0.3],
      [duration, 0],
    ],
  );
  tone(
    'sine',
    0,
    0.34,
    [
      [0, 112],
      [0.34, 48],
    ],
    [
      [0, 0],
      [0.012, 0.28],
      [0.34, 0],
    ],
  );

  // Converging harmonics give the charge weight without a disconnected fanfare.
  [110, 111.7].forEach((frequency, index) => {
    tone(
      'triangle',
      focus,
      launch - focus,
      [
        [0, frequency],
        [launch - focus, 330 + index * 1.4],
      ],
      [
        [0, 0],
        [0.12, 0.025],
        [launch - focus - 0.08, 0.095],
        [launch - focus, 0],
      ],
      [[0, from]],
    );
  });
  [0.08, 0.65, 1.12].forEach((offset, index) => {
    tone(
      'sine',
      focus + offset,
      0.11,
      [
        [0, 660 + index * 110],
        [0.11, 330],
      ],
      [
        [0, 0],
        [0.008, 0.055],
        [0.11, 0],
      ],
      [[0, from]],
    );
  });

  const flight = impact - launch;
  tone(
    'sawtooth',
    launch,
    flight + 0.08,
    [
      [0, 430],
      [flight, 95],
    ],
    [
      [0, 0],
      [0.02, 0.11],
      [flight - 0.04, 0.035],
      [flight + 0.08, 0],
    ],
    [
      [0, from],
      [flight, to],
    ],
  );
  air(
    launch,
    flight + 0.12,
    [
      [0, 0],
      [0.035, 0.18],
      [flight - 0.03, 0.3],
      [flight + 0.12, 0],
    ],
    [
      [0, 3600],
      [flight, 1400],
    ],
    [
      [0, from],
      [flight, to],
    ],
  );

  tone(
    'sine',
    impact,
    1.1,
    [
      [0, 132],
      [0.16, 51],
      [1.1, 31],
    ],
    [
      [0, 0],
      [0.008, 0.62],
      [0.24, 0.27],
      [1.1, 0],
    ],
    [[0, to * 0.45]],
  );
  air(
    impact,
    0.65,
    [
      [0, 0],
      [0.008, 0.46],
      [0.11, 0.19],
      [0.65, 0],
    ],
    [
      [0, 2400],
      [0.65, 320],
    ],
    [[0, to]],
  );
  [0.09, 0.23, 0.42, 0.67].forEach((offset, index) => {
    air(
      impact + offset,
      0.36,
      [
        [0, 0],
        [0.009, 0.09 - index * 0.014],
        [0.36, 0],
      ],
      [
        [0, 3100 - index * 420],
        [0.36, 1100],
      ],
      [[0, clampPan(to + (index % 2 ? -0.3 : 0.3))]],
    );
  });
  tone(
    'sine',
    impact + 0.08,
    duration - impact - 0.08,
    [
      [0, 82.4],
      [duration - impact - 0.08, 55],
    ],
    [
      [0, 0],
      [0.06, 0.08],
      [recover - impact, 0.045],
      [duration - impact - 0.08, 0],
    ],
    [[0, to * 0.2]],
  );

  // Reform / confirmation / return: a short open fifth resolves the tension,
  // accompanied by the reverse camera sweep and a shaped tail to the last frame.
  [130.81, 196].forEach((frequency, index) => {
    const offset = index * 0.12;
    const length = duration - recover - offset;
    tone(
      'sine',
      recover + offset,
      length,
      [
        [0, frequency],
        [length, frequency],
      ],
      [
        [0, 0],
        [0.045, index ? 0.075 : 0.11],
        [length - 0.17, 0.025],
        [length, 0],
      ],
      [
        [0, to * 0.3],
        [length, 0],
      ],
    );
  });
  air(
    recover,
    duration - recover,
    [
      [0, 0],
      [0.22, 0.09],
      [duration - recover - 0.14, 0.025],
      [duration - recover, 0],
    ],
    [
      [0, 850],
      [duration - recover, 220],
    ],
    [
      [0, to * 0.3],
      [duration - recover, 0],
    ],
  );

  if (options.announcer) {
    // Fetch/decode while the animation runs; reserve the graph until this cue
    // ends. A slow or failed asset never delays or removes the effect score.
    announcerPending = true;
    remaining += 1;
    void options.announcer
      .then((buffer) => {
        if (stopped || context.state === 'closed') return;
        const source = context.createBufferSource();
        source.buffer = buffer;
        const at = Math.max(duration, context.currentTime - start + 0.005);
        const length = buffer.duration;
        voice(
          source,
          at,
          length,
          [
            [0, 0],
            [0.008, 1.1],
            [Math.max(0.009, length - 0.025), 1.1],
            [length, 0],
          ],
          [[0, 0]],
        );
      })
      .catch(() => {
        // The complete effect score remains available if the announcer cannot load.
      })
      .finally(() => {
        if (announcerPending) {
          announcerPending = false;
          remaining -= 1;
          cleanup();
        }
      });
  }
  options.signal?.addEventListener('abort', stop, { once: true });
  return {
    stop,
    finished,
    setVolume(volume) {
      if (stopped) return;
      master.gain.setTargetAtTime(
        Math.max(0, Math.min(1, volume)) * 0.8,
        context.currentTime,
        0.025,
      );
    },
  };
}

function getNoiseBuffer(context: BaseAudioContext): AudioBuffer {
  const cached = noiseBuffers.get(context);
  if (cached) return cached;
  const buffer = context.createBuffer(
    1,
    context.sampleRate * 2,
    context.sampleRate,
  );
  const channel = buffer.getChannelData(0);
  let seed = 0x1f2e3d4c;
  let low = 0;
  for (let index = 0; index < channel.length; index += 1) {
    seed ^= seed << 13;
    seed ^= seed >>> 17;
    seed ^= seed << 5;
    const white = (seed >>> 0) / 0x80000000 - 1;
    low = low * 0.96 + white * 0.04;
    channel[index] = white * 0.6 + low * 0.4;
  }
  noiseBuffers.set(context, buffer);
  return buffer;
}
