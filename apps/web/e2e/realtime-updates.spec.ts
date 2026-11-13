import { expect, type Page } from "@playwright/test";
import { mockApiBaseUrl, realtimeTest as test } from "./test-utils";

async function installMockAttackSfx(page: Page) {
  await page.addInitScript(() => {
    window.localStorage.removeItem('ad-platform-theme');
    type Voice = { kind: string; start: number; stop: number; heard: boolean; ended: boolean };
    type AttackSfxWindow = Window & {
      __attackSfxStarts?: number[];
      __attackSfxKinds?: string[];
      __attackSfxVoices?: Voice[];
      AudioContext?: typeof AudioContext;
      webkitAudioContext?: typeof AudioContext;
    };
    const testWindow = window as AttackSfxWindow;
    testWindow.__attackSfxStarts = [];
    testWindow.__attackSfxKinds = [];
    testWindow.__attackSfxVoices = [];
    class MockAudioParam {
      value = 0;
      setValueAtTime(value: number) { this.value = value; }
      linearRampToValueAtTime(value: number) { this.value = value; }
      exponentialRampToValueAtTime(value: number) { this.value = value; }
      setTargetAtTime(value: number) { this.value = value; }
      cancelScheduledValues() {}
    }
    class MockNode {
      connect() {}
      disconnect() {}
    }
    class MockSource extends MockNode {
      onended: (() => void) | null = null;
      type = 'sine';
      frequency = new MockAudioParam();
      buffer: { duration: number; decoded?: boolean } | null = null;
      loop = false;
      private voice?: Voice;
      private endTimer?: number;
      private heardTimer?: number;
      constructor(private context: MockAudioContext, private sample = false) { super(); }
      start(time: number) {
        const kind = this.sample ? this.buffer?.decoded ? 'sample' : 'noise' : this.type;
        this.voice = { kind, start: time, stop: Infinity, heard: false, ended: false };
        testWindow.__attackSfxVoices?.push(this.voice);
        testWindow.__attackSfxStarts?.push(time);
        testWindow.__attackSfxKinds?.push(kind);
        this.heardTimer = window.setTimeout(() => { if (this.voice) this.voice.heard = true; }, Math.max(0, time - this.context.currentTime) * 1000);
      }
      stop(time = this.context.currentTime) {
        if (this.voice) {
          this.voice.stop = time;
          if (time < this.voice.start) window.clearTimeout(this.heardTimer);
        }
        window.clearTimeout(this.endTimer);
        this.endTimer = window.setTimeout(() => {
          if (this.voice) this.voice.ended = true;
          this.onended?.();
        }, Math.max(0, time - this.context.currentTime) * 1000);
      }
    }
    class MockGain extends MockNode { gain = new MockAudioParam(); }
    class MockAudioContext {
      private origin = performance.now();
      get currentTime() { return 1 + (performance.now() - this.origin) / 1000; }
      sampleRate = 24000;
      destination = new MockNode();
      state: AudioContextState = 'running';
      createOscillator() { return new MockSource(this); }
      createBufferSource() { return new MockSource(this, true); }
      createBuffer(_channels: number, length: number, sampleRate: number) {
        const data = new Float32Array(length);
        return { duration: length / sampleRate, getChannelData: () => data };
      }
      decodeAudioData() { return Promise.resolve({ duration: 2.624, decoded: true }); }
      createGain() { return new MockGain(); }
      createStereoPanner() { return Object.assign(new MockNode(), { pan: new MockAudioParam() }); }
      createBiquadFilter() { return Object.assign(new MockNode(), { type: 'bandpass', frequency: new MockAudioParam(), Q: new MockAudioParam() }); }
      createDynamicsCompressor() {
        return Object.assign(new MockNode(), Object.fromEntries(['threshold', 'knee', 'ratio', 'attack', 'release'].map((key) => [key, new MockAudioParam()])));
      }
      resume() { return Promise.resolve(); }
      close() { this.state = 'closed'; return Promise.resolve(); }
    }
    testWindow.AudioContext = MockAudioContext as unknown as typeof AudioContext;
    testWindow.webkitAudioContext = MockAudioContext as unknown as typeof AudioContext;
  });
}

