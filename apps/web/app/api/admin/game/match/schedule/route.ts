import { NextResponse } from "next/server";

import { updateAdminGameMatchSchedule } from "@/lib/admin-api";

export async function PUT(request: Request) {
  try {
    const body = (await request.json()) as {
      scheduled_start_at?: string;
      scheduled_end_at?: string;
    };
    const data = await updateAdminGameMatchSchedule(body);
    return NextResponse.json({ status: "success", data });
  } catch (error) {
    const message =
      error instanceof Error ? error.message : "game match schedule update failed";
    return NextResponse.json({ status: "failed", message }, { status: 502 });
  }
}
