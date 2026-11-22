import { NextResponse } from "next/server";

import { problemResponse, upstreamErrorResponse } from "@/lib/api-handler";
import { participantSessionCookie } from "@/lib/participant-session-cookie";
import { joinCurrentParticipantTeam } from "@/lib/platform-api";
import { readSessionRequest, sessionString } from "@/lib/session-request";

export async function POST(request: Request) {
  const body = await readSessionRequest(request);
  if (body instanceof NextResponse) return body;
  const teamKey = sessionString(body.team_key).trim() || sessionString(body.teamKey).trim();
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