async function scoreVoices(page: Page) {
  return page.evaluate(() => (window as Window & {
    __attackSfxVoices?: Array<{ kind: string; start: number; stop: number; heard: boolean; ended: boolean }>;
  }).__attackSfxVoices ?? []);
}

test("participant attacks applies realtime attack updates to the live slice", async ({
  page,
  request,
}) => {
  await page.goto("/attacks");
  await expect(page.getByRole("cell", { name: "#13" })).toHaveCount(0);

  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "realtime-updates" },
  });

  await expect(page.getByRole("cell", { name: "#13" })).toBeVisible();
});

test("participant attack map route applies realtime updates to the loaded attack count", async ({
  page,
}) => {
  await page.goto("/attacks");

  await expect(page.getByText("Showing 1-13 of 13")).toBeVisible();
});

test("participant attack map plays SFX for new realtime attacks after user activation", async ({
  page,
  request,
}) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "default" },
  });
  await page.addInitScript(() => {
    window.localStorage.setItem("ad-platform-attack-sfx-enabled", "on");
  });
  await installMockAttackSfx(page);
  await page.goto("/attacks");
  await expect(page.getByText("Showing 1-12 of 12")).toBeVisible();
  await page.mouse.click(24, 24);

  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "realtime-updates" },
  });

  await expect(page.getByText("Showing 1-13 of 13")).toBeVisible();
  await expect
    .poll(() =>
      page.evaluate(() =>
        ((window as Window & { __attackSfxStarts?: number[] }).__attackSfxStarts?.length ?? 0),
      ),
    )
    .toBeGreaterThan(0);
});

test("participant attack map respects muted attack SFX preference", async ({
  page,
  request,
}) => {
  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "default" },
  });
  await page.addInitScript(() => {
    window.localStorage.setItem("ad-platform-attack-sfx-enabled", "off");
  });
  await installMockAttackSfx(page);
  await page.goto("/attacks");
  await expect(page.getByText("Showing 1-12 of 12")).toBeVisible();
  await expect(page.getByRole("button", { name: "Enable attack sound" })).toBeVisible();
  await page.mouse.click(24, 24);

  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "realtime-updates" },
  });

  await expect(page.getByText("Showing 1-13 of 13")).toBeVisible();
  await expect
    .poll(() =>
      page.evaluate(() =>
        ((window as Window & { __attackSfxStarts?: number[] }).__attackSfxStarts?.length ?? 0),
      ),
    )
    .toBe(0);
});

test("organizer game page applies realtime game-status updates to the current tick card", async ({
  page,
}) => {
  await page.goto("/admin/game");
  const currentTickCard = page.getByTestId("current-tick-card").first();

  await expect(currentTickCard).toContainText("Tick #12");

  await expect(currentTickCard).toContainText("Tick #13");
  await expect(currentTickCard).toContainText("tick #13 completed");
});

test("organizer attacks route applies realtime updates to the loaded attack count", async ({
  page,
}) => {
  await page.goto("/admin/attacks");

  await expect(page.getByText("Showing 1-13 of 13")).toBeVisible();
});

