import { NextResponse } from "next/server";

import { SESSION_ENDED_MESSAGE, upstreamErrorDetails } from "@/lib/api-utils";

export function isSameOriginBrowserRequest(request: Request, allowMissingOrigin = true) {
  if (request.headers.get("sec-fetch-site")?.toLowerCase() === "cross-site") {
    return false;
  }

  const url = new URL(request.url);
  const host = request.headers.get("host") ?? url.host;
  const forwardedProto = request.headers.get("x-forwarded-proto")?.split(",")[0]?.trim();
  const protocol = forwardedProto || url.protocol.replace(/:$/, "");
  const source = request.headers.get("origin") ?? request.headers.get("referer");
  if (source === null) {
    return allowMissingOrigin;
  }
  try {
    return new URL(source).origin === `${protocol}://${host}`;
  } catch {
    return false;
  }
}

export async function handleAdminIdRoute<T>(
  context: { params: Promise<Record<string, string>> },
  idParamName: string,
  idName: string,
  actionName: string,
  actionFn: (id: number) => Promise<T>,
) {
  const params = await context.params;
  const idStr = params[idParamName];
  if (!idStr) {
    return problemResponse(400, "Invalid request", `${idName} is invalid.`);
  }

  const id = Number(idStr);
  if (!Number.isInteger(id) || id <= 0) {
    return problemResponse(400, "Invalid request", `${idName} is invalid.`);
  }

  try {
    const data = await actionFn(id);
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, `${actionName} failed`);
  }
}

export async function handleParticipantIdRoute<T>(
  context: { params: Promise<Record<string, string>> },
  idParamName: string,
  idName: string,
  actionName: string,
  actionFn: (id: number) => Promise<T>,
) {
  const params = await context.params;
  const idStr = params[idParamName];
  if (!idStr) {
    return problemResponse(400, "Invalid request", `${idName} is invalid.`);
  }

  const id = Number(idStr);
  if (!Number.isInteger(id) || id <= 0) {
    return problemResponse(400, "Invalid request", `${idName} is invalid.`);
  }

  try {
    const data = await actionFn(id);
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, `${actionName} failed`);
  }
}

export async function forwardUpstreamResponse(response: Response) {
  const responseHeaders = new Headers();
  for (const name of ["content-type", "retry-after"]) {
    const value = response.headers.get(name);
    if (value) {
      responseHeaders.set(name, value);
    }
  }
  return new NextResponse(await response.text(), {
    status: response.status,
    headers: responseHeaders,
  });
}

export function upstreamErrorResponse(
  error: unknown,
  fallback: string,
  title = "Upstream request failed",
) {
  if (
    error instanceof Error &&
    error.message === SESSION_ENDED_MESSAGE
  ) {
    return problemResponse(403, "Authentication required", error.message);
  }
  const details = upstreamErrorDetails(error, fallback);
  const response = problemResponse(details.status, title, details.detail);
  if (details.retryAfter) {
    response.headers.set("Retry-After", details.retryAfter);
  }
  return response;
}

export function problemResponse(status: number, title: string, detail: string) {
  return NextResponse.json(
    { title, status, detail },
    {
      status,
      headers: {
        "Content-Type": "application/problem+json",
      },
    },
  );
}
