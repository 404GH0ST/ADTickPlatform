import { NextRequest, NextResponse } from "next/server";

import { updateAdminGameScheduler } from "@/lib/admin-api";

export async function PUT(request: NextRequest) {
  try {
    const body = await request.json();
    const intervalSeconds = body.interval_seconds;

    if (typeof intervalSeconds !== "number" || intervalSeconds < 1) {
      return NextResponse.json(
        { status: "failed", message: "invalid interval" },
        { status: 400 },
      );
    }

    const data = await updateAdminGameScheduler(intervalSeconds);
    return NextResponse.json({ status: "success", data });
  } catch (error) {
    const message =
      error instanceof Error ? error.message : "scheduler update failed";
    return NextResponse.json({ status: "failed", message }, { status: 502 });
  }
}
