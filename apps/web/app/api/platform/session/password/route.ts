import { NextResponse } from "next/server";

import { problemResponse, upstreamErrorResponse } from "@/lib/api-handler";
import { participantSessionCookie } from "@/lib/participant-session-cookie";
import { changeParticipantPassword } from "@/lib/platform-api";

export async function PUT(request: Request) {
  const body = (await request.json().catch(() => null)) as {
    current_password?: string;
    new_password?: string;
  } | null;
  const currentPassword = body?.current_password ?? "";
  const newPassword = body?.new_password ?? "";
  if (!currentPassword || !newPassword) {
    return problemResponse(
      400,
      "Invalid request",
      "current_password and new_password are required.",
    );
  }

  try {
    const data = await changeParticipantPassword({
      currentPassword,
      newPassword,
    });
    const response = NextResponse.json({ updated: data.updated });
    response.cookies.set(participantSessionCookie(data.token, 60 * 60 * 24));
    return response;
  } catch (error) {
    return upstreamErrorResponse(
      error,
      "password change failed",
      "Password change unavailable",
    );
  }
}
