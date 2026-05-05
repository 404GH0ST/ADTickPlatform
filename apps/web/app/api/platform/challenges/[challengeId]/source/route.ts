import { downloadChallengeSource } from "@/lib/platform-api";

function problemResponse(status: number, title: string, detail: string) {
  return Response.json(
    { title, status, detail },
    {
      status,
      headers: {
        "Content-Type": "application/problem+json",
      },
    },
  );
}

function passthroughHeaders(headers: Headers): Headers {
  const next = new Headers();
  for (const name of ["content-type", "content-disposition", "content-length", "cache-control"]) {
    const value = headers.get(name);
    if (value) {
      next.set(name, value);
    }
  }
  return next;
}

export async function GET(
  _request: Request,
  context: { params: Promise<{ challengeId: string }> },
) {
  const { challengeId } = await context.params;
  const challengeID = Number(challengeId);
  if (!Number.isInteger(challengeID) || challengeID <= 0) {
    return problemResponse(400, "Invalid request", "challenge id is invalid.");
  }

  try {
    const response = await downloadChallengeSource(challengeID);
    return new Response(response.body, {
      status: response.status,
      headers: passthroughHeaders(response.headers),
    });
  } catch (error) {
    const message =
      error instanceof Error ? error.message : "challenge source download failed";
    const status = message === "participant session is not authenticated." ? 403 : 502;
    return problemResponse(
      status,
      status === 403 ? "Authentication required" : "Source download failed",
      message,
    );
  }
}
