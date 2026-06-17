import { NextResponse } from "next/server";

import { participantSessionCookie } from "@/lib/participant-session-cookie";
import { joinParticipantTeam } from "@/lib/platform-api";

type JoinRequest = {
  teamKey: string;
  displayName: string;
  email: string;
  password: string;
};

function problemResponse(status: number, title: string, detail: string) {
  return NextResponse.json(
    { title, status, detail },
    { status, headers: { "Content-Type": "application/problem+json" } },
  );
}

export async function POST(request: Request) {
  const joinRequest = await readJoinRequest(request);
  if (!hasRequiredFields(joinRequest)) {
    return problemResponse(
      400,
      "Invalid request",
      "team key, display name, email, and password are required.",
    );
  }

  try {
    const token = await joinParticipantTeam(joinRequest);
    const response = NextResponse.json({ authenticated: true });
    response.cookies.set(participantSessionCookie(token, 60 * 60 * 24));
    return response;
  } catch (error) {
    const message =
      error instanceof Error ? error.message : "team join failed";
    const status =
      message === "team key is invalid."
        ? 403
        : message.includes("unique email") || message.includes("required")
          ? 400
          : 502;
    return problemResponse(
      status,
      status === 502 ? "Team join unavailable" : "Team join failed",
      message,
    );
  }
}

async function readJoinRequest(request: Request): Promise<JoinRequest> {
  const body = (await request.json().catch(() => null)) as {
    team_key?: string;
    teamKey?: string;
    display_name?: string;
    displayName?: string;
    email?: string;
    password?: string;
  } | null;

  return {
    teamKey: body?.team_key?.trim() || body?.teamKey?.trim() || "",
    displayName:
      body?.display_name?.trim() || body?.displayName?.trim() || "",
    email: body?.email?.trim() ?? "",
    password: body?.password ?? "",
  };
}

function hasRequiredFields(request: JoinRequest): boolean {
  return (
    request.teamKey !== "" &&
    request.displayName !== "" &&
    request.email !== "" &&
    request.password.trim() !== ""
  );
}
