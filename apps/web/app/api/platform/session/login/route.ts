import { NextResponse } from "next/server";

import { participantSessionCookie } from "@/lib/participant-session-cookie";
import { authenticateParticipant } from "@/lib/platform-api";

type LoginCredentials = {
  email: string;
  password: string;
};

export async function POST(request: Request) {
  const credentials = await readLoginCredentials(request);
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

async function readLoginCredentials(request: Request): Promise<LoginCredentials> {
  const body = (await request.json().catch(() => null)) as {
    email?: string;
    password?: string;
  } | null;

  return {
    email: body?.email?.trim() ?? "",
    password: body?.password ?? "",
  };
}

function hasRequiredCredentials(credentials: LoginCredentials): boolean {
  return credentials.email !== "" && credentials.password.trim() !== "";
}

function missingCredentialsResponse() {
  return NextResponse.json(
    { status: "failed", message: "email and password are required." },
    { status: 400 },
  );
}

function loginResponse(token: string) {
  const response = NextResponse.json({ status: "success", data: {} });
  response.cookies.set(participantSessionCookie(token, 60 * 60 * 24));
  return response;
}

function loginErrorResponse(error: unknown) {
  const message =
    error instanceof Error
      ? error.message
      : "participant authentication failed";
  const status = message === "email or password is wrong." ? 403 : 502;
  return NextResponse.json(
    { status: status === 403 ? "forbidden" : "failed", message },
    { status },
  );
}
