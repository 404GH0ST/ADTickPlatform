import { NextResponse } from "next/server";

import { problemResponse, upstreamErrorResponse } from "@/lib/api-handler";
import { participantSessionCookie } from "@/lib/participant-session-cookie";
import { transferCurrentTeamCaptain } from "@/lib/platform-api";
import { readSessionRequest } from "@/lib/session-request";

export async function POST(request: Request) {
  const body = await readSessionRequest(request);
  if (body instanceof NextResponse) return body;
  const id = body.player_id ?? body.playerId;
  const playerID = typeof id === "number" || typeof id === "string" ? Number(id) : 0;
  if (!Number.isSafeInteger(playerID) || playerID <= 0) {
    return problemResponse(400, "Invalid request", "player_id is required.");
  }

  try {
    const token = await transferCurrentTeamCaptain(playerID);
    const response = NextResponse.json({ transferred: true });
    response.cookies.set(participantSessionCookie(token, 60 * 60 * 24));
    return response;
  } catch (error) {
    return upstreamErrorResponse(
      error,
      "captain transfer failed",
      "Captain transfer failed",
    );
  }
}
