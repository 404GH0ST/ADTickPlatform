import { NextResponse } from "next/server";

import { participantSessionCookie } from "@/lib/participant-session-cookie";
import { joinCurrentParticipantTeam } from "@/lib/platform-api";

function problemResponse(status: number, title: string, detail: string) {
  return NextResponse.json(
    { title, status, detail },
    { status, headers: { "Content-Type": "application/problem+json" } },
  );
}

export async function POST(request: Request) {
  const teamKey = await readTeamKey(request);
  if (teamKey === "") {
    return problemResponse(400, "Invalid request", "team key is required.");
  }

  try {
    const token = await joinCurrentParticipantTeam(teamKey);
    const response = NextResponse.json({ authenticated: true });
    response.cookies.set(participantSessionCookie(token, 60 * 60 * 24));
    return response;
  } catch (error) {
    const message =
      error instanceof Error ? error.message : "team join failed";
    const status =
      message === "team key is invalid."
        ? 403
        : message.includes("maximum member") ||
            message.includes("already joined") ||
            message.includes("required")
          ? 400
          : 502;
    return problemResponse(
      status,
      status === 502 ? "Team join unavailable" : "Team join failed",
      message,
    );
  }
}

async function readTeamKey(request: Request): Promise<string> {
  const body = (await request.json().catch(() => null)) as {
    team_key?: string;
    teamKey?: string;
  } | null;

  return body?.team_key?.trim() || body?.teamKey?.trim() || "";
}
