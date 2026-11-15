import {
  FIRST_BLOOD_ANNOUNCER_AT_MS,
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

/**
 * A complete, bounded score on the audio clock, including the camera's return.
 * The announcer (when supplied) speaks right after the impact, over the return.
 */
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
  const voiceAt = FIRST_BLOOD_ANNOUNCER_AT_MS / 1000;
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
  // Effects run through their own bus so they can duck under the announcer.
  const scoreBus = context.createGain();
  scoreBus.connect(limiter);
  // Overdrive stage for the impact: a tanh shaper, trimmed back to bus level.
  const drive = context.createWaveShaper();
  drive.curve = getDriveCurve();
  drive.oversample = '4x';
  const driveTrim = context.createGain();
  driveTrim.gain.value = 0.42;
  // The impact runs through its own bus with a short synthetic room, so the
  // hit blooms into space instead of sounding dry and buzzy.
  const impactBus = context.createGain();
  const roomSend = context.createGain();
  const room = context.createConvolver();
  room.buffer = getRoomImpulse(context);
  roomSend.gain.value = 0.5;
  impactBus.connect(scoreBus);
  impactBus.connect(roomSend);
  roomSend.connect(room);
  room.connect(scoreBus);
  drive.connect(driveTrim);
  driveTrim.connect(impactBus);
  nodes.push(limiter, master, scoreBus, drive, driveTrim, impactBus, roomSend, room);
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
    destination: AudioNode = scoreBus,
    bufferOffset = 0,
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
    panner.connect(destination);
    // Effect layers never outlast the score; only the announcer may run past it.
    if (destination !== limiter) length = Math.min(length, Math.max(0.001, duration - at));
    nodes.push(source, gain, panner);
    sources.push(source);
    remaining += 1;
    source.onended = () => {
      remaining -= 1;
      cleanup();
    };
    // A buffer offset decorrelates noise layers (e.g. a wide L/R boom).
    if (bufferOffset > 0) (source as AudioBufferSourceNode).start(start + at, bufferOffset);
    else source.start(start + at);
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
    destination?: AudioNode,
    q?: number,
    bufferOffset = 0,
  ) {
    const source = context.createBufferSource();
    source.buffer = getNoiseBuffer(context);
    source.loop = true;
    voice(source, at, length, levels, pan, { type, frequencies, q }, destination, bufferOffset);
  }

  /** An oscillator through a moving filter (risers, driven layers). */
  function filtered(
    type: OscillatorType,
    at: number,
    length: number,
    frequencies: Point[],
    levels: Point[],
    pan: Point[],
    filter: { type: BiquadFilterType; frequencies: Point[]; q?: number },
    destination?: AudioNode,
    detune = 0,
  ) {
    const oscillator = context.createOscillator();
    oscillator.type = type;
    oscillator.detune.value = detune;
    automate(oscillator.frequency, frequencies, at);
    voice(oscillator, at, length, levels, pan, filter, destination);
  }

  // A tiny deterministic generator so every render of the score is identical.
  let seed = 0x5eed1;
  function random(): number {
    seed = (seed * 1664525 + 1013904223) >>> 0;
    return seed / 0x100000000;
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
  [0.05, 0.4, 0.72].forEach((fraction, index) => {
    const offset = fraction * (launch - focus);
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
  // A clean sine "laser" for the strike; a falling sawtooth here read as a
  // deflating buzz that bled into the impact.
  tone(
    'sine',
    launch,
    flight + 0.08,
    [
      [0, 1500],
      [flight, 420],
    ],
    [
      [0, 0],
      [0.02, 0.09],
      [flight - 0.04, 0.05],
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

  // Impact. Every layer avoids slow low pitch glides (they read as comic);
  // the punch drops in pitch within 50ms like a kick drum, and the body,
  // crunch, crack and shatter are noise and short bright partials.
  const punch = context.createOscillator();
  punch.type = 'sine';
  automate(punch.frequency, [[0, 160], [0.05, 54], [0.9, 46]], impact);
  voice(punch, impact, 0.9, [[0, 0], [0.003, 0.6], [0.12, 0.3], [0.9, 0]], [[0, to * 0.4]], undefined, impactBus);
  air(impact, 0.32, [[0, 0], [0.004, 0.5], [0.32, 0]],
    [[0, 700], [0.32, 140]], [[0, to * 0.5]], 'lowpass', impactBus);
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
    'bandpass',
    impactBus,
  );
  // A bright crack on contact carries the hit on small speakers.
  air(
    impact,
    0.09,
    [
      [0, 0],
      [0.003, 0.5],
      [0.09, 0],
    ],
    [
      [0, 1800],
      [0.09, 1200],
    ],
    [[0, to]],
    'highpass',
    impactBus,
  );
  // Shatter: short inharmonic glassy pings for the flying shards.
  [1870, 2650, 3420, 4180, 5230, 6110].forEach((frequency, index) => {
    const at = impact + 0.01 + index * 0.017;
    const length = 0.3 - index * 0.03;
    tone('sine', at, length, [[0, frequency]],
      [[0, 0], [0.002, 0.05 - index * 0.004], [length, 0]],
      [[0, clampPan(to + (index % 2 ? 0.35 : -0.35))]]);
  });
  [0.06, 0.16, 0.29, 0.46].forEach((offset, index) => {
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

  // ---- Overdrive layers, one per visual beat. ----

  // Focus: the reticles lock on, attacker then victim, as double chirps.
  [[0.16, from], [0.52, to]].forEach(([at, pan]) => {
    [0, 0.065].forEach((offset, index) => {
      tone('square', at + offset, 0.045, [[0, index ? 2093 : 1568]],
        [[0, 0], [0.004, 0.065], [0.045, 0]], [[0, pan]]);
    });
  });
  // The camera fly-in, audible on small speakers: a rising filtered whoosh.
  air(0, focus, [[0, 0], [focus * 0.7, 0.16], [focus, 0.04]],
    [[0, 500], [focus, 2600]], [[0, -0.3], [focus, from]], 'bandpass', undefined, 1.2);

  // Charge: a detuned saw riser opening up, a growing rumble, and lightning
  // crackle whose density climbs toward the launch.
  const charge = launch - focus;
  [-9, 9].forEach((detune) => {
    filtered('sawtooth', focus, charge, [[0, 110], [charge, 330]],
      [[0, 0], [charge * 0.6, 0.05], [charge - 0.06, 0.12], [charge - 0.03, 0]],
      [[0, from]], { type: 'lowpass', frequencies: [[0, 300], [charge, 3200]], q: 4 }, undefined, detune);
  });
  air(focus, charge, [[0, 0], [charge - 0.1, 0.32], [charge - 0.03, 0]],
    [[0, 70], [charge, 240]], [[0, from]], 'lowpass');
  const crackles = 30;
  for (let index = 0; index < crackles; index += 1) {
    const at = focus + 0.08 + (charge - 0.3) * Math.sqrt(index / crackles);
    const level = 0.05 + 0.13 * (index / crackles);
    air(at, 0.028, [[0, 0], [0.002, level], [0.028, 0]],
      [[0, 2400 + random() * 3800]], [[0, clampPan(from + (random() - 0.5) * 0.5)]], 'bandpass', undefined, 6);
  }
  // A sharp breath in, then 30ms of near-silence before the shot.
  air(launch - 0.28, 0.25, [[0, 0], [0.24, 0.3], [0.25, 0]],
    [[0, 900], [0.25, 6000]], [[0, from]], 'highpass');

  // Launch: a muzzle kick and crack at the attacker.
  tone('sine', launch, 0.18, [[0, 180], [0.045, 58], [0.18, 50]],
    [[0, 0], [0.003, 0.5], [0.18, 0]], [[0, from]]);
  air(launch, 0.07, [[0, 0], [0.002, 0.36], [0.07, 0]],
    [[0, 3000], [0.07, 900]], [[0, from]]);

  // Impact: sparks spraying around the victim,
  // and a shockwave that sweeps across the stereo field like the globe ring.
  // ---- Explosion: vacuum, boom, blast, rumble, debris. ----

  // Vacuum: a reverse swell sucks in, then the whole score dips for a beat
  // so the detonation lands on contrast.
  air(impact - 0.2, 0.185, [[0, 0], [0.17, 0.3], [0.185, 0]],
    [[0, 700], [0.185, 5200]], [[0, to * 0.6]], 'highpass', limiter, 0.7, 0.41);
  scoreBus.gain.setValueAtTime(1, start + impact - 0.09);
  scoreBus.gain.linearRampToValueAtTime(0.3, start + impact - 0.012);
  scoreBus.gain.setValueAtTime(1, start + impact);

  // Boom: wide, decorrelated low noise whose filter closes as it decays,
  // the body of a real blast (a pitched tone here reads as comic).
  [[-0.65, 0.13], [0.65, 0.97]].forEach(([pan, offset]) => {
    air(impact, 1.8,
      [[0, 0], [0.005, 0.85], [0.22, 0.5], [0.9, 0.2], [1.8, 0]],
      [[0, 1100], [0.07, 300], [0.6, 150], [1.8, 70]],
      [[0, clampPan(to * 0.3 + pan)]], 'lowpass', impactBus, 0.5, offset);
  });

  // Blast: the bright, torn front of the explosion, driven hard.
  air(impact, 0.75, [[0, 0], [0.003, 1], [0.12, 0.6], [0.75, 0]],
    [[0, 5200], [0.18, 1600], [0.75, 380]], [[0, to * 0.5]], 'lowpass', drive, 0.7, 0.55);

  // Rumble: rolling low bursts after the blast, like distant thunder.
  [0.2, 0.46, 0.76, 1.08].forEach((offset, index) => {
    air(impact + offset, 0.75,
      [[0, 0], [0.12, 0.3 - index * 0.055], [0.75, 0]],
      [[0, 240], [0.75, 110]],
      [[0, index % 2 ? -0.5 : 0.5]], 'lowpass', impactBus, 0.5, 0.2 + index * 0.37);
  });

  // Debris: crackles that thin out as the rubble settles.
  for (let index = 0; index < 42; index += 1) {
    const u = index / 42;
    const at = impact + 0.08 + 1.35 * u * u;
    air(at, 0.022, [[0, 0], [0.002, 0.03 + 0.15 * (1 - u) ** 1.3], [0.022, 0]],
      [[0, 700 + random() * 2100]], [[0, clampPan((random() - 0.5) * 1.5)]], 'bandpass', impactBus, 3, random() * 1.8);
  }
  for (let index = 0; index < 18; index += 1) {
    const at = impact + 0.04 + random() * 0.6;
    air(at, 0.014, [[0, 0], [0.001, 0.06 + random() * 0.06], [0.014, 0]],
      [[0, 4500 + random() * 3500]], [[0, clampPan(to + (random() - 0.5) * 1.2)]], 'highpass');
  }
  air(impact + 0.05, 1.2, [[0, 0], [0.04, 0.22], [1.2, 0]],
    [[0, 3200], [1.2, 240]], [[0, to], [0.6, -to * 0.6], [1.2, 0]], 'bandpass', undefined, 1.4);

  // Banner: a low slam with a short metallic ring, then a shimmer for the
  // light sweep across the panel.
  tone('sine', recover, 0.3, [[0, 130], [0.04, 58], [0.3, 52]],
    [[0, 0], [0.003, 0.36], [0.3, 0]]);
  air(recover, 0.18, [[0, 0], [0.003, 0.3], [0.18, 0]],
    [[0, 520], [0.18, 160]], [[0, 0]], 'lowpass', impactBus);
  air(recover, 0.09, [[0, 0], [0.002, 0.34], [0.09, 0]],
    [[0, 1500], [0.09, 700]], [[0, 0]], 'bandpass', undefined, 1.1);
  [523.3, 1187, 1873].forEach((frequency, index) => {
    tone('sine', recover + 0.005, 0.9, [[0, frequency]],
      [[0, 0], [0.004, 0.045 - index * 0.01], [0.9, 0]], [[0, (index - 1) * 0.35]]);
  });
  const shimmer = duration - recover - 0.3;
  air(recover + 0.3, shimmer, [[0, 0], [shimmer * 0.5, 0.05], [shimmer, 0]],
    [[0, 2000], [shimmer, 9000]], [[0, -0.5], [shimmer, 0.5]], 'bandpass', undefined, 3);

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
        const at = Math.max(voiceAt, context.currentTime - start + 0.005);
        // A voice arriving after the camera settles would read as a separate
        // event; past that point the banner alone carries the result.
        if (at > recover) return;
        const source = context.createBufferSource();
        source.buffer = buffer;
        const length = buffer.duration;
        const duck = start + at;
        scoreBus.gain.setValueAtTime(1, duck);
        scoreBus.gain.linearRampToValueAtTime(0.55, duck + 0.06);
        // Release once the line ends: a short announcer must not leave the
        // banner slam and closing chord ducked for the rest of the score.
        const released = Math.max(duck + 0.06, duck + length);
        scoreBus.gain.setValueAtTime(0.55, released);
        scoreBus.gain.linearRampToValueAtTime(1, released + 0.2);
        voice(
          source,
          at,
          length,
          [
            [0, 0],
            [0.008, 0.8],
            [Math.max(0.009, length - 0.025), 0.8],
            [length, 0],
          ],
          [[0, 0]],
          undefined,
          limiter,
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

const roomImpulses = new WeakMap<BaseAudioContext, AudioBuffer>();
/** A short decaying stereo noise tail: a cheap, deterministic room. */
function getRoomImpulse(context: BaseAudioContext): AudioBuffer {
  const cached = roomImpulses.get(context);
  if (cached) return cached;
  const length = Math.floor(context.sampleRate * 1.4);
  const buffer = context.createBuffer(2, length, context.sampleRate);
  let seed = 0x2b7e1516;
  for (let channel = 0; channel < 2; channel += 1) {
    const data = buffer.getChannelData(channel);
    for (let index = 0; index < length; index += 1) {
      seed ^= seed << 13;
      seed ^= seed >>> 17;
      seed ^= seed << 5;
      const white = (seed >>> 0) / 0x80000000 - 1;
      const t = index / context.sampleRate;
      // Fast early decay, then a softer tail; a little pre-delay for size.
      data[index] = t < 0.012 ? 0 : white * (Math.exp(-t * 5.5) * 0.7 + Math.exp(-t * 2.2) * 0.3);
    }
  }
  roomImpulses.set(context, buffer);
  return buffer;
}

let driveCurve: Float32Array<ArrayBuffer> | null = null;
function getDriveCurve(): Float32Array<ArrayBuffer> {
  if (driveCurve) return driveCurve;
  const samples = 1024;
  driveCurve = new Float32Array(samples);
  for (let index = 0; index < samples; index += 1) {
    const x = (index / (samples - 1)) * 2 - 1;
    driveCurve[index] = Math.tanh(x * 6) / Math.tanh(6);
  }
  return driveCurve;
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
