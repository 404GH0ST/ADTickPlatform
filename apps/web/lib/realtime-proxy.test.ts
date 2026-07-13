// @ts-expect-error Bun exposes its test module at runtime without bundled TypeScript types.
import { expect, test } from "bun:test";

import { proxyPublicRealtimeStream } from "./realtime-proxy";

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