for (const muted of [false, true]) {
  test(`first blood has an exclusive route and ${muted ? 'respects mute' : 'plays a fanfare'}`, async ({ page, request }) => {
    await request.post(`${mockApiBaseUrl}/__reset`, { data: { scenario: 'default' } });
    await installMockAttackSfx(page);
    await page.emulateMedia({ colorScheme: muted ? 'dark' : 'light' });
    if (muted) await page.setViewportSize({ width: 390, height: 844 });
    await page.addInitScript((off) => {
      localStorage.setItem('ad-platform-attack-sfx-enabled', off ? 'off' : 'on');
    }, muted);
    await page.goto('/attacks');
    await expect(page.getByText('Showing 1-12 of 12')).toBeVisible();
    await page.mouse.click(24, 24);
    await request.post(`${mockApiBaseUrl}/__reset`, { data: { scenario: 'realtime-first-blood' } });
    await expect(page.getByText('Showing 1-13 of 13')).toBeVisible();
    const globe = page.getByTestId('cyber-attack-map');
    await globe.scrollIntoViewIfNeeded();
    await expect(globe).toHaveAttribute('data-surface-active', 'true');
    await expect(globe).toHaveAttribute('data-first-blood-phase', 'intro');
    await expect(globe.getByTestId('first-blood-intro')).toContainText('First blood');
    await expect(globe.getByTestId('first-blood-intro')).toContainText('College Gamma');
    await expect(globe.locator('[data-first-blood-effect]')).toHaveCount(0);
    const initialVoices = await scoreVoices(page);
    if (muted) expect(initialVoices).toHaveLength(0);
    else {
      expect(initialVoices.length).toBeGreaterThan(10);
      const announcer = initialVoices.filter((voice) => voice.kind === 'sample');
      expect(announcer).toHaveLength(1);
      expect(announcer[0].start - initialVoices[0].start).toBeCloseTo(6.45, 2);
      expect(announcer[0].heard).toBe(false);
    }
    await globe.getByTestId('first-blood-intro').evaluate((element) => Promise.all(
      element.getAnimations({ subtree: true }).map((animation) => animation.finished.catch(() => {})),
    ));
    await globe.screenshot({ path: `test-results/first-blood-intro-${muted ? 'dark' : 'light'}.png` });
    await expect(globe).toHaveAttribute('data-first-blood-phase', 'attack');
    await expect(globe.getByTestId('first-blood-intro')).toHaveCount(0);
    await expect(globe.getByTestId('first-blood-announcement')).toContainText('First blood · college-http');
    await expect(globe).toHaveAttribute('data-featured-attack-id', 'atk-13');
    await expect(globe.locator('[data-first-blood="true"]')).toHaveCount(1);
    await expect(globe.locator('[data-first-blood-effect]')).toHaveCount(1);
    await expect(globe.locator('[data-first-blood-endpoint="attacker"]')).toHaveCount(1);
    await expect(globe.locator('[data-first-blood-endpoint="victim"]')).toHaveCount(1);
    expect(await scoreVoices(page)).toHaveLength(initialVoices.length);
    const matchup = globe.getByTestId('first-blood-announcement').getByRole('heading');
    await expect(matchup).toBeVisible();
    await expect(matchup).toContainText('College Gamma');
    await expect(matchup).toContainText('College Alpha');
    await globe.screenshot({ path: `test-results/first-blood-${muted ? 'muted' : 'sound'}.png` });
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await expect(globe).toHaveAttribute('data-motion-enabled', 'false');
    await expect(globe.locator('[data-first-blood-effect] [class]')).toHaveCount(0);
    await page.setViewportSize({ width: 390, height: 844 });
    await globe.scrollIntoViewIfNeeded();
    await expect(globe.getByTestId('first-blood-announcement')).toBeVisible();
    await globe.screenshot({ path: `test-results/first-blood-mobile-${muted ? 'dark' : 'light'}.png` });
    // The live moment ends even while the original capture stays featured.
    await expect(globe.getByTestId('first-blood-announcement')).toHaveCount(0, { timeout: 6000 });
    await expect(globe.getByTestId('first-blood-banner')).toBeVisible();
    await expect(globe.getByTestId('first-blood-banner-team')).toHaveText('College Gamma');
    await expect(globe.getByTestId('first-blood-banner-challenge')).toHaveText('college-http');
    if (!muted) await expect.poll(async () => (await scoreVoices(page)).filter((voice) => voice.kind === 'sample' && voice.heard).length).toBe(1);
    await globe.screenshot({ path: `test-results/first-blood-banner-${muted ? 'dark' : 'light'}.png` });
    await expect(globe.getByTestId('first-blood-banner')).toHaveCount(0, { timeout: 6000 });
    await expect(globe.locator('[data-first-blood-effect]')).toHaveCount(0);
    await globe.evaluate((element) => element.dispatchEvent(new CustomEvent('ad-platform:feature-attack', {
      detail: { attackId: 'atk-13', animate: false },
    })));
    await expect(globe.locator('[data-first-blood-effect]')).toHaveCount(0);
    await page.getByRole('button', { name: 'Maximize' }).click();
    await expect(page.getByTestId('cyber-attack-map')).toHaveCount(1);
    await expect(page.getByTestId('attack-map-maximize-dialog')).toBeVisible();
    const expandedGlobe = page.getByTestId('cyber-attack-map');
    await expandedGlobe.evaluate((element) => element.dispatchEvent(new CustomEvent('ad-platform:feature-attack', {
      detail: { attackId: 'atk-13', animate: false },
    })));
    await expect(expandedGlobe.locator('[data-first-blood-effect]')).toHaveCount(0);
    await expect(expandedGlobe.getByTestId('first-blood-announcement')).toHaveCount(0);
    await expect(expandedGlobe.getByTestId('first-blood-banner')).toHaveCount(0);
    await page.getByRole('button', { name: 'Back to page' }).click();
    await request.post(`${mockApiBaseUrl}/__reset`, { data: { scenario: 'realtime-after-first-blood' } });
    await expect(page.getByText('Showing 1-14 of 14')).toBeVisible();
    await expect(page.getByTestId('first-blood-announcement')).toHaveCount(0);
    await expect(page.getByTestId('first-blood-banner')).toHaveCount(0);
    await expect(page.locator('[data-first-blood-effect]')).toHaveCount(0);
    await expect.poll(() => page.evaluate(() =>
      (window as Window & { __attackSfxStarts?: number[] }).__attackSfxStarts?.length ?? 0,
    )).toBe(muted ? 0 : initialVoices.length + 1);
  });
}

