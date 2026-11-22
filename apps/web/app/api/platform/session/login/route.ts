import { NextResponse } from "next/server";

import { problemResponse, upstreamErrorResponse } from "@/lib/api-handler";
import { participantSessionCookie } from "@/lib/participant-session-cookie";
import { authenticateParticipant } from "@/lib/platform-api";
import { readSessionRequest } from "@/lib/session-request";

type LoginCredentials = {
  email: string;
  password: string;
};

export async function POST(request: Request) {
  const body = await readSessionRequest(request);
  if (body instanceof NextResponse) return body;
  const credentials = {
    email: typeof body.email === "string" ? body.email.trim() : "",
    password: typeof body.password === "string" ? body.password : "",
  };
  if (!hasRequiredCredentials(credentials)) {
    return missingCredentialsResponse();
  }

  try {
    const token = await authenticateParticipant(
      credentials.email,
      credentials.password,
    );
    return loginResponse(token);
  } catch (error) {
    return loginErrorResponse(error);
  }
}

function hasRequiredCredentials(credentials: LoginCredentials): boolean {
  return credentials.email !== "" && credentials.password.trim() !== "";
}

function missingCredentialsResponse() {
  return problemResponse(400, "Invalid request", "email and password are required.");
}

function loginResponse(token: string) {
  const response = NextResponse.json({ authenticated: true });
  response.cookies.set(participantSessionCookie(token, 60 * 60 * 24));
  return response;
}

function loginErrorResponse(error: unknown) {
  return upstreamErrorResponse(
    error,
    "participant authentication failed",
    "Authentication failed",
  );
}
