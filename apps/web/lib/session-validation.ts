export type ValidatedParticipantSession = {
  player_id: number;
  team_id: number;
  team_name: string;
  team_contact_email: string;
  display_name: string;
  email: string;
  role: string;
};

function trimBaseUrl(value: string) {
  return value.trim().replace(/\/+$/, "");
}

function apiBaseUrl() {
  return trimBaseUrl(
    process.env.AD_PLATFORM_API_URL ?? "http://127.0.0.1:8080",
  );
}

// Every page waits on this check, so a silent gateway must not hold it open.
const SESSION_CHECK_TIMEOUT_MS = 3_000;

export async function revokeParticipantSession(token: string) {
  const response = await fetch(`${apiBaseUrl()}/api/v2/session/logout`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
    signal: AbortSignal.timeout(SESSION_CHECK_TIMEOUT_MS),
  });
  if (!response.ok) throw new Error("Session revocation failed");
}

/**
 * "unavailable" means the gateway could not answer, not that the session is
 * bad. It grants nothing: callers must keep access closed, but should say the
 * platform is unreachable instead of asking the person to sign in again.
 */
export type ParticipantSessionValidation =
  | ValidatedParticipantSession
  | "deactivated"
  | "unavailable"
  | null;

export async function validateParticipantSessionWithAPI(
  token: string,
): Promise<ParticipantSessionValidation> {
  if (!token.trim()) {
    return null;
  }

  try {
    const response = await fetch(`${apiBaseUrl()}/api/v2/session`, {
      headers: {
        Authorization: `Bearer ${token}`,
      },
      cache: "no-store",
      signal: AbortSignal.timeout(SESSION_CHECK_TIMEOUT_MS),
    });
    if (!response.ok) {
      // A deactivated player/team is reported by the gateway as a 403 with the
      // "Access deactivated" problem title; surface it so the UI can explain why
      // access stopped instead of showing a generic "please log in" prompt.
      if (response.status === 403) {
        try {
          const problem = (await response.json()) as { title?: string };
          if (problem?.title === "Access deactivated") {
            return "deactivated";
          }
        } catch {
          // fall through to the generic null result below
        }
      }
      if (response.status === 408 || response.status === 429 || response.status >= 500) {
        return "unavailable";
      }
      return null;
    }
    const payload = (await response.json()) as Partial<ValidatedParticipantSession>;
    if (
      typeof payload.player_id !== "number" ||
      typeof payload.team_id !== "number" ||
      typeof payload.role !== "string"
    ) {
      return null;
    }
    return {
      player_id: payload.player_id,
      team_id: payload.team_id,
      team_name: typeof payload.team_name === "string" ? payload.team_name : "",
      team_contact_email:
        typeof payload.team_contact_email === "string"
          ? payload.team_contact_email
          : "",
      display_name: typeof payload.display_name === "string" ? payload.display_name : "",
      email: typeof payload.email === "string" ? payload.email : "",
      role: payload.role,
    };
  } catch {
    // Network failure or an unreadable response from the gateway.
    return "unavailable";
  }
}
