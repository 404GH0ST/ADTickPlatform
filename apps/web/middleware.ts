import { NextRequest, NextResponse } from "next/server";

import { participantSessionCookieName } from "@/lib/participant-session-cookie";
import { verifyParticipantSessionToken } from "@/lib/session-token";

function unauthorizedApi(detail: string) {
  return NextResponse.json(
    {
      title: "Authentication required",
      status: 403,
      detail,
    },
    { status: 403 },
  );
}

async function organizerRole(request: NextRequest) {
  const token = request.cookies.get(participantSessionCookieName)?.value?.trim();
  const secret = process.env.TEAM_JWT_SECRET?.trim() ?? "";
  if (!token || !secret) {
    return null;
  }
  return verifyParticipantSessionToken(token, secret);
}

export async function middleware(request: NextRequest) {
  const pathname = request.nextUrl.pathname;
  const claims = await organizerRole(request);
  const organizer = claims?.role === "organizer";

  if (pathname.startsWith("/api/admin/")) {
    if (!organizer) {
      const detail = claims
        ? "please authenticate as organizer."
        : "please authenticate before accessing organizer routes.";
      return unauthorizedApi(detail);
    }
    return NextResponse.next();
  }

  if (!pathname.startsWith("/admin")) {
    return NextResponse.next();
  }

  if (!claims) {
    const loginURL = new URL("/login", request.url);
    return NextResponse.redirect(loginURL);
  }

  if (!organizer) {
    const homeURL = new URL("/", request.url);
    return NextResponse.redirect(homeURL);
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/admin/:path*", "/api/admin/:path*"],
};