test('muting during the first-blood intro silences the following attack', async ({ page, request }) => {
  await request.post(`${mockApiBaseUrl}/__reset`, { data: { scenario: 'default' } });
  await installMockAttackSfx(page);
  await page.addInitScript(() => localStorage.setItem('ad-platform-attack-sfx-enabled', 'on'));
  await page.goto('/attacks');
  await expect(page.getByText('Showing 1-12 of 12')).toBeVisible();
  await page.mouse.click(24, 24);
  await request.post(`${mockApiBaseUrl}/__reset`, { data: { scenario: 'realtime-first-blood' } });
  const globe = page.getByTestId('cyber-attack-map');
  await expect(globe).toHaveAttribute('data-first-blood-phase', 'intro');
  await page.getByRole('button', { name: 'Mute attack sound' }).click();
  await expect(globe).toHaveAttribute('data-first-blood-phase', 'attack');
  const muted = await scoreVoices(page);
  expect(muted.filter((voice) => voice.kind === 'sample').every((voice) => voice.stop < voice.start)).toBe(true);
});

for (const view of [
  { name: 'participant', route: '/attacks', mobile: false },
  { name: 'organizer', route: '/admin/attacks', mobile: false },
  { name: 'mobile participant', route: '/attacks', mobile: true },
]) {
  test(`first blood on an empty ${view.name} globe focuses, charges, strikes and restores`, async ({ page, request }) => {
    await request.post(`${mockApiBaseUrl}/__reset`, { data: { scenario: 'empty-attacks' } });
    await installMockAttackSfx(page);
    await page.addInitScript(() => localStorage.setItem('ad-platform-attack-sfx-enabled', 'on'));
    if (view.mobile) await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(view.route);
    const globe = page.getByTestId('cyber-attack-map');
    await expect(globe).toBeVisible();
    await globe.scrollIntoViewIfNeeded();
    await expect(globe).toHaveAttribute('data-route-count', '0');
    await expect(globe.getByTestId('attack-map-waiting')).toBeVisible();
    // Prove the exact same globe survives the empty-to-first-arrival transition.
    await globe.evaluate((element) => {
      element.setAttribute('data-mount-proof', 'waiting');
      const observer = new MutationObserver(() => {
        const banner = element.querySelector('[data-testid="first-blood-banner"]');
        if (banner && !element.hasAttribute('data-test-banner-start')) {
          element.setAttribute('data-test-banner-start', String(performance.now()));
        } else if (!banner && element.hasAttribute('data-test-banner-start')) {
          element.setAttribute('data-test-banner-duration', String(performance.now() - Number(element.getAttribute('data-test-banner-start'))));
          observer.disconnect();
        }
      });
      observer.observe(element, { childList: true, subtree: true });
    });
    await page.mouse.click(24, 24);
    await request.post(`${mockApiBaseUrl}/__reset`, { data: { scenario: 'realtime-first-blood-empty' } });
    await expect(globe).toHaveAttribute('data-first-blood-stage', 'focus');
    await expect(globe).toHaveAttribute('data-mount-proof', 'waiting');
    await expect(globe).toHaveAttribute('data-first-blood-stage', 'charge');
    await expect(globe).toHaveAttribute('data-featured-attack-id', 'atk-13');
    await expect(globe.locator('[data-first-blood-charge]')).toBeVisible();
    expect(Number(await globe.getAttribute('data-zoom'))).toBeGreaterThan(1.05);
    // Both real team markers remain inside the shot without test-driven camera moves.
    const framing = await globe.evaluate((element) => {
      const bounds = element.getBoundingClientRect();
      return Array.from(element.querySelectorAll('[data-first-blood-endpoint]')).map((node) => {
        const matrix = (node as SVGGraphicsElement).getScreenCTM()!;
        return { x: matrix.e - bounds.left, y: matrix.f - bounds.top, width: bounds.width, height: bounds.height };
      });
    });
    expect(framing).toHaveLength(2);
    for (const point of framing) {
      expect(point.x).toBeGreaterThan(24);
      expect(point.x).toBeLessThan(point.width - 24);
      expect(point.y).toBeGreaterThan(48);
      expect(point.y).toBeLessThan(point.height - 120);
    }
    const croppedLabels = await globe.evaluate((element) => {
      const bounds = element.getBoundingClientRect();
      return Array.from(element.querySelectorAll('[data-attack-team-label]')).filter((label) => {
        const box = label.getBoundingClientRect();
        return box.left < bounds.left || box.right > bounds.right || box.top < bounds.top || box.bottom > bounds.bottom;
      }).map((label) => label.getAttribute('data-attack-team-id'));
    });
    expect(croppedLabels).toEqual([]);
    await globe.screenshot({ path: `test-results/first-blood-charge-${view.name.replaceAll(' ', '-')}.png` });
    await expect(globe).toHaveAttribute('data-first-blood-stage', 'flight');
    await expect(globe.locator('[data-first-blood-strike]')).toBeVisible();
    await expect(globe).toHaveAttribute('data-first-blood-stage', 'impact');
    await expect(globe.locator('[data-first-blood-shard]')).toHaveCount(12);
    await expect(globe.getByTestId('first-blood-announcement')).toContainText('Flag captured');
    await globe.screenshot({ path: `test-results/first-blood-impact-${view.name.replaceAll(' ', '-')}.png` });
    const voices = await scoreVoices(page);
    const announcer = voices.find((voice) => voice.kind === 'sample')!;
    expect(announcer.heard).toBe(false);
    expect(announcer.start - voices[0].start).toBeCloseTo(6.45, 2);
    await expect(globe).toHaveAttribute('data-first-blood-stage', 'recover');
    await expect(globe).not.toHaveAttribute('data-first-blood-stage');
    await expect(globe).toHaveAttribute('data-zoom', '1.000');
    const banner = globe.getByTestId('first-blood-banner');
    await expect(banner).toBeVisible();
    await expect(banner.getByTestId('first-blood-banner-team')).toHaveText('College Gamma');
    await expect(banner.getByTestId('first-blood-banner-challenge')).toHaveText('college-http');
    await expect.poll(async () => (await scoreVoices(page)).find((voice) => voice.kind === 'sample')?.heard).toBe(true);
    await globe.screenshot({ path: `test-results/first-blood-closing-${view.name.replaceAll(' ', '-')}.png` });
    const layout = await banner.evaluate((element) => {
      const bounds = element.closest('[data-testid="cyber-attack-map"]')!.getBoundingClientRect();
      const panel = element.getBoundingClientRect();
      return panel.left >= bounds.left && panel.right <= bounds.right && panel.top >= bounds.top && panel.bottom <= bounds.bottom && element.scrollWidth <= element.clientWidth + 1;
    });
    expect(layout).toBe(true);
    await expect.poll(async () => (await scoreVoices(page)).find((voice) => voice.kind === 'sample')?.ended).toBe(true);
    await expect(banner).toBeVisible();
    await expect(banner).toHaveCount(0, { timeout: 6000 });
    expect(Number(await globe.getAttribute('data-test-banner-duration'))).toBeGreaterThanOrEqual(4000);
    await expect(globe.locator('[data-first-blood-effect]')).toHaveCount(0);
  });
}

