// @ts-expect-error Bun exposes its test module at runtime without bundled TypeScript types.
import { expect, test } from "bun:test";

import {
  proxyAdminRealtimeStream,
  proxyPublicRealtimeStream,
} from "./realtime-proxy";

test("realtime proxy preserves upstream throttling metadata", async () => {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async () =>
    new Response(JSON.stringify({ detail: "slow down" }), {
      status: 429,
      headers: {
        "Content-Type": "application/problem+json",
        "Retry-After": "9",
      },
    });

  let response: Response;
  try {
    response = await proxyPublicRealtimeStream(
      "/public/v1/scoreboard/stream",
      "stream unavailable",
    );
  } finally {
    globalThis.fetch = originalFetch;
  }

  expect(response.status).toBe(429);
  expect(response.headers.get("Content-Type")).toBe("application/problem+json");
  expect(response.headers.get("Retry-After")).toBe("9");
});

test("realtime proxy forwards only the original sanitized client address", async () => {
  const originalFetch = globalThis.fetch;
  let forwardedFor: string | null = null;
  globalThis.fetch = async (_input, init) => {
    forwardedFor = new Headers(init?.headers).get("x-forwarded-for");
    return new Response("data: []\n\n", {
      status: 200,
      headers: { "Content-Type": "text/event-stream" },
    });
  };

  try {
    await proxyPublicRealtimeStream(
      "/public/v1/scoreboard/stream",
      "stream unavailable",
      new Request("https://example.test", {
        headers: { "X-Forwarded-For": "198.51.100.7, 10.0.0.4" },
      }),
    );
  } finally {
    globalThis.fetch = originalFetch;
  }

  expect(forwardedFor).toBe("198.51.100.7");
});

test("admin realtime proxy prefers REALTIME_ADMIN_TOKEN over ADMIN_API_TOKEN", async () => {
  const originalFetch = globalThis.fetch;
  const previousRealtime = process.env.REALTIME_ADMIN_TOKEN;
  const previousAdmin = process.env.ADMIN_API_TOKEN;
  process.env.REALTIME_ADMIN_TOKEN = "realtime-stream-token";
  process.env.ADMIN_API_TOKEN = "admin-api-token";

  let authorization: string | null = null;
  globalThis.fetch = async (_input, init) => {
    authorization = new Headers(init?.headers).get("authorization");
    return new Response("data: []\n\n", {
      status: 200,
      headers: { "Content-Type": "text/event-stream" },
    });
  };

  try {
    await proxyAdminRealtimeStream(
      "/admin/v1/game/status/stream",
      "admin stream unavailable",
    );
  } finally {
    globalThis.fetch = originalFetch;
    if (previousRealtime === undefined) {
      delete process.env.REALTIME_ADMIN_TOKEN;
    } else {
      process.env.REALTIME_ADMIN_TOKEN = previousRealtime;
    }
    if (previousAdmin === undefined) {
      delete process.env.ADMIN_API_TOKEN;
    } else {
      process.env.ADMIN_API_TOKEN = previousAdmin;
    }
  }

  expect(authorization).toBe("Bearer realtime-stream-token");
});

test("admin realtime follows the request lifetime and caps each subscription", async () => {
  const originalFetch = globalThis.fetch;
  const originalTimeout = AbortSignal.timeout;
  const previousToken = process.env.REALTIME_ADMIN_TOKEN;
  const request = new AbortController();
  const expiry = new AbortController();
  const upstreamSignals: Array<AbortSignal | null | undefined> = [];
  process.env.REALTIME_ADMIN_TOKEN = "realtime-stream-token";
  AbortSignal.timeout = (milliseconds) => {
    expect(milliseconds).toBe(60_000);
    return expiry.signal;
  };
  globalThis.fetch = async (_input, init) => {
    upstreamSignals.push(init?.signal);
    return new Response("data: []\n\n");
  };

  try {
    for (let count = 0; count < 2; count += 1) {
      const browserRequest = new Request("https://example.test", {
        signal: count === 0 ? request.signal : undefined,
      });
      const response = await proxyAdminRealtimeStream(
        "/admin/v1/game/status/stream", "admin stream unavailable", browserRequest,
      );
      expect(await response.text()).toBe("data: []\n\n");
    }
    expect(upstreamSignals.map((signal) => signal?.aborted)).toEqual([false, false]);
    request.abort();
    expect(upstreamSignals.map((signal) => signal?.aborted)).toEqual([true, false]);
    expiry.abort();
    expect(upstreamSignals.map((signal) => signal?.aborted)).toEqual([true, true]);
    expect(upstreamSignals[1]?.reason.name).toBe("AbortError");
  } finally {
    globalThis.fetch = originalFetch;
    AbortSignal.timeout = originalTimeout;
    if (previousToken === undefined) {
      delete process.env.REALTIME_ADMIN_TOKEN;
    } else {
      process.env.REALTIME_ADMIN_TOKEN = previousToken;
    }
  }
});
