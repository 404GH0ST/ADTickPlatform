import { NextResponse } from "next/server";

import { problemResponse, upstreamErrorResponse } from "@/lib/api-handler";
import { participantSessionCookie } from "@/lib/participant-session-cookie";
import { registerParticipant } from "@/lib/platform-api";
import { readSessionRequest } from "@/lib/session-request";

type RegisterRequest = {
  displayName: string;
  email: string;
  password: string;
};

export async function POST(request: Request) {
  const body = await readSessionRequest(request);
  if (body instanceof NextResponse) return body;
  const registerRequest = {
    displayName: trimmedString(body.display_name) || trimmedString(body.displayName),
    email: trimmedString(body.email),
    password: typeof body.password === "string" ? body.password : "",
  };
  if (!hasRequiredFields(registerRequest)) {
    return problemResponse(
      400,
      "Invalid request",
      "display name, email, and password are required.",
    );
  }

  try {
    const token = await registerParticipant(registerRequest);
    const response = NextResponse.json({ authenticated: true });
    response.cookies.set(participantSessionCookie(token, 60 * 60 * 24));
    return response;
  } catch (error) {
    return upstreamErrorResponse(error, "player registration failed", "Registration failed");
  }
}

function trimmedString(value: unknown) {
  return typeof value === "string" ? value.trim() : "";
}

function hasRequiredFields(request: RegisterRequest): boolean {
  return (
    request.displayName !== "" &&
    request.email !== "" &&
    request.password.trim() !== ""
  );
}
