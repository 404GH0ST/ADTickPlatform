import { NextResponse } from "next/server";

const participantSessionCookieName = "ad_platform_team_jwt";

export async function POST() {
  const response = NextResponse.json({ status: "success" });

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
    value: "",
    httpOnly: true,
    sameSite: "lax",
    secure: isSecure,
    path: "/",
    domain,
    maxAge: 0,
  });
  return response;
}
