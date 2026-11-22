import { NextResponse } from "next/server";

import { isSameOriginBrowserRequest, problemResponse } from "@/lib/api-handler";

const maxSessionBodyBytes = 16 * 1024;

export async function readSessionRequest(request: Request): Promise<Record<string, unknown> | NextResponse> {
  if (!isSameOriginBrowserRequest(request)) {
    return problemResponse(403, "Authentication required", "cross-site mutation requests are not allowed.");
  }
  if (Number(request.headers.get("content-length")) > maxSessionBodyBytes) {
    return oversizedSessionRequest();
  }

  const reader = request.body?.getReader();
  const decoder = new TextDecoder("utf-8", { fatal: true });
  let bytesRead = 0;
  let text = "";
  try {
    if (reader) {
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        bytesRead += value.byteLength;
        if (bytesRead > maxSessionBodyBytes) {
          void reader.cancel().catch(() => {});
          return oversizedSessionRequest();
        }
        text += decoder.decode(value, { stream: true });
      }
    }
    text += decoder.decode();
    const body: unknown = JSON.parse(text);
    if (body && typeof body === "object" && !Array.isArray(body)) {
      return body as Record<string, unknown>;
    }
  } catch {
    // Malformed bodies follow the same validation path as missing fields.
  } finally {
    reader?.releaseLock();
  }
  return {};
}

function oversizedSessionRequest() {
  return problemResponse(413, "Request too large", "session request exceeds the size limit.");
}

export function sessionString(value: unknown): string {
  return typeof value === "string" ? value : "";
}
