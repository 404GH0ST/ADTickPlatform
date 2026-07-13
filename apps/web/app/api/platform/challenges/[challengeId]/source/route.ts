import { problemResponse, upstreamErrorResponse } from "@/lib/api-handler";
import { downloadChallengeSource } from "@/lib/platform-api";

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
    return upstreamErrorResponse(
      error,
      "challenge source download failed",
      "Source download failed",
    );
  }
}
