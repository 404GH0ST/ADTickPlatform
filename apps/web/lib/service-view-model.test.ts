// @ts-expect-error Bun exposes its test module at runtime without bundled TypeScript types.
import { describe, expect, test } from "bun:test";

import type { ServiceRow } from "./dashboard-types";
import { describeService } from "./service-view-model";

function service(overrides: Partial<ServiceRow> = {}): ServiceRow {
  return {
    id: "svc-1",
    challengeId: 1,
    teamId: 101,
    name: "notes",
    endpoint: "10.0.0.1:8080",
    port: 8080,
    status: "stable",
    checker: "passing",
    hasSourceDownload: true,
    category: "web",
    protocol: "http",
    unlocked: true,
    sshHint: "",
    lastEvent: "",
    resetCooldown: "ready",
    maintenance: false,
    slaStatus: "ok",
    slaPhase: "",
    slaTickId: null,
    slaMessage: "",
    ...overrides,
  };
}

const idle = { notice: null, restartCooldown: 0, resetCooldown: 0 };

describe("describeService", () => {
  test("unlocked idle service enables every action", () => {
    const view = describeService(service(), { feedback: idle, pendingAction: null });
    expect(view.ssh).toEqual({ label: "SSH Access", disabled: false, pending: false });
    expect(view.restart.disabled).toBe(false);
    expect(view.reset.disabled).toBe(false);
    expect(view.lock).toBeNull();
    expect(view.unlockHint).toBeNull();
    expect(view.sourceHref).toBe("/api/platform/challenges/1/source");
  });

  test("locked SSH stays disabled and shows the unlock hint", () => {
    const view = describeService(service({ unlocked: false }), { feedback: idle, pendingAction: null });
    expect(view.ssh.label).toBe("SSH Locked");
    expect(view.ssh.disabled).toBe(true);
    expect(view.unlockHint).not.toBeNull();
  });

  test("a pending action disables all actions and marks the right one pending", () => {
    const view = describeService(service(), { feedback: idle, pendingAction: "reset:svc-1" });
    expect(view.reset.pending).toBe(true);
    expect(view.restart.pending).toBe(false);
    expect([view.ssh.disabled, view.restart.disabled, view.reset.disabled]).toEqual([true, true, true]);
  });

  test("maintenance locks actions, hides source and explains why", () => {
    const view = describeService(service({ maintenance: true }), {
      feedback: idle,
      pendingAction: null,
      matchState: "paused",
    });
    expect(view.lock?.reason).toBe("match_paused");
    expect(view.lock?.label).toBe("Match paused");
    expect(view.restart.disabled).toBe(true);
    expect(view.sourceHref).toBeNull();
  });

  test("live cooldowns override the API label", () => {
    const view = describeService(service(), {
      feedback: { notice: null, restartCooldown: 0, resetCooldown: 42 },
      pendingAction: null,
    });
    expect(view.reset.cooldownSeconds).toBe(42);
    expect(view.cooldownLabel).toBe("factory reset cooldown: 42s");
  });
});
