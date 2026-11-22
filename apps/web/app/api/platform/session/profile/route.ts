import { NextResponse } from "next/server";

import { problemResponse, upstreamErrorResponse } from "@/lib/api-handler";
import { participantSessionCookie } from "@/lib/participant-session-cookie";
import { updateCurrentParticipantProfile } from "@/lib/platform-api";
import { readSessionRequest, sessionString } from "@/lib/session-request";

export async function PUT(request: Request) {
  const body = await readSessionRequest(request);
  if (body instanceof NextResponse) return body;
  const profile = {
    displayName: sessionString(body.display_name).trim() || sessionString(body.displayName).trim(),
    email: sessionString(body.email).trim(),
    teamName: sessionString(body.team_name).trim() || sessionString(body.teamName).trim(),
    teamContactEmail: sessionString(body.team_contact_email).trim() || sessionString(body.teamContactEmail).trim(),
  };
  if (profile.displayName === "" || profile.email === "") {
    return problemResponse(
      400,
      "Invalid request",
      "display name and email are required.",
    );
  }

  try {
    const token = await updateCurrentParticipantProfile(profile);
    const response = NextResponse.json({ updated: true });
    response.cookies.set(participantSessionCookie(token, 60 * 60 * 24));
    return response;
  } catch (error) {
    return upstreamErrorResponse(error, "profile update failed", "Profile update failed");
  }
}
