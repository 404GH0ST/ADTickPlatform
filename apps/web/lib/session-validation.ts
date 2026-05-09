export type ValidatedParticipantSession = {
  player_id: number;
  team_id: number;
  team_name: string;
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

export async function validateParticipantSessionWithAPI(
  token: string,
): Promise<ValidatedParticipantSession | null> {
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
      display_name: typeof payload.display_name === "string" ? payload.display_name : "",
      email: typeof payload.email === "string" ? payload.email : "",
      role: payload.role,
    };
  } catch {
    return null;
  }
}
