// @ts-expect-error Bun exposes its test module at runtime without bundled TypeScript types.
import { describe, expect, test } from "bun:test";

import { PlatformAPIError } from "./api-utils";
import { stopMatchTolerantOfTimeouts } from "./match-stop";

type Status = { state: string };

function fakeClock() {
  let current = 0;
  return {
    now: () => current,
    sleep: async (ms: number) => {
      current += ms;
    },
  };
}

describe("stopMatchTolerantOfTimeouts", () => {
  test("returns the stop result when the call succeeds", async () => {
    const result = await stopMatchTolerantOfTimeouts<Status>(
      async () => ({ state: "finished" }),
      async () => {
        throw new Error("status should not be read");
      },
    );
    expect(result.state).toBe("finished");
  });

  test("waits for the match to finish after a transport timeout", async () => {
    const states = ["running", "running", "finished"];
    const clock = fakeClock();
    const result = await stopMatchTolerantOfTimeouts<Status>(
      async () => {
        throw new TypeError("fetch failed: UND_ERR_HEADERS_TIMEOUT");
      },
      async () => ({ state: states.shift() ?? "finished" }),
      { ...clock, pollIntervalMs: 1_000, maxWaitMs: 60_000 },
    );
    expect(result.state).toBe("finished");
  });

  test("rethrows an upstream error response without polling", async () => {
    let polled = false;
    const failure = new PlatformAPIError("final flag retention verification failed", 500);
    await expect(
      stopMatchTolerantOfTimeouts<Status>(
        async () => {
          throw failure;
        },
        async () => {
          polled = true;
          return { state: "finished" };
        },
      ),
    ).rejects.toBe(failure);
    expect(polled).toBe(false);
  });

  test("gives up with the original error if the match never finishes", async () => {
    const timeout = new TypeError("fetch failed");
    const clock = fakeClock();
    await expect(
      stopMatchTolerantOfTimeouts<Status>(
        async () => {
          throw timeout;
        },
        async () => ({ state: "running" }),
        { ...clock, pollIntervalMs: 1_000, maxWaitMs: 5_000 },
      ),
    ).rejects.toBe(timeout);
  });
});
