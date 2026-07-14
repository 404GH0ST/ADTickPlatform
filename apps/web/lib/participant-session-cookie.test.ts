// @ts-expect-error Bun exposes its test module at runtime without bundled TypeScript types.
import { afterEach, describe, expect, test } from "bun:test";

import { participantSessionCookie } from "./participant-session-cookie";

const testEnv = process.env as Record<string, string | undefined>;
const originalNodeEnv = process.env.NODE_ENV;
const originalPublicBaseURL = process.env.AD_PLATFORM_PUBLIC_BASE_URL;

afterEach(() => {
  testEnv.NODE_ENV = originalNodeEnv;
  testEnv.AD_PLATFORM_PUBLIC_BASE_URL = originalPublicBaseURL;
});

describe("participantSessionCookie", () => {
  test("always marks production session cookies secure", () => {
    testEnv.NODE_ENV = "production";
    testEnv.AD_PLATFORM_PUBLIC_BASE_URL = "http://example.test";

    expect(participantSessionCookie("token", 60).secure).toBe(true);
  });

  test("keeps the bearer cookie scoped to the exact host", () => {
    testEnv.NODE_ENV = "production";
    testEnv.AD_PLATFORM_PUBLIC_BASE_URL = "https://ctf.example.com";

    expect("domain" in participantSessionCookie("token", 60)).toBe(false);
  });

  test("allows insecure cookies only in development", () => {
    testEnv.NODE_ENV = "development";
    testEnv.AD_PLATFORM_PUBLIC_BASE_URL = "http://localhost";

    expect(participantSessionCookie("token", 60).secure).toBe(false);
  });
});
