import { NextRequest, NextResponse } from "next/server";

import { updateAdminGameScheduler } from "@/lib/admin-api";
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
    const message =
      error instanceof Error ? error.message : "scheduler update failed";
    return NextResponse.json(
      {
        title: "Upstream request failed",
        status: 502,
        detail: message,
      } satisfies ProblemDetails,
      { status: 502 },
    );
  }
}
