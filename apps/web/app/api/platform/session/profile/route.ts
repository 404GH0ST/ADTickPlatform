import { NextResponse } from "next/server";

import { problemResponse, upstreamErrorResponse } from "@/lib/api-handler";
import { participantSessionCookie } from "@/lib/participant-session-cookie";
import { updateCurrentParticipantProfile } from "@/lib/platform-api";

export async function PUT(request: Request) {
  const profile = await readProfileUpdate(request);
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

async function readProfileUpdate(request: Request) {
  const body = (await request.json().catch(() => null)) as {
    display_name?: string;
    displayName?: string;
    email?: string;
    team_name?: string;
    teamName?: string;
    team_contact_email?: string;
    teamContactEmail?: string;
  } | null;

  return {
    displayName:
      body?.display_name?.trim() || body?.displayName?.trim() || "",
    email: body?.email?.trim() ?? "",
    teamName: body?.team_name?.trim() || body?.teamName?.trim() || "",
    teamContactEmail:
      body?.team_contact_email?.trim() ||
      body?.teamContactEmail?.trim() ||
      "",
  };
}
