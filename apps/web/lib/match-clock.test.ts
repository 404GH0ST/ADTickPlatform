// @ts-expect-error Bun exposes its test module at runtime without bundled TypeScript types.
import { expect, test } from "bun:test";

import { headerRefreshDelayMs, nextClockOffset, overdueRefreshDelayMs } from "./match-clock";

test("page refreshes back off during a status outage", () => {
  expect([0, 1, 2, 3, 4, 9].map(headerRefreshDelayMs)).toEqual([15_000, 30_000, 60_000, 120_000, 120_000, 120_000]);
});

test("the first sample sets the clock offset", () => {
  expect(nextClockOffset(null, -97_000)).toBe(-97_000);
});

test("a slower response does not drag the offset down", () => {
  expect(nextClockOffset(1_000, 400)).toBe(1_000);
  expect(nextClockOffset(1_000, 1_200)).toBe(1_200);
});

test("a large jump means the device clock changed", () => {
  expect(nextClockOffset(1_000, -90_000)).toBe(-90_000);
  expect(nextClockOffset(-90_000, 1_000)).toBe(1_000);
});

test("refreshes for a late tick back off to a limit", () => {
  expect([0, 1, 2, 3, 9].map(overdueRefreshDelayMs)).toEqual([3_500, 7_000, 14_000, 15_000, 15_000]);
});
