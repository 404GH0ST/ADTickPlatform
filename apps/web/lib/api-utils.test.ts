// @ts-expect-error Bun exposes its test module at runtime without bundled TypeScript types.
import { describe, expect, test } from "bun:test";

import {
  authenticatedFetch,
  browserRequestErrorMessage,
  buildQueryString,
  firstForwardedAddress,
  parseTeamBody,
  PlatformAPIError,
  processApiResponse,
  upstreamErrorDetails,
} from "./api-utils";

describe("upstreamErrorDetails", () => {
  test("preserves upstream status and retry metadata", () => {
    const details = upstreamErrorDetails(
      new PlatformAPIError("slow down", 429, "7"),
      "request failed",
    );

    expect(details).toEqual({
      detail: "slow down",
      retryAfter: "7",
      status: 429,
    });
  });

  test("maps transport failures to bad gateway", () => {
    expect(upstreamErrorDetails(new Error("connection refused"), "request failed")).toEqual({
      detail: "connection refused",
      status: 502,
    });
    expect(upstreamErrorDetails(null, "request failed")).toEqual({
      detail: "request failed",
      status: 502,
    });
  });
});

describe("network failures", () => {
  test("a refused connection never exposes the raw fetch error", () => {
    expect(upstreamErrorDetails(new TypeError("fetch failed"), "request failed")).toEqual({
      detail: "The platform API can't be reached. Try again in a moment.",
      status: 502,
    });
  });

  test("a timed-out request reports a gateway timeout", () => {
    const timeout = new DOMException("The operation timed out.", "TimeoutError");
    expect(upstreamErrorDetails(timeout, "request failed")).toEqual({
      detail: "The platform API didn't respond in time. Try again in a moment.",
      status: 504,
    });
  });

  test("browser messages replace network errors and keep API errors", () => {
    expect(browserRequestErrorMessage(new TypeError("Failed to fetch"), "fallback")).toBe(
      "Can't reach the platform. Check your connection and try again.",
    );
    expect(browserRequestErrorMessage(new Error("invalid credentials"), "fallback")).toBe("invalid credentials");
    expect(browserRequestErrorMessage(null, "fallback")).toBe("fallback");
  });
});

describe("firstForwardedAddress", () => {
  test("selects the original address from a proxy chain", () => {
    expect(firstForwardedAddress("203.0.113.5, 10.0.0.8")).toBe("203.0.113.5");
  });

  test("ignores empty forwarding headers", () => {
    expect(firstForwardedAddress(" , ")).toBeUndefined();
    expect(firstForwardedAddress(null)).toBeUndefined();
  });
});

describe("authenticatedFetch", () => {
  test("preserves Headers instances while adding authorization", async () => {
    const originalFetch = globalThis.fetch;
    let capturedHeaders = new Headers();
    globalThis.fetch = async (_input, init) => {
      capturedHeaders = new Headers(init?.headers);
      return Response.json({ ok: true });
    };

    try {
      await authenticatedFetch<{ ok: boolean }>(
        "http://api-gateway:8080",
        "/api/v2/challenges",
        "participant-token",
        {
          headers: new Headers({ "X-Forwarded-For": "203.0.113.5" }),
        },
      );
    } finally {
      globalThis.fetch = originalFetch;
    }

    expect(capturedHeaders.get("Authorization")).toBe(
      "Bearer participant-token",
    );
    expect(capturedHeaders.get("X-Forwarded-For")).toBe("203.0.113.5");
  });

  test("preserves a caller content type on a request without a bearer token", async () => {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = async (_input, init) => {
      expect(new Headers(init?.headers).get("content-type")).toBe("text/plain");
      expect(new Headers(init?.headers).has("authorization")).toBe(false);
      expect(init?.cache).toBe("no-store");
      return Response.json({ ok: true });
    };
    try {
      expect(await authenticatedFetch("http://api-gateway:8080", "/healthz", null, {
        headers: { "Content-Type": "text/plain" },
      })).toEqual({ ok: true });
    } finally {
      globalThis.fetch = originalFetch;
    }
  });
});

test("query strings retain zero and encode filter text", () => {
  expect(buildQueryString({ offset: 0, attacker: "Team A + B", blank: "", invalid: NaN, missing: undefined }))
    .toBe("?offset=0&attacker=Team+A+%2B+B");
  expect(buildQueryString({})).toBe("");
});

test("team request parsing trims names and handles malformed JSON", async () => {
  expect(await parseTeamBody(new Request("http://example.test", {
    method: "POST",
    body: JSON.stringify({ name: "  Team A ", contact_email: " team@example.test " }),
  }))).toEqual({ name: "Team A", contactEmail: "team@example.test" });
  expect(await parseTeamBody(new Request("http://example.test", { method: "POST", body: "{" })))
    .toEqual({ name: undefined, contactEmail: undefined });
});

test("response handling preserves useful errors and permits empty successes", async () => {
  const path = "/api/v2/challenges";
  const fallback = `API request to ${path} failed with status 503`;
  expect(await processApiResponse(new Response(null, { status: 204 }), path)).toBeUndefined();
  for (const [response, message] of [
    [Response.json({ detail: "Try again", title: "Unavailable" }, { status: 503, headers: {
      "Content-Type": "application/problem+json", "Retry-After": " 8 ",
    } }), "Try again"],
    [Response.json({ title: "Unavailable" }, { status: 503 }), "Unavailable"],
    [Response.json({ message: "Maintenance" }, { status: 503 }), "Maintenance"],
    [Response.json({}, { status: 503 }), fallback],
    [new Response("{", { status: 503, headers: { "Content-Type": "application/json" } }), fallback],
    [new Response("Offline", { status: 503 }), fallback],
  ] as const) {
    await expect(processApiResponse(response, path)).rejects.toMatchObject({
      name: "PlatformAPIError",
      message,
      status: 503,
      retryAfter: response.headers.get("retry-after") ?? undefined,
    });
  }
});
