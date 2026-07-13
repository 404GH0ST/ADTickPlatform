import { NextResponse } from "next/server";

import { problemResponse, upstreamErrorResponse } from "@/lib/api-handler";
import { participantSessionCookie } from "@/lib/participant-session-cookie";
import { joinCurrentParticipantTeam } from "@/lib/platform-api";

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
    return upstreamErrorResponse(error, "team join failed", "Team join failed");
  }
}

async function readTeamKey(request: Request): Promise<string> {
  const body = (await request.json().catch(() => null)) as {
    team_key?: string;
    teamKey?: string;
  } | null;

  return body?.team_key?.trim() || body?.teamKey?.trim() || "";
}
