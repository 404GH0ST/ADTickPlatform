import { expect, type Page } from "@playwright/test";
import { mockApiBaseUrl, realtimeTest as test } from "./test-utils";

async function installMockAttackSfx(page: Page) {
  await page.addInitScript(() => {
    window.localStorage.removeItem("ad-platform-theme");
    type AttackSfxWindow = Window & {
      __attackSfxStarts?: number[];
      AudioContext?: typeof AudioContext;
      webkitAudioContext?: typeof AudioContext;
    };
    const testWindow = window as AttackSfxWindow;
    testWindow.__attackSfxStarts = [];

    class MockAudioParam {
      setValueAtTime() {}
      exponentialRampToValueAtTime() {}
    }

    class MockOscillator {
      frequency = new MockAudioParam();
      type = "sine";
      connect() {}
      start(time: number) {
        testWindow.__attackSfxStarts?.push(time);
      }
      stop() {}
    }

    class MockGain {
      gain = new MockAudioParam();
      connect() {}
    }

    class MockAudioContext {
      currentTime = 1;
      destination = {};
      state: AudioContextState = "running";
      createOscillator() {
        return new MockOscillator();
      }
      createGain() {
        return new MockGain();
      }
      resume() {
        return Promise.resolve();
      }
      close() {
        this.state = "closed";
        return Promise.resolve();
      }
    }

    testWindow.AudioContext = MockAudioContext as unknown as typeof AudioContext;
    testWindow.webkitAudioContext = MockAudioContext as unknown as typeof AudioContext;
  });
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

  await expect(page.getByText("Loaded 13 of 13 attack(s)")).toBeVisible();
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
  await expect(page.getByText("Loaded 12 of 12 attack(s)")).toBeVisible();
  await page.mouse.click(24, 24);

  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "realtime-updates" },
  });

  await expect(page.getByText("Loaded 13 of 13 attack(s)")).toBeVisible();
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
  await expect(page.getByText("Loaded 12 of 12 attack(s)")).toBeVisible();
  await expect(page.getByRole("button", { name: "Enable attack sound" })).toBeVisible();
  await page.mouse.click(24, 24);

  await request.post(`${mockApiBaseUrl}/__reset`, {
    data: { scenario: "realtime-updates" },
  });

  await expect(page.getByText("Loaded 13 of 13 attack(s)")).toBeVisible();
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

  await expect(page.getByText("Loaded 13 of 13 attack(s)")).toBeVisible();
});
