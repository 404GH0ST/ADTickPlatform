import { NextResponse } from "next/server";

import { authenticateParticipant } from "@/lib/platform-api";

const participantSessionCookieName = "ad_platform_team_jwt";

export async function POST(request: Request) {
  const body = (await request.json().catch(() => null)) as {
    email?: string;
    password?: string;
  } | null;
  const email = body?.email?.trim() ?? "";
  const password = body?.password ?? "";

  if (email === "" || password.trim() === "") {
    return NextResponse.json(
      { status: "failed", message: "email and password are required." },
      { status: 400 },
    );
  }

  try {
    const token = await authenticateParticipant(email, password);
    const response = NextResponse.json({ status: "success", data: {} });

    let domain: string | undefined;
    const baseUrl = process.env.AD_PLATFORM_PUBLIC_BASE_URL;
    if (baseUrl) {
      try {
        const parsedHostname = new URL(baseUrl).hostname;
        if (parsedHostname !== "localhost" && parsedHostname !== "127.0.0.1") {
          domain = parsedHostname;
        }
      } catch (e) {
        // ignore
      }
    }

    let isSecure = process.env.NODE_ENV === "production";
    if (baseUrl && baseUrl.startsWith("http://")) {
      isSecure = false;
    }

    response.cookies.set({
      name: participantSessionCookieName,
      value: token,
      httpOnly: true,
      sameSite: "lax",
      secure: isSecure,
      path: "/",
      domain,
      maxAge: 60 * 60 * 24,
    });
    return response;
  } catch (error) {
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
}
