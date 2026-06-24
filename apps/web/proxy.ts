import { NextRequest, NextResponse } from "next/server";

import { participantSessionCookieName } from "@/lib/participant-session-cookie";
import { validateParticipantSessionWithAPI } from "@/lib/session-validation";
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

function isSafeMethod(method: string) {
  return method === "GET" || method === "HEAD" || method === "OPTIONS";
}

function isBrowserMutationRoute(pathname: string) {
  return (
    pathname.startsWith("/api/admin/") ||
    pathname.startsWith("/api/platform/services/") ||
    pathname.startsWith("/api/platform/session/")
  );
}

function requestOrigin(request: NextRequest) {
  const host = request.headers.get("host") ?? request.nextUrl.host;
  const forwardedProto = request.headers.get("x-forwarded-proto")?.split(",")[0]?.trim();
  const protocol = forwardedProto || request.nextUrl.protocol.replace(/:$/, "");
  return `${protocol}://${host}`;
}

function sameOriginBrowserMutation(request: NextRequest) {
  if (isSafeMethod(request.method) || !isBrowserMutationRoute(request.nextUrl.pathname)) {
    return true;
  }

  const expectedOrigin = requestOrigin(request);
  const secFetchSite = request.headers.get("sec-fetch-site")?.toLowerCase();
  if (secFetchSite === "cross-site") {
    return false;
  }

  const origin = request.headers.get("origin");
  if (origin) {
    try {
      return new URL(origin).origin === expectedOrigin;
    } catch {
      return false;
    }
  }

  const referer = request.headers.get("referer");
  if (referer) {
    try {
      return new URL(referer).origin === expectedOrigin;
    } catch {
      return false;
    }
  }

  return true;
}

async function organizerSession(request: NextRequest) {
  const token = request.cookies.get(participantSessionCookieName)?.value?.trim();
  const secret = process.env.TEAM_JWT_SECRET?.trim() ?? "";
  if (!token || !secret) {
    return null;
  }
  const claims = await verifyParticipantSessionToken(token, secret);
  if (!claims) {
    return null;
  }
  const validated = await validateParticipantSessionWithAPI(token);
  // A deactivated organizer has no usable session; treat it as unauthenticated
  // so admin routes redirect to login rather than leaking a partial session.
  return validated === "deactivated" ? null : validated;
}

export async function proxy(request: NextRequest) {
  const pathname = request.nextUrl.pathname;
  if (!sameOriginBrowserMutation(request)) {
    return unauthorizedApi("cross-site mutation requests are not allowed.");
  }

  const session = await organizerSession(request);
  const organizer = session?.role === "organizer";

  if (pathname.startsWith("/api/admin/")) {
    if (!organizer) {
      const detail = session
        ? "please authenticate as organizer."
        : "please authenticate before accessing organizer routes.";
      return unauthorizedApi(detail);
    }
    return NextResponse.next();
  }

  if (!pathname.startsWith("/admin")) {
    return NextResponse.next();
  }

  if (!session) {
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
  matcher: [
    "/admin/:path*",
    "/api/admin/:path*",
    "/api/platform/services/:path*",
    "/api/platform/session/:path*",
  ],
};
