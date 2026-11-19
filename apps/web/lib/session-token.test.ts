// @ts-expect-error Bun exposes its test module at runtime without bundled TypeScript types.
import { expect, test } from "bun:test";
import { createHmac } from "node:crypto";

import { decodeJWTClaims, verifyParticipantSessionToken } from "./session-token";

const secret = "unit-test-session-signing-key";
const header = { alg: "HS256", typ: "JWT" };
const session = { player_id: 5, team_id: 12, display_name: "Renée", exp: Math.floor(Date.now() / 1000) + 3600 };

function signSession(claims: object, tokenHeader = header) {
  const encode = (value: object) => Buffer.from(JSON.stringify(value)).toString("base64url");
  const data = `${encode(tokenHeader)}.${encode(claims)}`;
  const signature = createHmac("sha256", secret).update(data).digest("base64url");
  return `${data}.${signature}`;
}

test("decodes participant claims with UTF-8 names", () => {
  expect(decodeJWTClaims(signSession(session))).toEqual(session);
  expect(decodeJWTClaims("invalid")).toBeNull();
  expect(decodeJWTClaims("a.%.b")).toBeNull();
});

test("accepts a current session signed with the configured key", async () => {
  expect(await verifyParticipantSessionToken(signSession(session), ` ${secret} `)).toEqual(session);
});

test("rejects unusable and expired sessions", async () => {
  const token = signSession(session);
  const [tokenHeader, payload] = token.split(".");
  for (const [invalidToken, key] of [
    [token, ""],
    ["", secret],
    ["invalid", secret],
    ["%.%.%", secret],
    [token, "incorrect-signing-key"],
    [`${tokenHeader}.${payload}.AA`, secret],
    [signSession(session, { alg: "HS384", typ: "JWT" }), secret],
    [signSession({ ...session, exp: 1 }), secret],
  ]) {
    expect(await verifyParticipantSessionToken(invalidToken, key)).toBeNull();
  }
});
