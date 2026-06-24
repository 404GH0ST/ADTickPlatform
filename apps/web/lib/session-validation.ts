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

export type ParticipantSessionValidation =
  | ValidatedParticipantSession
  | "deactivated"
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
    return null;
  }
}