test('reduced motion keeps first blood legible without zoom, shake or fragments', async ({ page, request }) => {
  await request.post(`${mockApiBaseUrl}/__reset`, { data: { scenario: 'empty-attacks' } });
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.goto('/attacks');
  const globe = page.getByTestId('cyber-attack-map');
  await globe.scrollIntoViewIfNeeded();
  await request.post(`${mockApiBaseUrl}/__reset`, { data: { scenario: 'realtime-first-blood-empty' } });
  await expect(globe).toHaveAttribute('data-first-blood-stage', 'charge');
  await expect(globe).toHaveAttribute('data-zoom', '1.000');
  await expect(globe.locator('[data-first-blood-charge] [class]')).toHaveCount(0);
  await expect(globe).toHaveAttribute('data-first-blood-stage', 'impact');
  await expect(globe.locator('[data-first-blood-shard]')).toHaveCount(0);
  await expect(globe.locator('[data-first-blood-impact] [class]')).toHaveCount(0);
  await expect(globe.getByTestId('first-blood-announcement')).toContainText('Flag captured');
  const banner = globe.getByTestId('first-blood-banner');
  await expect(banner).toBeVisible();
  expect(await banner.evaluate((element) => element.getAnimations({ subtree: true }).length)).toBe(0);
  await expect(banner).toHaveCount(0, { timeout: 6000 });
});

