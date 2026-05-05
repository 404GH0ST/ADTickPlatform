import { NextResponse } from "next/server";

import { participantSessionCookie } from "@/lib/participant-session-cookie";

export async function POST() {
  const response = new NextResponse(null, { status: 204 });
  response.cookies.set(participantSessionCookie("", 0));
  return response;
}
