import { NextResponse } from "next/server";

import { updateAdminGameMatchSchedule } from "@/lib/admin-api";
import { upstreamErrorResponse } from "@/lib/api-handler";

export async function PUT(request: Request) {
  try {
    const body = (await request.json()) as {
      scheduled_start_at?: string;
      scheduled_end_at?: string;
    };
    const data = await updateAdminGameMatchSchedule(body);
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(
      error,
      "game match schedule update failed",
      "Upstream request failed",
    );
  }
}
