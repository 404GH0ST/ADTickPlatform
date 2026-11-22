import { cookies } from "next/headers";
import { NextResponse } from "next/server";

import { isSameOriginBrowserRequest, problemResponse } from "@/lib/api-handler";
import { participantSessionCookie, participantSessionCookieName } from "@/lib/participant-session-cookie";
import { revokeParticipantSession } from "@/lib/session-validation";

export async function POST(request: Request) {
  if (!isSameOriginBrowserRequest(request)) {
    return problemResponse(403, "Authentication required", "cross-site mutation requests are not allowed.");
  }
  const token = (await cookies()).get(participantSessionCookieName)?.value;
  if (token) {
    try {
      await revokeParticipantSession(token);
    } catch {
      return problemResponse(503, "Sign-out unavailable", "Your session couldn't be ended. Try again in a moment.");
    }
  }
  const response = new NextResponse(null, { status: 204 });
  response.cookies.set(participantSessionCookie("", 0));
  return response;
}
