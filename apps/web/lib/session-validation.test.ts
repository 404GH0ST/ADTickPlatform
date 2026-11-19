// @ts-expect-error Bun exposes its test module at runtime without bundled TypeScript types.
import { expect, test } from "bun:test";

import { validateParticipantSessionWithAPI } from "./session-validation";

async function withSessionUpstream(
  fetchMock: typeof fetch,
  run: () => Promise<void>,
) {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = fetchMock;
  try {
    await run();
  } finally {
    globalThis.fetch = originalFetch;
  }
}

test("session validation requests current identity without caching it", async () => {
  const session = {
    player_id: 3,
    team_id: 7,
    role: "participant",
    team_name: "Team Seven",
    team_contact_email: "team@example.test",
    display_name: "Player",
    email: "player@example.test",
  };
  let called = false;
  await withSessionUpstream(async (input, init) => {
    expect(String(input)).toMatch(/\/api\/v2\/session$/);
    expect(new Headers(init?.headers).get("authorization")).toBe("Bearer session-test-token");
    expect(init?.cache).toBe("no-store");
    called = true;
    return Response.json(session);
  }, async () => {
    expect(await validateParticipantSessionWithAPI("session-test-token")).toEqual(session);
    expect(called).toBe(true);
  });
});

test("session validation preserves deactivation and fails closed on unusable responses", async () => {
  for (const [response, expected] of [
    [Response.json({ title: "Access deactivated" }, { status: 403 }), "deactivated"],
    [Response.json({ title: "Authentication required" }, { status: 403 }), null],
    [new Response("unavailable", { status: 403 }), null],
    [new Response("unavailable", { status: 502 }), null],
    [Response.json({ player_id: 3 }), null],
    [Response.json({ player_id: 3, team_id: 7, role: "participant" }), {
      player_id: 3, team_id: 7, role: "participant", team_name: "", team_contact_email: "", display_name: "", email: "",
    }],
  ] as const) {
    await withSessionUpstream(async () => response, async () => {
      expect(await validateParticipantSessionWithAPI("session-test-token")).toEqual(expected);
    });
  }

  let calls = 0;
  await withSessionUpstream(async () => {
    calls += 1;
    throw new Error("connection failed");
  }, async () => {
    expect(await validateParticipantSessionWithAPI(" ")).toBeNull();
    expect(calls).toBe(0);
    expect(await validateParticipantSessionWithAPI("session-test-token")).toBeNull();
    expect(calls).toBe(1);
  });
});
