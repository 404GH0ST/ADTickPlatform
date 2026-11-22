// @ts-expect-error Bun exposes its test module at runtime without bundled TypeScript types.
import { describe, expect, test } from "bun:test";

import {
  formatIndonesianDate,
  formatTimestampsInText,
  formatWibTime,
  fromWibInputValue,
  toWibInputValue,
} from "./date-format";

describe("WIB date and time fields", () => {
  test("reads a typed time as WIB", () => {
    expect(fromWibInputValue("2026-12-01T10:00")).toBe("2026-12-01T03:00:00.000Z");
  });

  test("shows an instant as WIB and survives a round trip across midnight", () => {
    expect(toWibInputValue("2026-12-01T17:30:00Z")).toBe("2026-12-02T00:30");
    expect(fromWibInputValue(toWibInputValue("2026-12-01T17:30:00Z"))).toBe("2026-12-01T17:30:00.000Z");
  });

  test("rejects incomplete and impossible values", () => {
    for (const value of ["", "T", "2026-12-01T", "T10:00", "2026-02-31T10:00", "2026-12-01T25:00"]) {
      expect(fromWibInputValue(value)).toBeUndefined();
    }
    expect(toWibInputValue("not a date")).toBe("");
    expect(toWibInputValue(undefined)).toBe("");
  });

  test("formats midnight as 00, not 24", () => {
    expect(formatIndonesianDate("2026-12-01T17:00:00Z")).toBe("2026-12-02 00:00:00 WIB");
    expect(formatWibTime("2026-12-01T17:00:00Z")).toBe("00:00:00 WIB");
  });
});

describe("formatTimestampsInText", () => {
  test("rewrites an ISO timestamp inside a sentence to WIB", () => {
    expect(formatTimestampsInText("Expected the next run around 2026-03-20T10:18:00Z with a 30s interval.")).toBe(
      "Expected the next run around 2026-03-20 17:18:00 WIB with a 30s interval.",
    );
  });

  test("handles fractional seconds, offsets and several timestamps", () => {
    expect(formatTimestampsInText("from 2026-03-20T10:18:00.250Z to 2026-03-20T18:00:00+07:00")).toBe(
      "from 2026-03-20 17:18:00 WIB to 2026-03-20 18:00:00 WIB",
    );
  });

  test("leaves text without a full timestamp alone", () => {
    expect(formatTimestampsInText("tick 2026-03-20 on port 10:18")).toBe("tick 2026-03-20 on port 10:18");
  });
});
