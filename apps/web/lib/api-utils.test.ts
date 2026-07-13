// @ts-expect-error Bun exposes its test module at runtime without bundled TypeScript types.
import { describe, expect, test } from "bun:test";

import {
  authenticatedFetch,
  firstForwardedAddress,
  PlatformAPIError,
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
});
