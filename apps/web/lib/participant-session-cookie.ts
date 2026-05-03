export const participantSessionCookieName = "ad_platform_team_jwt";

function publicBaseHostname(): string | undefined {
  const baseUrl = process.env.AD_PLATFORM_PUBLIC_BASE_URL;
  try {
    return baseUrl ? new URL(baseUrl).hostname : undefined;
  } catch {
    return undefined;
  }
}

function isLocalCookieHost(hostname: string): boolean {
  return hostname === "localhost" || hostname === "127.0.0.1";
}

function sessionCookieDomain(): string | undefined {
  const hostname = publicBaseHostname();
  return hostname && !isLocalCookieHost(hostname) ? hostname : undefined;
}

function sessionCookieSecure(): boolean {
  const baseUrl = process.env.AD_PLATFORM_PUBLIC_BASE_URL;
  return process.env.NODE_ENV === "production" && !baseUrl?.startsWith("http://");
}

export function participantSessionCookie(value: string, maxAge: number) {
  return {
    name: participantSessionCookieName,
    value,
    httpOnly: true,
    sameSite: "lax" as const,
    secure: sessionCookieSecure(),
    path: "/",
    domain: sessionCookieDomain(),
    maxAge,
  };
}