test('first blood on two challenges queues each team sequence and its sound once', async ({ page, request }) => {
  await request.post(`${mockApiBaseUrl}/__reset`, { data: { scenario: 'empty-attacks' } });
  await installMockAttackSfx(page);
  await page.addInitScript(() => localStorage.setItem('ad-platform-attack-sfx-enabled', 'on'));
  await page.goto('/attacks');
  const globe = page.getByTestId('cyber-attack-map');
  await globe.scrollIntoViewIfNeeded();
  await page.mouse.click(24, 24);
  await request.post(`${mockApiBaseUrl}/__reset`, { data: { scenario: 'realtime-first-blood-burst' } });
  await expect(globe).toHaveAttribute('data-featured-attack-id', 'atk-13');
  await expect(globe).toHaveAttribute('data-first-blood-stage', 'impact', { timeout: 6000 });
  await expect(globe.getByTestId('first-blood-announcement')).toContainText('college-http');
  await expect(globe.locator('[data-first-blood-effect]')).toHaveCount(1);
  await expect(globe.getByTestId('first-blood-banner')).toBeVisible();
  await expect(globe.getByTestId('first-blood-banner')).toContainText('college-http');
  await expect(globe).toHaveAttribute('data-first-blood-stage', 'charge', { timeout: 10000 });
  await expect(globe).toHaveAttribute('data-featured-attack-id', 'atk-14');
  await expect(globe.getByTestId('first-blood-intro')).toContainText('college-vault');
  await expect(globe).toHaveAttribute('data-first-blood-stage', 'impact');
  await expect(globe.locator('[data-first-blood-effect]')).toHaveCount(1);
  const announcers = (await scoreVoices(page)).filter((voice) => voice.kind === 'sample');
  expect(announcers).toHaveLength(2);
  expect(announcers[0].heard).toBe(true);
  expect(announcers[0].stop).toBeLessThanOrEqual(announcers[1].start - 6.45 + 0.05);
  await expect(globe).not.toHaveAttribute('data-first-blood-stage');
  await expect(globe.getByTestId('first-blood-banner')).toContainText('college-vault');
});

test('manual camera control interrupts first blood and cancels the pending strike sound', async ({ page, request }) => {
  await request.post(`${mockApiBaseUrl}/__reset`, { data: { scenario: 'empty-attacks' } });
  await installMockAttackSfx(page);
  await page.addInitScript(() => localStorage.setItem('ad-platform-attack-sfx-enabled', 'on'));
  await page.goto('/attacks');
  const globe = page.getByTestId('cyber-attack-map');
  await globe.scrollIntoViewIfNeeded();
  await page.mouse.click(24, 24);
  await request.post(`${mockApiBaseUrl}/__reset`, { data: { scenario: 'realtime-first-blood-empty' } });
  await expect(globe).toHaveAttribute('data-first-blood-stage', 'charge');
  await globe.getByRole('button', { name: 'Zoom in' }).click();
  await expect(globe).not.toHaveAttribute('data-first-blood-stage');
  await expect(globe).toHaveAttribute('data-zoom', '1.350');
  // Cross the canceled launch cue and ensure it stays canceled.
  await page.waitForTimeout(2200);
  const interrupted = await scoreVoices(page);
  expect(interrupted.filter((voice) => voice.kind === 'sample').every((voice) => !voice.heard && voice.stop < voice.start)).toBe(true);
  await expect(globe.locator('[data-first-blood-effect]')).toHaveCount(0);
});


