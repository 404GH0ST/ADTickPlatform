import { NextResponse } from "next/server";

import { isSameOriginBrowserRequest, problemResponse, upstreamErrorResponse } from "@/lib/api-handler";
import { getParticipantToken, submitFlags } from "@/lib/platform-api";

const maxSubmissionBodyBytes = 128 * 1024;

export async function POST(request: Request) {
  if (!isSameOriginBrowserRequest(request, false)) {
    return problemResponse(403, "Origin rejected", "flag submissions must originate from this site.");
  }
  const contentType = request.headers.get("content-type")?.split(";")[0]?.trim().toLowerCase();
  if (contentType !== "application/json") {
    return problemResponse(415, "Unsupported media type", "flag submissions require application/json.");
  }
  if (Number(request.headers.get("content-length")) > maxSubmissionBodyBytes) {
    return oversizedSubmission();
  }

  try {
    const token = await getParticipantToken();
    const body = await readSubmission(request);
    if (body instanceof NextResponse) {
      return body;
    }
    const data = await submitFlags(token, body);
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, "submit failed", "Submit unavailable");
  }
}

function oversizedSubmission() {
  return problemResponse(413, "Request too large", "flag submission exceeds the request size limit.");
}

async function readSubmission(request: Request): Promise<string[] | NextResponse> {
  const reader = request.body?.getReader();
  const decoder = new TextDecoder("utf-8", { fatal: true });
  const chunks: string[] = [];
  let bytesRead = 0;
  try {
    if (reader) {
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        bytesRead += value.byteLength;
        if (bytesRead > maxSubmissionBodyBytes) {
          void reader.cancel().catch(() => {});
          return oversizedSubmission();
        }
        chunks.push(decoder.decode(value, { stream: true }));
      }
    }
    chunks.push(decoder.decode());
    const body = JSON.parse(chunks.join("")) as { flags?: unknown } | null;
    if (Array.isArray(body?.flags) && body.flags.every((flag: unknown) => typeof flag === "string")) {
      const flags = body.flags.map((flag: string) => flag.trim()).filter(Boolean);
      if (flags.length > 0) return flags;
    }
  } catch {
    // Invalid JSON and malformed UTF-8 are both invalid submissions.
  } finally {
    reader?.releaseLock();
  }
  return problemResponse(400, "Invalid request", "at least one flag is required.");
}
