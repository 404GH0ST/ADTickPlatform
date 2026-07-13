import { NextRequest, NextResponse } from "next/server";

import { updateAdminGameScheduler } from "@/lib/admin-api";
import { upstreamErrorResponse } from "@/lib/api-handler";
import { ProblemDetails } from "@/lib/api-utils";

export async function PUT(request: NextRequest) {
  try {
    const body = await request.json();
    const intervalSeconds = body.interval_seconds;

    if (typeof intervalSeconds !== "number" || intervalSeconds < 1) {
      return NextResponse.json(
        {
          title: "Invalid request",
          status: 400,
          detail: "invalid interval",
        } satisfies ProblemDetails,
        { status: 400 },
      );
    }

    const data = await updateAdminGameScheduler(intervalSeconds);
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, "scheduler update failed");
  }
}