test('first blood closing banner keeps long team and challenge names readable on mobile', async ({ page, request }) => {
  await request.post(`${mockApiBaseUrl}/__reset`, { data: { scenario: 'empty-attacks' } });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.addInitScript(() => localStorage.setItem('ad-platform-attack-sfx-enabled', 'off'));
  await page.goto('/attacks');
  const globe = page.getByTestId('cyber-attack-map');
  await globe.scrollIntoViewIfNeeded();
  await request.post(`${mockApiBaseUrl}/__reset`, { data: { scenario: 'realtime-first-blood-long-names' } });
  const banner = globe.getByTestId('first-blood-banner');
  await expect(banner).toBeVisible({ timeout: 12000 });
  await expect(banner.getByTestId('first-blood-banner-team')).toHaveText('International Cyber Defense Research and Incident Response Alliance');
  await expect(banner.getByTestId('first-blood-banner-challenge')).toHaveText('HyperconvergedIdentityAndAccessManagementChallengeServiceWithRegionalReplicasAndSecondaryFailover');
  expect(await banner.evaluate((element) => {
    const bounds = element.closest('[data-testid="cyber-attack-map"]')!.getBoundingClientRect();
    const box = element.getBoundingClientRect();
    return box.left >= bounds.left && box.right <= bounds.right && box.top >= bounds.top + 48 && box.bottom <= bounds.bottom - 48 && element.scrollWidth <= element.clientWidth + 1;
  })).toBe(true);
  await globe.screenshot({ path: 'test-results/first-blood-banner-long-mobile.png' });
  await expect(banner).toHaveCount(0, { timeout: 6000 });
});


test('replacing first blood while its closing banner is showing celebrates the new capturer', async ({ page, request }) => {
  await request.post(`${mockApiBaseUrl}/__reset`, { data: { scenario: 'empty-attacks' } });
  await installMockAttackSfx(page);
  await page.addInitScript(() => localStorage.setItem('ad-platform-attack-sfx-enabled', 'on'));
  await page.goto('/attacks');
  const globe = page.getByTestId('cyber-attack-map');
  await globe.scrollIntoViewIfNeeded();
  await page.mouse.click(24, 24);
  await request.post(`${mockApiBaseUrl}/__reset`, { data: { scenario: 'realtime-first-blood-empty' } });
  await expect(globe.getByTestId('first-blood-banner')).toContainText('College Gamma', { timeout: 12000 });
  const replacement = {
    revision: 1,
    items: [{ id: 'atk-14', first_blood: true, attacker: 'College Beta', victim: 'College Alpha', service: 'college-http', tick: 14, verdict: 'first valid submission accepted' }],
    limit: 12, offset: 0, total_count: 1, has_prev: false, has_next: false,
  };
  await page.route('**/api/platform/attacks**', (route) => route.fulfill({ json: replacement }));
  await page.route('**/api/platform/realtime/attacks/stream', (route) => route.fulfill({
    contentType: 'text/event-stream', body: `data: ${JSON.stringify(replacement)}\n\n`,
  }));
  // Drop the current stream so the next frame contains deletion + new capture.
  await request.post(`${mockApiBaseUrl}/__reset`, { data: { scenario: 'empty-attacks' } });
  await expect(globe.getByTestId('first-blood-banner')).toHaveCount(0);
  await expect(globe.getByTestId('first-blood-intro')).toContainText('College Beta');
  await expect(globe).toHaveAttribute('data-featured-attack-id', 'atk-14');
  await expect(globe).toHaveAttribute('data-first-blood-stage', 'impact', { timeout: 6000 });
  await expect(globe.getByTestId('first-blood-banner')).toContainText('College Beta');
  await expect(globe.getByTestId('first-blood-banner-challenge')).toHaveText('college-http');
  expect((await scoreVoices(page)).filter((voice) => voice.kind === 'sample')).toHaveLength(2);
});
