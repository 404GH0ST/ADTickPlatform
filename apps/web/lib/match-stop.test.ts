// @ts-expect-error Bun exposes its test module at runtime without bundled TypeScript types.
import { describe, expect, test } from "bun:test";

import { PlatformAPIError } from "./api-utils";
import { isConnectFailure, stopMatchTolerantOfTimeouts } from "./match-stop";

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

  test("rethrows at once when the gateway refuses the connection", async () => {
    let polled = false;
    const refused = new TypeError("fetch failed", {
      cause: Object.assign(new Error("connect ECONNREFUSED 127.0.0.1:8080"), {
        code: "ECONNREFUSED",
      }),
    });
    await expect(
      stopMatchTolerantOfTimeouts<Status>(
        async () => {
          throw refused;
        },
        async () => {
          polled = true;
          return { state: "running" };
        },
        fakeClock(),
      ),
    ).rejects.toBe(refused);
    expect(polled).toBe(false);
  });

  test("detects connect failures wrapped in an AggregateError", async () => {
    const wrapped = new TypeError("fetch failed", {
      cause: new AggregateError([
        Object.assign(new Error("connect ECONNREFUSED ::1:8080"), { code: "ECONNREFUSED" }),
        Object.assign(new Error("connect ECONNREFUSED 127.0.0.1:8080"), { code: "ECONNREFUSED" }),
      ]),
    });
    expect(isConnectFailure(wrapped)).toBe(true);
  });

  test("keeps polling when the request was sent but no response arrived", async () => {
    const headersTimeout = new TypeError("fetch failed", {
      cause: Object.assign(new Error("Headers Timeout Error"), {
        code: "UND_ERR_HEADERS_TIMEOUT",
      }),
    });
    expect(isConnectFailure(headersTimeout)).toBe(false);
    const states = ["running", "finished"];
    const result = await stopMatchTolerantOfTimeouts<Status>(
      async () => {
        throw headersTimeout;
      },
      async () => ({ state: states.shift() ?? "finished" }),
      { ...fakeClock(), pollIntervalMs: 1_000, maxWaitMs: 60_000 },
    );
    expect(result.state).toBe("finished");
  });
});
