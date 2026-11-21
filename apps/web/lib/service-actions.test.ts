// @ts-expect-error Bun exposes its test module at runtime without bundled TypeScript types.
import { describe, expect, test } from "bun:test";

import {
  cooldownMessage,
  factoryResetSuccessMessage,
  parseRetryAfterSeconds,
  remainingCooldownSeconds,
  restartSuccessMessage,
  slowActionMessage,
} from "./service-actions";

describe("service action cooldowns", () => {
  test("parses Retry-After delta seconds and HTTP dates", () => {
    expect(parseRetryAfterSeconds("42")).toBe(42);
    expect(parseRetryAfterSeconds(" 7 ")).toBe(7);
    expect(parseRetryAfterSeconds("0")).toBeNull();
    expect(parseRetryAfterSeconds(undefined)).toBeNull();
    expect(parseRetryAfterSeconds("soon")).toBeNull();
    const now = Date.parse("2026-11-15T10:00:00Z");
    expect(parseRetryAfterSeconds("Sun, 15 Nov 2026 10:00:30 GMT", now)).toBe(30);
  });

  test("rounds remaining cooldown up to whole seconds", () => {
    expect(remainingCooldownSeconds(undefined, 1000)).toBe(0);
    expect(remainingCooldownSeconds(1000, 1000)).toBe(0);
    expect(remainingCooldownSeconds(1001, 1000)).toBe(1);
    expect(remainingCooldownSeconds(91_000, 1000)).toBe(90);
  });

  test("names the action and the wait", () => {
    expect(cooldownMessage("reset", 42)).toBe(
      "Factory reset is on cooldown for this service. Wait 42 seconds before trying again.",
    );
    expect(cooldownMessage("restart", 1)).toBe(
      "Restart is on cooldown for this service. Wait 1 second before trying again.",
    );
  });
});

describe("service action results", () => {
  test("restart and factory reset outcomes are distinguishable", () => {
    expect(restartSuccessMessage()).toContain("patches kept");
    const reset = factoryResetSuccessMessage({ unlock_preserved: true });
    expect(reset).toContain("baseline image");
    expect(reset).toContain("patches were discarded");
    expect(reset).toContain("SSH unlock is preserved.");
    expect(reset).not.toContain("defense penalty");
  });

  test("factory reset reports the flag penalty", () => {
    expect(factoryResetSuccessMessage({ penalty_points: 10 })).toContain(
      "10-point defense penalty",
    );
  });
});

test("a slow action names what is still running", () => {
  expect(slowActionMessage("restart:svc-1")).toContain("Restart is still running");
  expect(slowActionMessage("reset:svc-1")).toContain("Factory reset is still running");
  expect(slowActionMessage("ssh:svc-1")).toContain("root access");
  expect(slowActionMessage("attacks:refresh")).toBeNull();
});
