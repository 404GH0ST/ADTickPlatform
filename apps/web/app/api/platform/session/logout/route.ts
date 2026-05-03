import { NextResponse } from "next/server";

import { participantSessionCookie } from "@/lib/participant-session-cookie";

export async function POST() {
  const response = NextResponse.json({ status: "success" });
  response.cookies.set(participantSessionCookie("", 0));
  return response;
}
