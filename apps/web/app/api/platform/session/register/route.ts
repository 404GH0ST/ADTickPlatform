import { NextResponse } from "next/server";

import { participantSessionCookie } from "@/lib/participant-session-cookie";
import { registerParticipant } from "@/lib/platform-api";

type RegisterRequest = {
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
  const registerRequest = await readRegisterRequest(request);
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
    const message =
      error instanceof Error ? error.message : "player registration failed";
    const status =
      message.includes("unique email") || message.includes("required")
        ? 400
        : 502;
    return problemResponse(
      status,
      status === 502 ? "Registration unavailable" : "Registration failed",
      message,
    );
  }
}

async function readRegisterRequest(request: Request): Promise<RegisterRequest> {
  const body = (await request.json().catch(() => null)) as {
    display_name?: string;
    displayName?: string;
    email?: string;
    password?: string;
  } | null;

  return {
    displayName:
      body?.display_name?.trim() || body?.displayName?.trim() || "",
    email: body?.email?.trim() ?? "",
    password: body?.password ?? "",
  };
}

function hasRequiredFields(request: RegisterRequest): boolean {
  return (
    request.displayName !== "" &&
    request.email !== "" &&
    request.password.trim() !== ""
  );
}
