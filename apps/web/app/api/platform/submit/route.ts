import { NextResponse } from "next/server";

import { problemResponse, upstreamErrorResponse } from "@/lib/api-handler";
import { submitFlags } from "@/lib/platform-api";

export async function POST(request: Request) {
  const body = (await request.json().catch(() => null)) as {
    flags?: string[];
  } | null;
  const flags = Array.isArray(body?.flags)
    ? body.flags.map((flag) => String(flag).trim()).filter(Boolean)
    : [];
  if (flags.length === 0) {
    return problemResponse(400, "Invalid request", "at least one flag is required.");
  }

  try {
    const data = await submitFlags(flags);
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, "submit failed", "Submit unavailable");
  }
}
