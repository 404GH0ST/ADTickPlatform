export type ParticipantSessionClaims = Partial<{
  team_id: number;
  player_id: number;
  team_name: string;
  display_name: string;
  email: string;
  role: string;
  exp: number;
}>;

function decodeBase64Url(value: string) {
  const normalized = value.replace(/-/g, "+").replace(/_/g, "/");
  const padding =
    normalized.length % 4 === 0
      ? ""
      : "=".repeat(4 - (normalized.length % 4));
  const binary = atob(`${normalized}${padding}`);
  const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0));
  return new TextDecoder().decode(bytes);
}

function timingSafeEqual(a: Uint8Array, b: Uint8Array) {
  if (a.length !== b.length) {
    return false;
  }

  let diff = 0;
  for (let i = 0; i < a.length; i += 1) {
    diff |= a[i] ^ b[i];
  }
  return diff === 0;
}

export function decodeJWTClaims(token: string): ParticipantSessionClaims | null {
  const parts = token.split(".");
  if (parts.length !== 3) {
    return null;
  }

  try {
    return JSON.parse(decodeBase64Url(parts[1])) as ParticipantSessionClaims;
  } catch {
    return null;
  }
}

export async function verifyParticipantSessionToken(
  token: string,
  secret: string,
): Promise<ParticipantSessionClaims | null> {
  const trimmedSecret = secret.trim();
  if (!token.trim() || !trimmedSecret) {
    return null;
  }

  const parts = token.split(".");
  if (parts.length !== 3) {
    return null;
  }

  try {
    const header = JSON.parse(decodeBase64Url(parts[0])) as Partial<{
      alg: string;
      typ: string;
    }>;
    if (header.alg !== "HS256" || header.typ !== "JWT") {
      return null;
    }

    const key = await crypto.subtle.importKey(
      "raw",
      new TextEncoder().encode(trimmedSecret),
      { name: "HMAC", hash: "SHA-256" },
      false,
      ["sign"],
    );
    const data = new TextEncoder().encode(`${parts[0]}.${parts[1]}`);
    const signature = new Uint8Array(
      await crypto.subtle.sign("HMAC", key, data),
    );
    const provided = Uint8Array.from(
      atob(
        `${parts[2].replace(/-/g, "+").replace(/_/g, "/")}${
          parts[2].length % 4 === 0 ? "" : "=".repeat(4 - (parts[2].length % 4))
        }`,
      ),
      (char) => char.charCodeAt(0),
    );
    if (!timingSafeEqual(signature, provided)) {
      return null;
    }

    const claims = JSON.parse(
      decodeBase64Url(parts[1]),
    ) as ParticipantSessionClaims;
    if (
      typeof claims.exp === "number" &&
      claims.exp <= Math.floor(Date.now() / 1000)
    ) {
      return null;
    }
    return claims;
  } catch {
    return null;
  }
}
