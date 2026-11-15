import { execFileSync } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { expect, test } from '@playwright/test';

let bundle: string;
let directory: string;
let announcer: string;

test.beforeAll(() => {
  directory = mkdtempSync(join(tmpdir(), 'ad-first-blood-audio-'));
  const output = join(directory, 'score.js');
  execFileSync('bun', [
    'build',
    'lib/first-blood-audio.ts',
    '--target=browser',
    '--format=esm',
    `--outfile=${output}`,
  ]);
  bundle = readFileSync(output, 'utf8');
  announcer = readFileSync('public/first-blood-announcer.mp3').toString('base64');
});
test.afterAll(() => {
  if (directory) rmSync(directory, { recursive: true, force: true });
});

test('real first-blood mix covers every animation phase and lands the announcer on the impact', async ({
  page,
}, testInfo) => {
  const result = await page.evaluate(
    async ({ code, mp3 }) => {
      const url = URL.createObjectURL(
        new Blob([code], { type: 'text/javascript' }),
      );
      const { playFirstBloodScore } = await import(url);
      URL.revokeObjectURL(url);
      const sampleRate = 44100;
      const decoder = new OfflineAudioContext(2, 1, sampleRate);
      const bytes = Uint8Array.from(atob(mp3), (value) => value.charCodeAt(0));
      const clip = await decoder.decodeAudioData(bytes.buffer);
      // Announcer at impact + 120ms (4.12s); the 6.6s score fades under it.
      const voiceAt = 4.12;
      // Render until both the 6.6s score and the announcer have finished.
      const duration = Math.max(6.6, voiceAt + clip.duration) + 0.15;
      async function render(withAnnouncer: boolean) {
        const context = new OfflineAudioContext(
          2,
          Math.ceil(duration * sampleRate),
          sampleRate,
        );
        const sound = playFirstBloodScore(context, {
          volume: 1,
          attackerPan: 0.6,
          victimPan: -0.5,
          announcer: withAnnouncer ? Promise.resolve(clip) : undefined,
        });
        const output = await context.startRendering();
        await sound.finished;
        return output;
      }
      const mix = await render(true);
      const effects = await render(false);
      const left = mix.getChannelData(0);
      const right = mix.getChannelData(1);
      const dry = effects.getChannelData(0);
      function rms(start: number, end: number, data = left) {
        let sum = 0;
        let count = 0;
        for (
          let index = Math.floor(start * sampleRate);
          index < Math.min(data.length, end * sampleRate);
          index += 1
        ) {
          sum += data[index] ** 2;
          count += 1;
        }
        return Math.sqrt(sum / Math.max(1, count));
      }
      let peak = 0;
      let preAnnouncerDifference = 0;
      for (let index = 0; index < left.length; index += 1) {
        peak = Math.max(peak, Math.abs(left[index]), Math.abs(right[index]));
        if (index < voiceAt * sampleRate)
          preAnnouncerDifference = Math.max(
            preAnnouncerDifference,
            Math.abs(left[index] - dry[index]),
          );
      }
      // After the line and its 0.2s duck release, the score is back at full
      // level, so its tail matches the effects-only render (still ducked: ~0.55).
      const releasedAt = voiceAt + clip.duration + 0.25;
      const postAnnouncerLevel =
        rms(releasedAt + 0.15, 6.5) / Math.max(1e-9, rms(releasedAt + 0.15, 6.5, dry));
      // Continuous from the first frame through the announcer's spoken line.
      const windows = Array.from({ length: 25 }, (_, index) =>
        rms(0.04 + index * 0.25, 0.29 + index * 0.25),
      );
      // Attach the actual rendered stereo mix for listening, not only mock calls.
      const wav = new Uint8Array(44 + mix.length * 4);
      const view = new DataView(wav.buffer);
      function label(offset: number, text: string) {
        for (let i = 0; i < text.length; i += 1)
          view.setUint8(offset + i, text.charCodeAt(i));
      }
      label(0, 'RIFF');
      view.setUint32(4, wav.length - 8, true);
      label(8, 'WAVE');
      label(12, 'fmt ');
      view.setUint32(16, 16, true);
      view.setUint16(20, 1, true);
      view.setUint16(22, 2, true);
      view.setUint32(24, sampleRate, true);
      view.setUint32(28, sampleRate * 4, true);
      view.setUint16(32, 4, true);
      view.setUint16(34, 16, true);
      label(36, 'data');
      view.setUint32(40, wav.length - 44, true);
      for (let index = 0; index < mix.length; index += 1) {
        view.setInt16(
          44 + index * 4,
          Math.max(-1, Math.min(1, left[index])) * 32767,
          true,
        );
        view.setInt16(
          46 + index * 4,
          Math.max(-1, Math.min(1, right[index])) * 32767,
          true,
        );
      }
      let binary = '';
      for (let offset = 0; offset < wav.length; offset += 8192)
        binary += String.fromCharCode(...wav.subarray(offset, offset + 8192));
      return {
        peak,
        windows,
        preAnnouncerDifference,
        postAnnouncerLevel,
        releasedAt,
        closingRms: rms(5.8, 6.3),
        endRms: rms(duration - 0.1, duration),
        duration,
        wav: btoa(binary),
      };
    },
    { code: bundle, mp3: announcer },
  );
  expect(result.peak).toBeLessThan(0.98);
  expect(result.windows.every((rms) => rms > 0.001)).toBe(true);
  expect(result.preAnnouncerDifference).toBeLessThan(0.000001);
  if (result.releasedAt + 0.15 < 6.5)
    expect(result.postAnnouncerLevel).toBeGreaterThan(0.9);
  expect(result.closingRms).toBeGreaterThan(0.005);
  expect(result.endRms).toBeLessThan(0.0001);
  const preview = Buffer.from(result.wav, 'base64');
  writeFileSync('test-results/first-blood-complete.wav', preview);
  await testInfo.attach('first-blood-complete.wav', {
    body: preview,
    contentType: 'audio/wav',
  });
});

test('stopping the real mix silences all future effects and the closing announcer', async ({
  page,
}) => {
  const remainingRms = await page.evaluate(
    async ({ code, mp3 }) => {
      const url = URL.createObjectURL(
        new Blob([code], { type: 'text/javascript' }),
      );
      const { playFirstBloodScore } = await import(url);
      URL.revokeObjectURL(url);
      const context = new OfflineAudioContext(2, 44100 * 10, 44100);
      const bytes = Uint8Array.from(atob(mp3), (value) => value.charCodeAt(0));
      const clip = await context.decodeAudioData(bytes.buffer);
      const sound = playFirstBloodScore(context, {
        volume: 1,
        announcer: Promise.resolve(clip),
      });
      const pause = context.suspend(1.85);
      const rendering = context.startRendering();
      await pause;
      sound.stop();
      await context.resume();
      const output = await rendering;
      await sound.finished;
      let sum = 0;
      const data = output.getChannelData(0);
      for (let index = 2 * 44100; index < data.length; index += 1)
        sum += data[index] ** 2;
      return Math.sqrt(sum / (data.length - 2 * 44100));
    },
    { code: bundle, mp3: announcer },
  );
  expect(remainingRms).toBeLessThan(0.000001);
});
