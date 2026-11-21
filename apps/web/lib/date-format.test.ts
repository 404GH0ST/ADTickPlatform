// @ts-expect-error Bun exposes its test module at runtime without bundled TypeScript types.
import { describe, expect, test } from "bun:test";

import { formatTimestampsInText } from "./date-format";

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
