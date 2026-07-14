export const participantSessionCookieName = "ad_platform_team_jwt";

function sessionCookieSecure(): boolean {
  return process.env.NODE_ENV === "production";
}

export function participantSessionCookie(value: string, maxAge: number) {
  return {
    name: participantSessionCookieName,
    value,
    httpOnly: true,
    sameSite: "lax" as const,
    secure: sessionCookieSecure(),
    path: "/",
    maxAge,
  };
}
