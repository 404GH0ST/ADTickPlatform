import { NextResponse } from "next/server";

import { problemResponse, upstreamErrorResponse } from "@/lib/api-handler";
import { participantSessionCookie } from "@/lib/participant-session-cookie";
import { transferCurrentTeamCaptain } from "@/lib/platform-api";

export async function POST(request: Request) {
  const body = (await request.json().catch(() => null)) as {
    player_id?: number;
    playerId?: number;
  } | null;
  const playerID = Number(body?.player_id ?? body?.playerId ?? 0);
  if (!Number.isFinite(playerID) || playerID <= 0) {
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
